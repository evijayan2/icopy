package icopy

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
)

// Md5Sum returns the MD5 checksum of the file at the given path.
func Md5Sum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
