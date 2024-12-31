package icopy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
)

type FileProcessor struct {
	Overwrite string
	ForceCopy bool
	Recursive bool
	DateFmt   string
}

func (fp *FileProcessor) CopyImageFiles(ctx context.Context, srcdir string, destdir string) ([]FileObject, []ErroredFileObject, []FileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)

	db, err := OpenBadgerDB("./badger")
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to open badger db")
	}

	imagefiles, erroredfiles := ReadJpegDate(ctx, db, srcdir, fp.Recursive)

	ScanAndGenerateMd5sumFiles(ctx, db, destdir, "dst")

	SortFilesByDate(imagefiles)

	if !fp.ForceCopy {
		imagefiles = checkStatusFile(ctx, imagefiles)
		if len(imagefiles) == 0 {
			logger.Info().Msg("No valid image files to copy")
			return nil, erroredfiles, nil
		}
	}

	filesCopied, erroredfiles1, skipedfiles := fp.copy_file(ctx, db, imagefiles, destdir)
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

	videofiles, erroredfiles := ReadVideoCreationTimeMetadata(ctx, db, srcdir, fp.Recursive)

	SortFilesByDate(videofiles)

	if !fp.ForceCopy {
		videofiles = checkStatusFile(ctx, videofiles)
		if len(videofiles) == 0 {
			logger.Info().Msg("No valid video files to copy")
			return nil, erroredfiles, nil
		}
	}

	filesCopied, erroredfiles1, skipedfiles := fp.copy_file(ctx, db, videofiles, destdir)
	erroredfiles = append(erroredfiles, erroredfiles1...)

	CloseBadgerDB(db)

	return filesCopied, erroredfiles, skipedfiles
}

func (fp *FileProcessor) copy_file(ctx context.Context, db *badger.DB, imagefiles []FileObject, destdir string) ([]FileObject, []ErroredFileObject, []FileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	logger.Info().Msgf("Total files expected to copy ...%d", len(imagefiles))

	i := 1
	filesCopied := []FileObject{}
	erroredFiles := []ErroredFileObject{}
	skipedfiles := []FileObject{}

	for _, image := range imagefiles {
		tm := image.DateTime
		fpath := path.Join(image.Path, image.Name)

		value, err := GetBadgerDBValue(db, "dst-"+image.Md5Sum)
		if (err == nil || value != "") && fp.Overwrite == "no" {
			skipedfiles = append(skipedfiles, FileObject{Path: image.Path, Name: image.Name, DateTime: tm})
			continue
		}

		fis, _ := os.Stat(fpath)

		fd, err := os.Open(fpath) // Open the source file for reading
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to open file: %s", fpath)
			erroredFiles = append(erroredFiles, ErroredFileObject{Path: image.Path, Name: image.Name, DateTime: tm, ErrorMessage: err.Error()})
			continue
		}
		defer fd.Close()

		// Create the destination file for writing
		fYMdir := getDestinationPath(tm, destdir, fp.DateFmt)
		if os.MkdirAll(fYMdir, 0755) != nil {
			logger.Error().Err(err).Msgf("Failed to create directory: %s", fYMdir)
			panic(err)
		}
		fYMpath := path.Join(fYMdir, strings.ReplaceAll(image.Name, "%20", "_"))
		fYMpath = strings.ReplaceAll(fYMpath, " ", "_")

		if fi, err := os.Stat(fYMpath); err == nil {
			logger.Info().Msgf("File already exists: %s", fYMpath)
			if !fi.IsDir() {
				if strings.ToLower(fp.Overwrite) == "yes" {
					logger.Info().Msgf("Overwriting file %s", fYMpath)
				} else if strings.ToLower(fp.Overwrite) == "no" {
					logger.Info().Msgf("Skipping file %s", fYMpath)
					skipedfiles = append(skipedfiles, FileObject{Path: fYMdir, Name: image.Name, DateTime: tm})
					continue
				} else if strings.ToLower(fp.Overwrite) == "ask" {
					logger.Info().Msgf("File exists, do you want to overwrite? (y/n) :")

					var answer string
					fmt.Scanln(&answer)
					if strings.ToLower(answer) == "y" {
						logger.Info().Msgf("Overwriting file %s", fYMpath)
					} else {
						logger.Info().Msgf("Skipping file %s", fYMpath)
						skipedfiles = append(skipedfiles, FileObject{Path: fYMdir, Name: image.Name, DateTime: tm})
						continue
					}
				} else {
					logger.Info().Msgf("Skipping file %s", fYMpath)
					skipedfiles = append(skipedfiles, FileObject{Path: fYMdir, Name: image.Name, DateTime: tm})
					continue
				}

				filesCopied = writeFile(fYMpath, logger, fd, fi, ctx, image, fYMdir, i, imagefiles, filesCopied, tm)
				i++
			}
		} else {
			logger.Info().Msgf("File does not exist, so create a file %s", fYMpath)
			filesCopied = writeFile(fYMpath, logger, fd, fis, ctx, image, fYMdir, i, imagefiles, filesCopied, tm)
			i++
		}
	}
	logger.Info().Msgf("Total Files Copied: %d/%d", len(filesCopied), len(imagefiles))
	return filesCopied, erroredFiles, skipedfiles
}

func writeFile(fYMpath string, logger zerolog.Logger, fd *os.File, fi fs.FileInfo, ctx context.Context, image FileObject, fYMdir string, i int, imagefiles []FileObject, filesCopied []FileObject, tm time.Time) []FileObject {
	fwout, err := os.Create(fYMpath)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create file: %s", fYMpath)
	}
	defer fwout.Close()

	if _, err := io.Copy(fwout, fd); err != nil {
		logger.Fatal().Err(err).Msgf("Failed to copy file: %s", fYMpath)
	}
	err = fwout.Close()
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to close file: %s", fYMpath)
	}
	err = os.Chtimes(fYMpath, fi.ModTime(), fi.ModTime())
	if err != nil {
		logger.Info().Msgf("Failed to change time of file: %s", fYMpath)
	}

	writeStatusFile(ctx, image)
	logger.Info().Msgf("Copied %s to %s: %d of %d", image.Name, fYMdir, i, len(imagefiles))
	filesCopied = append(filesCopied, FileObject{Path: fYMdir, Name: image.Name, DateTime: tm})
	return filesCopied
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
	return path.Join(destdir, dPath)
}
