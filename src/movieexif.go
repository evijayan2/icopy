package icopy

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
)

// mov spec: https://developer.apple.com/standards/qtff-2001.pdf
// Page 31-33 contain information used in this file

const appleEpochAdjustment = 2082844800

const (
	movieResourceAtomType   = "moov"
	movieHeaderAtomType     = "mvhd"
	referenceMovieAtomType  = "rmra"
	compressedMovieAtomType = "cmov"
)

func ReadVideoCreationTimeMetadata(ctx context.Context, db *badger.DB, src_dirname string, options ScanOptions) ([]FileObject, []ErroredFileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)

	imageFiles := []FileObject{}
	erroredFiles := []ErroredFileObject{}

	// Channels for results
	videoChan := make(chan FileObject)
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
					select {
					case options.ProgressChan <- fmt.Sprintf("Scanning: %s", filepath.Base(fpath)):
					default:
					}
				}
				processVideoFile(ctx, db, fpath, videoChan, erroredChan, options.UseFastHash)
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
			case img := <-videoChan:
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
		if strings.HasSuffix(lowerName, ".mp4") || strings.HasSuffix(lowerName, ".mov") ||
			strings.HasSuffix(lowerName, ".wmv") || strings.HasSuffix(lowerName, ".avi") ||
			strings.HasSuffix(lowerName, ".mpg") || strings.HasSuffix(lowerName, ".3gp") ||
			strings.HasSuffix(lowerName, ".m4v") || strings.HasSuffix(lowerName, ".mkv") ||
			strings.HasSuffix(lowerName, ".webm") || strings.HasSuffix(lowerName, ".flv") ||
			strings.HasSuffix(lowerName, ".ts") || strings.HasSuffix(lowerName, ".mts") ||
			strings.HasSuffix(lowerName, ".m2ts") || strings.HasSuffix(lowerName, ".vob") ||
			strings.HasSuffix(lowerName, ".ogg") || strings.HasSuffix(lowerName, ".qt") ||
			strings.HasSuffix(lowerName, ".yuv") || strings.HasSuffix(lowerName, ".rm") ||
			strings.HasSuffix(lowerName, ".rmvb") || strings.HasSuffix(lowerName, ".viv") ||
			strings.HasSuffix(lowerName, ".asf") || strings.HasSuffix(lowerName, ".amv") ||
			strings.HasSuffix(lowerName, ".svi") || strings.HasSuffix(lowerName, ".3g2") ||
			strings.HasSuffix(lowerName, ".mxf") || strings.HasSuffix(lowerName, ".roq") ||
			strings.HasSuffix(lowerName, ".nsv") || strings.HasSuffix(lowerName, ".f4v") ||
			strings.HasSuffix(lowerName, ".f4p") || strings.HasSuffix(lowerName, ".f4a") ||
			strings.HasSuffix(lowerName, ".f4b") {
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

	close(videoChan)
	close(erroredChan)

	return imageFiles, erroredFiles
}

func processVideoFile(ctx context.Context, db *badger.DB, fpath string, videoChan chan<- FileObject, erroredChan chan<- ErroredFileObject, useFastHash bool) {
	logger := ctx.Value("logger").(zerolog.Logger)
	fileName := filepath.Base(fpath)

	md5sum, err := ComputeFileHash(fpath, useFastHash)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to calculate md5sum for file: %s", fpath)
		return
	}
	PutBadgerDB(db, "src-"+md5sum, fpath)

	lowerName := strings.ToLower(fileName)

	if strings.HasSuffix(lowerName, ".mp4") || strings.HasSuffix(lowerName, ".mov") ||
		strings.HasSuffix(lowerName, ".3gp") || strings.HasSuffix(lowerName, ".m4v") ||
		strings.HasSuffix(lowerName, ".qt") || strings.HasSuffix(lowerName, ".3g2") ||
		strings.HasSuffix(lowerName, ".f4v") || strings.HasSuffix(lowerName, ".f4p") ||
		strings.HasSuffix(lowerName, ".f4a") || strings.HasSuffix(lowerName, ".f4b") {
		// log.Printf("Reading file: %s", fileName)
		processMp4Mov(ctx, fpath, fileName, md5sum, videoChan, erroredChan)
	} else if strings.HasSuffix(lowerName, ".mpg") || strings.HasSuffix(lowerName, ".vob") {
		processMpg(ctx, fpath, fileName, md5sum, videoChan, erroredChan)
	} else if strings.HasSuffix(lowerName, ".wmv") || strings.HasSuffix(lowerName, ".avi") ||
		strings.HasSuffix(lowerName, ".mkv") || strings.HasSuffix(lowerName, ".webm") ||
		strings.HasSuffix(lowerName, ".flv") || strings.HasSuffix(lowerName, ".ts") ||
		strings.HasSuffix(lowerName, ".mts") || strings.HasSuffix(lowerName, ".m2ts") ||
		strings.HasSuffix(lowerName, ".ogg") || strings.HasSuffix(lowerName, ".yuv") ||
		strings.HasSuffix(lowerName, ".rm") || strings.HasSuffix(lowerName, ".rmvb") ||
		strings.HasSuffix(lowerName, ".viv") || strings.HasSuffix(lowerName, ".asf") ||
		strings.HasSuffix(lowerName, ".amv") || strings.HasSuffix(lowerName, ".svi") ||
		strings.HasSuffix(lowerName, ".mxf") || strings.HasSuffix(lowerName, ".roq") ||
		strings.HasSuffix(lowerName, ".nsv") {
		// log.Printf("Its WMV/AVI/MKV... : %s", fileName)
		fi, err := os.Stat(fpath)
		if err == nil {
			videoChan <- FileObject{Name: fileName, Path: filepath.Dir(fpath), DateTime: fi.ModTime(), Md5Sum: md5sum}
		}
	}
}

func processMp4Mov(ctx context.Context, fpath string, fileName string, md5sum string, videoChan chan<- FileObject, erroredChan chan<- ErroredFileObject) {
	videoBuffer, err := os.Open(fpath)
	if err != nil {
		erroredChan <- ErroredFileObject{
			DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
			ErrorMessage: err.Error(),
		}
		return
	}
	defer videoBuffer.Close()

	buf := make([]byte, 8)

	// Traverse videoBuffer to find movieResourceAtom
	for {
		if _, err := videoBuffer.Read(buf); err != nil {
			if err.Error() == "EOF" {
				fi, err := os.Stat(fpath)
				if err == nil {
					videoChan <- FileObject{Name: fileName, Path: filepath.Dir(fpath), DateTime: fi.ModTime(), Md5Sum: md5sum}
				}
			} else {
				erroredChan <- ErroredFileObject{
					DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
					ErrorMessage: err.Error(),
				}
			}
			return
		}

		if bytes.Equal(buf[4:8], []byte(movieResourceAtomType)) {
			break // found it!
		}

		atomSize := binary.BigEndian.Uint32(buf)
		if atomSize < 8 {
			// Invalid atom size or extended size (1) or EOF (0) which we don't support fully here.
			// Just fallback to file time if we can't parse structure.
			fi, err := os.Stat(fpath)
			if err == nil {
				videoChan <- FileObject{Name: fileName, Path: filepath.Dir(fpath), DateTime: fi.ModTime(), Md5Sum: md5sum}
			}
			return
		}
		videoBuffer.Seek(int64(atomSize)-8, 1)
	}

	// read next atom
	if _, err := videoBuffer.Read(buf); err != nil {
		erroredChan <- ErroredFileObject{
			DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
			ErrorMessage: err.Error(),
		}
		return
	}

	atomType := string(buf[4:8])
	switch atomType {
	case movieHeaderAtomType:
		if _, err := videoBuffer.Read(buf); err != nil {
			erroredChan <- ErroredFileObject{
				DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
				ErrorMessage: err.Error(),
			}
			return
		}
		appleEpoch := int64(binary.BigEndian.Uint32(buf[4:]))
		tm := time.Unix(appleEpoch-appleEpochAdjustment, 0).Local()
		videoChan <- FileObject{Name: fileName, Path: filepath.Dir(fpath), DateTime: tm, Md5Sum: md5sum}
	default:
		erroredChan <- ErroredFileObject{
			DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
			ErrorMessage: atomType + " is not a valid atom type",
		}
	}
}

func processMpg(ctx context.Context, fpath string, fileName string, md5sum string, videoChan chan<- FileObject, erroredChan chan<- ErroredFileObject) {
	videoBuffer, err := os.Open(fpath)
	if err != nil {
		erroredChan <- ErroredFileObject{
			DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
			ErrorMessage: err.Error(),
		}
		return
	}
	defer videoBuffer.Close()

	// Read first 4 bytes for Pack Header Start Code 0x000001BA
	buf := make([]byte, 4)
	if _, err := videoBuffer.Read(buf); err != nil {
		if err == io.EOF {
			fi, err := os.Stat(fpath)
			if err == nil {
				videoChan <- FileObject{Name: fileName, Path: filepath.Dir(fpath), DateTime: fi.ModTime(), Md5Sum: md5sum}
			} else {
				erroredChan <- ErroredFileObject{
					DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
					ErrorMessage: err.Error(),
				}
			}
		} else {
			erroredChan <- ErroredFileObject{
				DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
				ErrorMessage: err.Error(),
			}
		}
		return
	}

	// 00 00 01 BA
	if bytes.Equal(buf, []byte{0x00, 0x00, 0x01, 0xBA}) {
		fi, err := os.Stat(fpath)
		if err == nil {
			videoChan <- FileObject{Name: fileName, Path: filepath.Dir(fpath), DateTime: fi.ModTime(), Md5Sum: md5sum}
		}
	} else {
		erroredChan <- ErroredFileObject{
			DateTime: time.Now(), Name: fileName, Path: filepath.Dir(fpath),
			ErrorMessage: "Invalid MPG file header",
		}
	}
}

// files, err := ioutil.ReadDir(src_dirname)
// if err != nil {
// 	panic(err)
// }

// imageFiles := []FileObject{}
// erroredFiles := []ErroredFileObject{}

// for _, file := range files {
// 	fpath := path.Join(src_dirname, file.Name())
// 	ext := strings.ToLower(fpath[len(fpath)-4:])

// 	if ext == ".mp4" || ext == ".mov" {
// 		videoBuffer, err := os.Open(fpath)
// 		if err != nil {
// 			panic(err)
// 		}
// 		defer videoBuffer.Close()

// 		buf := make([]byte, 8)
// 		failed := false

// 		// Traverse videoBuffer to find movieResourceAtom
// 		for {
// 			// bytes 1-4 is atom size, 5-8 is type
// 			// Read atom
// 			if _, err := videoBuffer.Read(buf); err != nil {
// 				erroredFiles = append(erroredFiles, ErroredFileObject{
// 					DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 					ErrorMessage: err.Error(),
// 				})
// 				failed = true
// 				break
// 			}

// 			if bytes.Equal(buf[4:8], []byte(movieResourceAtomType)) {
// 				break // found it!
// 			}

// 			atomSize := binary.BigEndian.Uint32(buf) // check size of atom
// 			videoBuffer.Seek(int64(atomSize)-8, 1)   // jump over data and set seeker at beginning of next atom
// 		}

// 		if failed {
// 			continue
// 		}

// 		// read next atom
// 		if _, err := videoBuffer.Read(buf); err != nil {
// 			erroredFiles = append(erroredFiles, ErroredFileObject{
// 				DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 				ErrorMessage: err.Error(),
// 			})
// 			continue
// 		}

// 		atomType := string(buf[4:8]) // skip size and read type
// 		switch atomType {
// 		case movieHeaderAtomType:
// 			// read next atom
// 			if _, err := videoBuffer.Read(buf); err != nil {
// 				erroredFiles = append(erroredFiles, ErroredFileObject{
// 					DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 					ErrorMessage: err.Error(),
// 				})
// 				continue
// 			}

// 			// byte 1 is version, byte 2-4 is flags, 5-8 Creation time
// 			appleEpoch := int64(binary.BigEndian.Uint32(buf[4:])) // Read creation time

// 			tm := time.Unix(appleEpoch-appleEpochAdjustment, 0).Local()
// 			imageFiles = append(imageFiles, FileObject{Name: file.Name(), Path: src_dirname, DateTime: tm})
// 		default:
// 			erroredFiles = append(erroredFiles, ErroredFileObject{
// 				DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
// 				ErrorMessage: atomType + " is not a valid atom type",
// 			})
// 		}
// 	}
// }
// return imageFiles, erroredFiles
