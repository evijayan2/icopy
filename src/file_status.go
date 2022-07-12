package icopy

import (
	"bufio"
	"context"
	"os"
	"path"

	"github.com/rs/zerolog"
)

func writeStatusFile(ctx context.Context, imagefile FileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	_, err := os.Stat("./.file_status.txt")
	if err != nil {
		if os.IsNotExist(err) {
			os.Create("./.file_status.txt")
			logger.Info().Msg("Created .file_status.txt file")
		}
	}

	fd, err := os.OpenFile("./.file_status.txt", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger.Panic().Err(err).Msg("Error opening .file_status.txt file")
	}
	defer fd.Close()

	fd.WriteString(path.Join(imagefile.Path, imagefile.Name) + "\n")
	err = fd.Close()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to close file")
	}
}

func checkStatusFile(ctx context.Context, imageFiles []FileObject) []FileObject {
	logger := ctx.Value("logger").(zerolog.Logger)
	_, err := os.Stat("./.file_status.txt")
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info().Msg("No .file_status.txt file found")
			return imageFiles
		}
	}
	logger.Info().Msg("Found .file_status.txt file")

	fd, err := os.Open("./.file_status.txt")
	if err != nil {
		logger.Error().Err(err).Msg("Error opening .file_status.txt file")
		return nil
	}
	defer fd.Close()
	scanner := bufio.NewScanner(fd)
	filesCopied := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		filesCopied = append(filesCopied, line)
	}

	rImageFiles := []FileObject{}
	for _, image := range imageFiles {
		if !contains(filesCopied, path.Join(image.Path, image.Name)) {
			rImageFiles = append(rImageFiles, image)
		}
	}
	return rImageFiles
}

func contains(filesCopied []string, s string) bool {
	for _, f := range filesCopied {
		if f == s {
			return true
		}
	}
	return false
}
