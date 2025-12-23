package icopy

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
)

type MatchObject struct {
	SrcFileName string `json:"src_file_name"`
	DstFileName string `json:"dst_file_name"`
}

func ScanFiles(ctx context.Context, src_dirname string, dst_dirname string, options ScanOptions) {
	logger := ctx.Value("logger").(zerolog.Logger)
	db, err := OpenBadgerDB("./badger")
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to open badger db")
	}

	ScanAndGenerateMd5sumFiles(ctx, db, src_dirname, "src", options)
	ScanAndGenerateMd5sumFiles(ctx, db, dst_dirname, "dst", options)

	CloseBadgerDB(db)
}

func ScanAndGenerateMd5sumFiles(ctx context.Context, db *badger.DB, dirname string, prefix string, options ScanOptions) {
	logger := ctx.Value("logger").(zerolog.Logger)

	jobs := make(chan string)
	var wg sync.WaitGroup

	numWorkers := options.NumWorkers
	if numWorkers <= 0 {
		numWorkers = 10
	}

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if options.ProgressChan != nil {
					select {
					case options.ProgressChan <- fmt.Sprintf("Scanning: %s", filepath.Base(path)):
					default:
					}
				}
				md5sum, err := ComputeFileHash(path, options.UseFastHash)
				if err != nil {
					logger.Error().Err(err).Msgf("Failed to calculate md5sum for file: %s", path)
					continue
				}
				PutBadgerDB(db, prefix+"-"+md5sum, path)
			}
		}()
	}

	// Walk directory and send jobs
	err := filepath.WalkDir(dirname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Error().Err(err).Msgf("Error walking path: %s", path)
			return nil
		}
		if !d.IsDir() {
			jobs <- path
		}
		return nil
	})

	if err != nil {
		logger.Error().Err(err).Msg("Error walking directory")
	}

	close(jobs)
	wg.Wait()
}

func ValidateMd5sumFiles(ctx context.Context, src_prefix string, dst_prefix string) []MatchObject {
	logger := ctx.Value("logger").(zerolog.Logger)

	db, err := OpenBadgerDB("./badger")
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to open badger db")
	}

	src_files, err := IterateWithPrefix(db, src_prefix)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to iterate with prefix")
	}

	dst_files, err := IterateWithPrefix(db, dst_prefix)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to iterate with prefix")
	}
	fmt.Println("")
	logger.Info().Msgf("Scanned Sources: %d", len(src_files))
	logger.Info().Msgf("Scanned Destinations: %d", len(dst_files))

	matches := findMatchesOptimized(src_files, dst_files)

	logger.Info().Msgf("Found %d matches", len(matches))

	matchedFiles := getMatchedFiles(ctx, db, matches, src_prefix, dst_prefix)

	CloseBadgerDB(db)
	return matchedFiles
}

func getMatchedFiles(ctx context.Context, db *badger.DB, matches []string, src_prefix string, dst_prefix string) []MatchObject {
	// logger := ctx.Value("logger").(zerolog.Logger)

	matchedFiles := []MatchObject{}

	for _, match := range matches {
		srcValue, _ := GetBadgerDBValue(db, src_prefix+"-"+match)
		dstValue, _ := GetBadgerDBValue(db, dst_prefix+"-"+match)
		matchedFiles = append(matchedFiles, MatchObject{SrcFileName: srcValue, DstFileName: dstValue})
	}
	return matchedFiles
}

func findMatchesOptimized(arr1, arr2 []string) []string {
	// Determine the smaller array for better memory usage
	if len(arr1) > len(arr2) {
		arr1, arr2 = arr2, arr1
	}

	matches := []string{}
	seen := make(map[string]struct{}, len(arr1)) // Preallocate map capacity

	// Add elements of the smaller array to the map
	for _, str := range arr1 {
		seen[str] = struct{}{}
	}

	// Check if elements of the larger array exist in the map
	for _, str := range arr2 {
		if _, exists := seen[str]; exists {
			matches = append(matches, str)
		}
	}

	return matches
}
