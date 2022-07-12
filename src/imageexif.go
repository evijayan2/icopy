package icopy

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rwcarlsen/goexif/exif"
)

func ReadJpegDate(ctx context.Context, src_dirname string, recursive bool) ([]FileObject, []ErroredFileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	logger.Info().Msg("Reading image files")
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
				logger.Info().Msgf("Recursively reading directory: %s", file.Name())
				imageFiles1, erroredFiles1 := ReadJpegDate(ctx, path.Join(src_dirname, file.Name()), recursive)
				imageFiles = append(imageFiles, imageFiles1...)
				erroredFiles = append(erroredFiles, erroredFiles1...)
			}
		} else {
			logger.Info().Msgf("Reading file: %s", file.Name())
			fpath := path.Join(src_dirname, file.Name())
			if strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(file.Name()), ".jpeg") {
				logger.Info().Msgf("Reading file: %s", file.Name())

				fd, err := os.Open(fpath) // Open the source file for reading
				if err != nil {
					logger.Panic().Err(err).Msgf("Failed to open file: %s", fpath)
				}
				defer fd.Close()
				x, err := exif.Decode(fd)
				if err != nil {
					errMsg := err.Error()
					logger.Info().Msgf("Error: %s", errMsg)
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
				imageFiles = append(imageFiles, FileObject{DateTime: tm, Name: file.Name(), Path: src_dirname})
			} else if strings.HasSuffix(strings.ToLower(file.Name()), ".gif") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".png") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".bmp") ||
				strings.HasSuffix(strings.ToLower(file.Name()), ".heic") {
				logger.Info().Msgf("Reading Other file: %s", file.Name())

				fi, err := os.Stat(fpath)
				if err != nil {
					logger.Panic().Err(err).Msgf("Failed to open file: %s", fpath)
				}
				tm = fi.ModTime()
				imageFiles = append(imageFiles, FileObject{DateTime: tm, Name: file.Name(), Path: src_dirname})
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
