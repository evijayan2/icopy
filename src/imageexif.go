package icopy

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
	"github.com/rwcarlsen/goexif/exif"
)

func ReadJpegDate(ctx context.Context, db *badger.DB, src_dirname string, options ScanOptions) ([]FileObject, []ErroredFileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)

	imageFiles := []FileObject{}
	erroredFiles := []ErroredFileObject{}

	// Channels for results
	imageChan := make(chan FileObject)
	erroredChan := make(chan ErroredFileObject)

	// Worker pool
	jobs := make(chan string)
	var wg sync.WaitGroup

	numWorkers := options.NumWorkers
	if numWorkers <= 0 {
		numWorkers = 10
	}

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fpath := range jobs {
				if options.ProgressChan != nil {
					options.ProgressChan <- fmt.Sprintf("Scanning: %s", filepath.Base(fpath))
				}
				processImageFile(ctx, db, fpath, imageChan, erroredChan, options.UseFastHash)
			}
		}()
	}

	// Result collector
	done := make(chan struct{})
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for {
			select {
			case img := <-imageChan:
				imageFiles = append(imageFiles, img)
			case errFile := <-erroredChan:
				erroredFiles = append(erroredFiles, errFile)
			case <-done:
				return
			}
		}
	}()

	err := filepath.WalkDir(src_dirname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Error().Err(err).Msgf("Error walking path: %s", path)
			return nil
		}
		if d.IsDir() {
			if path != src_dirname && !options.Recursive {
				return filepath.SkipDir
			}
			return nil
		}

		lowerName := strings.ToLower(d.Name())
		if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
			strings.HasSuffix(lowerName, ".gif") || strings.HasSuffix(lowerName, ".png") ||
			strings.HasSuffix(lowerName, ".bmp") || strings.HasSuffix(lowerName, ".heic") ||
			strings.HasSuffix(lowerName, ".tiff") || strings.HasSuffix(lowerName, ".tif") ||
			strings.HasSuffix(lowerName, ".webp") || strings.HasSuffix(lowerName, ".svg") ||
			strings.HasSuffix(lowerName, ".psd") || strings.HasSuffix(lowerName, ".ai") ||
			strings.HasSuffix(lowerName, ".cr2") || strings.HasSuffix(lowerName, ".nef") ||
			strings.HasSuffix(lowerName, ".arw") || strings.HasSuffix(lowerName, ".dng") ||
			strings.HasSuffix(lowerName, ".orf") || strings.HasSuffix(lowerName, ".rw2") ||
			strings.HasSuffix(lowerName, ".raf") || strings.HasSuffix(lowerName, ".cr3") {
			jobs <- path
		}
		return nil
	})

	if err != nil {
		logger.Error().Err(err).Msg("Error walking directory")
	}

	close(jobs)
	wg.Wait()
	close(done)
	<-collectorDone

	// drain channels just in case
	close(imageChan)
	close(erroredChan)

	return imageFiles, erroredFiles
}

func processImageFile(ctx context.Context, db *badger.DB, fpath string, imageChan chan<- FileObject, erroredChan chan<- ErroredFileObject, useFastHash bool) {
	logger := ctx.Value("logger").(zerolog.Logger)
	fileName := filepath.Base(fpath)

	md5sum, err := ComputeFileHash(fpath, useFastHash)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to calculate md5sum for file: %s", fpath)
		// Consider adding to erroredFiles or just logging. For now logging.
		return
	}
	PutBadgerDB(db, "src-"+md5sum, fpath)

	lowerName := strings.ToLower(fileName)
	var tm time.Time

	// Determine if we should attempt to read EXIF data
	tryExif := false
	isHeic := false
	if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
		strings.HasSuffix(lowerName, ".tiff") || strings.HasSuffix(lowerName, ".tif") ||
		strings.HasSuffix(lowerName, ".cr2") || strings.HasSuffix(lowerName, ".nef") ||
		strings.HasSuffix(lowerName, ".arw") || strings.HasSuffix(lowerName, ".dng") ||
		strings.HasSuffix(lowerName, ".orf") || strings.HasSuffix(lowerName, ".rw2") ||
		strings.HasSuffix(lowerName, ".raf") {
		tryExif = true
	} else if strings.HasSuffix(lowerName, ".heic") {
		isHeic = true
	}

	foundDate := false
	if tryExif {
		fd, err := os.Open(fpath)
		if err == nil {
			defer fd.Close()
			x, err := exif.Decode(fd)
			if err == nil {
				t, err := x.DateTime()
				if err == nil {
					tm = t
					foundDate = true
				}
			} else {
				// Log debug if needed
			}
		} else {
			logger.Error().Err(err).Msgf("Failed to open file: %s", fpath)
			erroredChan <- ErroredFileObject{
				DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
				ErrorMessage: err.Error(),
			}
			return
		}
	} else if isHeic {
		// HEIC custom parsing
		fd, err := os.Open(fpath)
		if err == nil {
			defer fd.Close()
			exifData, err := ExtractHeicExif(fd)
			if err == nil && exifData != nil {
				x, err := exif.Decode(bytes.NewReader(exifData))
				if err == nil {
					t, err := x.DateTime()
					if err == nil {
						tm = t
						foundDate = true
					}
				}
			}
		} else {
			logger.Error().Err(err).Msgf("Failed to open file: %s", fpath)
			erroredChan <- ErroredFileObject{
				DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
				ErrorMessage: err.Error(),
			}
			return
		}
	}

	// Fallback to file modification time if EXIF failed or not supported
	if !foundDate {
		fi, err := os.Stat(fpath)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to stat file: %s", fpath)
			return
		}
		tm = fi.ModTime()
	}

	// Fallback to file modification time if EXIF failed or not supported
	if !foundDate {
		fi, err := os.Stat(fpath)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to stat file: %s", fpath)
			return
		}
		tm = fi.ModTime()
	}

	imageChan <- FileObject{DateTime: tm, Name: fileName, Path: filepath.Dir(fpath), Md5Sum: md5sum}
}
