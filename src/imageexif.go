package icopy

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
	"github.com/rwcarlsen/goexif/exif"
)

func ReadJpegDate(ctx context.Context, db *badger.DB, src_dirname string, recursive bool) ([]FileObject, []ErroredFileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	de, err := os.ReadDir(src_dirname)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to read directory")
	}

	imageFiles := []FileObject{}
	erroredFiles := []ErroredFileObject{}

	tm := time.Time{}

	for _, file := range de {
		if file.IsDir() {
			if recursive {
				imageFiles1, erroredFiles1 := ReadJpegDate(ctx, db, path.Join(src_dirname, file.Name()), recursive)
				imageFiles = append(imageFiles, imageFiles1...)
				erroredFiles = append(erroredFiles, erroredFiles1...)
			}
		} else {
			fpath := path.Join(src_dirname, file.Name())
			if strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(file.Name()), ".jpeg") {

				md5sum, err := Md5Sum(fpath)
				if err != nil {
					logger.Panic().Err(err).Msgf("Failed to calculate md5sum for file: %s", fpath)
				}
				PutBadgerDB(db, "src-"+md5sum, fpath)

				fd, err := os.Open(fpath) // Open the source file for reading
				if err != nil {
					logger.Panic().Err(err).Msgf("Failed to open file: %s", fpath)
				}
				defer fd.Close()
				x, err := exif.Decode(fd)
				if err != nil {
					errMsg := err.Error()
					if errMsg == "EOF" {
						fi, err := os.Stat(fpath)
						if err == nil {
							tm = fi.ModTime()
						}
					} else if strings.HasPrefix(strings.TrimSpace(errMsg), "exif: error reading 4 byte header") {
						fi, err := os.Stat(fpath)
						if err == nil {
							tm = fi.ModTime()
						}
					} else {
						erroredFiles = append(erroredFiles, ErroredFileObject{
							DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
							ErrorMessage: err.Error(),
						})
						continue
					}
				} else {
					tm, _ = x.DateTime()
				}
				imageFiles = append(imageFiles, FileObject{DateTime: tm, Name: file.Name(), Path: src_dirname, Md5Sum: md5sum})
			} else if strings.HasSuffix(strings.ToLower(file.Name()), ".gif") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".png") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".bmp") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".heic") {

				fi, err := os.Stat(fpath)
				if err != nil {
					logger.Panic().Err(err).Msgf("Failed to open file: %s", fpath)
				}

				md5sum, err := Md5Sum(fpath)
				if err != nil {
					logger.Panic().Err(err).Msgf("Failed to calculate md5sum for file: %s", fpath)
				}

				PutBadgerDB(db, "src-"+md5sum, fpath)

				tm = fi.ModTime()
				imageFiles = append(imageFiles, FileObject{DateTime: tm, Name: file.Name(), Path: src_dirname, Md5Sum: md5sum})
			}
		}
	}
	return imageFiles, erroredFiles

	// files, err := ioutil.ReadDir(src_dirname)
	// if err != nil {
	// 	panic(err)
	// }

	// imageFiles := []FileObject{}
	// erroredFiles := []ErroredFileObject{}

	// tm := time.Time{}

	// for _, file := range files {
	// 	fpath := path.Join(src_dirname, file.Name())
	// 	ext := strings.ToLower(fpath[len(fpath)-4:])

	// 	if ext == ".jpg" || ext == "jpeg" {

	// 		f, err := os.Open(fpath)
	// 		if err != nil {
	// 			log.Fatal("Failed to read file", err)
	// 			panic(err)
	// 		}
	// 		defer f.Close()
	// 		x, err := exif.Decode(f)
	// 		if err != nil {
	// 			errMsg := err.Error()
	// 			logger.Info().Msgf("Error:", errMsg)
	// 			if errMsg == "EOF" {
	// 				fi, err := os.Stat(fpath)
	// 				if err == nil {
	// 					tm = fi.ModTime()
	// 				}
	// 			} else {
	// 				erroredFiles = append(erroredFiles, ErroredFileObject{
	// 					DateTime: time.Now(), Name: file.Name(), Path: src_dirname,
	// 					ErrorMessage: err.Error(),
	// 				})
	// 				continue
	// 			}
	// 		} else {
	// 			tm, _ = x.DateTime()
	// 		}
	// 		imageFiles = append(imageFiles, FileObject{DateTime: tm, Name: file.Name(), Path: src_dirname})

	// 	}
	// }
	// return imageFiles, erroredFiles
}
