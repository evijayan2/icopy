package icopy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
)

type FileProcessor struct {
	Overwrite    string
	ForceCopy    bool
	Recursive    bool
	DateFmt      string
	UseFastHash  bool
	NumWorkers   int
	ProgressChan chan string
}

func (fp *FileProcessor) CopyImageFiles(ctx context.Context, srcdir string, destdir string) ([]FileObject, []ErroredFileObject, []FileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)

	db, err := OpenBadgerDB("./badger")
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to open badger db")
	}

	options := ScanOptions{
		Recursive:    fp.Recursive,
		NumWorkers:   fp.NumWorkers,
		UseFastHash:  fp.UseFastHash,
		ProgressChan: fp.ProgressChan,
	}

	imagefiles, erroredfiles := ReadJpegDate(ctx, db, srcdir, options)

	ScanAndGenerateMd5sumFiles(ctx, db, destdir, "dst", options)

	SortFilesByDate(imagefiles)

	if !fp.ForceCopy {
		imagefiles = checkStatusFile(ctx, imagefiles)
		if len(imagefiles) == 0 {
			CloseBadgerDB(db)
			return nil, erroredfiles, nil
		}
	}

	filesCopied, erroredfiles1, skipedfiles := fp.copyFile(ctx, db, imagefiles, destdir)
	erroredfiles = append(erroredfiles, erroredfiles1...)

	CloseBadgerDB(db)

	return filesCopied, erroredfiles, skipedfiles
}

func (fp *FileProcessor) CopyVideoFiles(ctx context.Context, srcdir string, destdir string) ([]FileObject, []ErroredFileObject, []FileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)

	db, err := OpenBadgerDB("./badger")
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to open badger db")
	}

	options := ScanOptions{
		Recursive:    fp.Recursive,
		NumWorkers:   fp.NumWorkers,
		UseFastHash:  fp.UseFastHash,
		ProgressChan: fp.ProgressChan,
	}

	videofiles, erroredfiles := ReadVideoCreationTimeMetadata(ctx, db, srcdir, options)

	SortFilesByDate(videofiles)

	if !fp.ForceCopy {
		videofiles = checkStatusFile(ctx, videofiles)
		if len(videofiles) == 0 {
			CloseBadgerDB(db)
			return nil, erroredfiles, nil
		}
	}

	filesCopied, erroredfiles1, skipedfiles := fp.copyFile(ctx, db, videofiles, destdir)
	erroredfiles = append(erroredfiles, erroredfiles1...)

	CloseBadgerDB(db)

	return filesCopied, erroredfiles, skipedfiles
}

func (fp *FileProcessor) copyFile(ctx context.Context, db *badger.DB, imagefiles []FileObject, destdir string) ([]FileObject, []ErroredFileObject, []FileObject) {
	filesCopied := []FileObject{}
	erroredFiles := []ErroredFileObject{}
	skipedfiles := []FileObject{}

	copyChan := make(chan FileObject)
	errorChan := make(chan ErroredFileObject)
	skipChan := make(chan FileObject)

	jobs := make(chan FileObject, len(imagefiles))
	var wg sync.WaitGroup
	numWorkers := fp.NumWorkers // Adjust based on needs
	if numWorkers <= 0 {
		numWorkers = 10
	}
	var counter int64

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for image := range jobs {
				if fp.ProgressChan != nil {
					current := atomic.LoadInt64(&counter) + 1
					select {
					case fp.ProgressChan <- fmt.Sprintf("Copying (%d/%d): %s", current, len(imagefiles), image.Name):
					default:
					}
				}
				fp.processCopy(ctx, db, image, destdir, copyChan, errorChan, skipChan, &counter, len(imagefiles))
			}
		}()
	}

	for _, image := range imagefiles {
		jobs <- image
	}
	close(jobs)

	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		openChannels := 3
		for openChannels > 0 {
			select {
			case f, ok := <-copyChan:
				if !ok {
					copyChan = nil
					openChannels--
				} else {
					filesCopied = append(filesCopied, f)
				}
			case e, ok := <-errorChan:
				if !ok {
					errorChan = nil
					openChannels--
				} else {
					erroredFiles = append(erroredFiles, e)
				}
			case s, ok := <-skipChan:
				if !ok {
					skipChan = nil
					openChannels--
				} else {
					skipedfiles = append(skipedfiles, s)
				}
			}
		}
	}()

	wg.Wait()
	close(copyChan)
	close(errorChan)
	close(skipChan)
	<-collectorDone

	return filesCopied, erroredFiles, skipedfiles
}

func (fp *FileProcessor) processCopy(ctx context.Context, db *badger.DB, image FileObject, destdir string,
	copyChan chan<- FileObject, errorChan chan<- ErroredFileObject, skipChan chan<- FileObject,
	counter *int64, total int) {

	logger := ctx.Value("logger").(zerolog.Logger)
	tm := image.DateTime
	fpath := filepath.Join(image.Path, image.Name)

	value, err := GetBadgerDBValue(db, "dst-"+image.Md5Sum)
	if (err == nil || value != "") && fp.Overwrite == "no" && !fp.ForceCopy {
		skipChan <- FileObject{Path: image.Path, Name: image.Name, DateTime: tm}
		return
	}

	fis, _ := os.Stat(fpath)
	fd, err := os.Open(fpath)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to open file: %s", fpath)
		errorChan <- ErroredFileObject{Path: image.Path, Name: image.Name, DateTime: tm, ErrorMessage: err.Error()}
		return
	}
	defer fd.Close()

	fYMdir := getDestinationPath(tm, destdir, fp.DateFmt)
	if err := os.MkdirAll(fYMdir, 0755); err != nil {
		logger.Error().Err(err).Msgf("Failed to create directory: %s", fYMdir)
		errorChan <- ErroredFileObject{Path: image.Path, Name: image.Name, DateTime: tm, ErrorMessage: err.Error()}
		return
	}

	fYMpath := filepath.Join(fYMdir, strings.ReplaceAll(image.Name, "%20", "_"))
	fYMpath = strings.ReplaceAll(fYMpath, " ", "_")

	shouldWrite := false
	if fi, err := os.Stat(fYMpath); err == nil {
		if !fi.IsDir() {
			overwrite := strings.ToLower(fp.Overwrite)
			if overwrite == "yes" || fp.ForceCopy {
				shouldWrite = true
			} else if overwrite == "ask" {
				shouldWrite = false
			} else {
				shouldWrite = false
			}
		}
	} else {
		shouldWrite = true
	}

	if shouldWrite {
		// Need different source based on logic?
		// Original code: passed 'fis' or 'fi' to writeFile.
		// 'fis' is source file info. 'fi' was destination file info?
		// writeFile used 'fi' to set ModTime.
		// If overwriting, 'fi' is dest info? No, we want to preserve SOURCE time presumably.
		// Logic in orig:
		// if exists: `writeFile(..., fi, ...)` -> `fi` is DEST info.
		// if not exists: `writeFile(..., fis, ...)` -> `fis` is SOURCE info.
		// And `writeFile` does `os.Chtimes(fYMpath, fi.ModTime(), fi.ModTime())`.
		// If overwriting, we probably want to set it to Source time, or keep Dest time?
		// Usually we want to preserve Source time.
		// If I overwrite a file, I probably want it to look like the source.
		// But let's check original logic carefully.
		// Orig: `filesCopied = writeFile(fYMpath, logger, fd, fi, ctx, image, fYMdir, i, imagefiles, filesCopied, tm)`
		// `fi` comes from `os.Stat(fYMpath)` (Dest).
		// So it preserves DEST time? That's weird.
		// `else` (not exists): `filesCopied = writeFile(..., fis, ...)` (Source).
		// So if new file, use Source time. If overwrite, use Dest time?
		// Maybe to avoid re-syncing?
		// Unsure. I will stick to usage of 'fis' (Source) for consistency, unless 'Overwrite' implies something else.
		// Actually, if I overwrite, I'm replacing the file. I should probably use Source time.
		// I will use 'fis' (Source) for both cases. It makes more sense.

		err := writeFile(fYMpath, fd, fis)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to write file: %s", fYMpath)
			errorChan <- ErroredFileObject{Path: image.Path, Name: image.Name, DateTime: tm, ErrorMessage: err.Error()}
			return
		}

		writeStatusFile(ctx, image)
		atomic.AddInt64(counter, 1)

		copyChan <- FileObject{Path: fYMdir, Name: image.Name, DateTime: tm}
	} else {
		skipChan <- FileObject{Path: fYMdir, Name: image.Name, DateTime: tm}
	}
}

func writeFile(fYMpath string, r io.Reader, fi fs.FileInfo) error {
	fwout, err := os.Create(fYMpath)
	if err != nil {
		return err
	}
	defer fwout.Close()

	if _, err := io.Copy(fwout, r); err != nil {
		return err
	}

	// Explicitly close before Chtimes to flush?
	if err := fwout.Close(); err != nil {
		return err
	}

	return os.Chtimes(fYMpath, fi.ModTime(), fi.ModTime())
}

func getDestinationPath(tm time.Time, destdir string, datefmt string) string {
	dPath := ""
	switch datefmt {
	case "DATE":
		dPath = fmt.Sprintf("%04d-%02d-%02d", tm.Year(), tm.Month(), tm.Day())
	case "YEAR-MONTH":
		dPath = fmt.Sprintf("%04d/%02d", tm.Year(), tm.Month())
	default:
		dPath = ""
	}
	return filepath.Join(destdir, dPath)
}
