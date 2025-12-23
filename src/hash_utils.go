package icopy

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
)

const (
	// FastHashThreshold is the file size limit (50 MB) above which we switch to partial hashing
	FastHashThreshold = 50 * 1024 * 1024
	// ChunkSize is the size of the chunks to read for partial hashing (1 MB)
	ChunkSize = 1 * 1024 * 1024
)

// ComputeFileHash calculates a hash for the file.
// If useFastHash is true and the file is larger than FastHashThreshold,
// it computes a partial hash based on the beginning, middle, and end of the file.
// Otherwise, it computes the full MD5.
func ComputeFileHash(filePath string, useFastHash bool) (string, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if useFastHash && fi.Size() > FastHashThreshold {
		return computePartialHash(filePath, fi.Size())
	}

	return Md5Sum(filePath)
}

// computePartialHash reads the first, middle, and last ChunkSize bytes of the file
// and computes an MD5 checksum of those combined chunks.
func computePartialHash(filePath string, fileSize int64) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()

	// 1. Read start chunk
	// If file is smaller than 3 * ChunkSize, logic shouldn't reach here due to Threshold,
	// but purely safe logic:
	if fileSize <= 3*ChunkSize {
		// Just hash the whole thing if it somehow got here
		if _, err := io.Copy(hash, file); err != nil {
			return "", err
		}
		return fmt.Sprintf("fast-%x", hash.Sum(nil)), nil
	}

	buf := make([]byte, ChunkSize)

	// Start
	if _, err := io.ReadFull(file, buf); err != nil {
		return "", err
	}
	hash.Write(buf)

	// Middle
	if _, err := file.Seek(fileSize/2-int64(ChunkSize/2), 0); err != nil {
		return "", err
	}
	if _, err := io.ReadFull(file, buf); err != nil {
		return "", err
	}
	hash.Write(buf)

	// End
	if _, err := file.Seek(-int64(ChunkSize), 2); err != nil {
		return "", err
	}
	if _, err := io.ReadFull(file, buf); err != nil {
		return "", err
	}
	hash.Write(buf)

	// Append file size to hash to avoid collisions with files having same chunks but diff size (rare but possible)
	fmt.Fprintf(hash, "|%d", fileSize)

	// Prefix with "fast-" to distinguish from full MD5
	return fmt.Sprintf("fast-%x", hash.Sum(nil)), nil
}
