package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	icopy "github.com/evijayan2/icopy/src"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	scan          = flag.Bool("scan", false, "Scan and generate md5sum files. (true/false)")
	video         = flag.Bool("video", false, "Read video creation date time metadata. (true/false)")
	image         = flag.Bool("image", false, "Read image creation date time metadata. (true/false)")
	remove_source = flag.Bool("removesource", false, "Remove source files after copying. (true/false)")
	outdir_fmt    = flag.String("dirformat", "NOF", "DATE or YEAR-MONTH or NOF (No Format/Preserve Original)")
	outdir        = flag.String("out", ".", "Output directory")
	indir         = flag.String("in", "", "Input directory")
	recursive     = flag.Bool("recursive", false, "Recursively copy files. (true/false)")
	forceCopy     = flag.Bool("force", false, "Force copy of files. (true/false)")
	overwrite     = flag.String("overwrite", "no", "Overwrite existing files. (yes/no/ask)")
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.With().Caller().Logger()

	file, err := os.OpenFile("custom.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open log file")
	}
	defer file.Close()

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	multi := zerolog.MultiLevelWriter(consoleWriter, file)
	logger := zerolog.New(multi).
		With().
		Timestamp().
		Caller().
		Logger()

	ctx := context.WithValue(context.Background(), "logger", logger)

	handleSigtem(ctx)
	flag.Parse()

	if *video && *image {
		error(ctx, "Only one of -video or -image can be specified. Exiting.")
	}

	if *indir == "" {
		error(ctx, "No input directory specified. Exiting.")
	}

	imageFiles := []icopy.FileObject{}
	erroredFiles := []icopy.ErroredFileObject{}
	skippedFiles := []icopy.FileObject{}
	matchedFiles := []icopy.MatchObject{}

	fp := icopy.FileProcessor{
		Overwrite: *overwrite,
		ForceCopy: *forceCopy,
		Recursive: *recursive,
		DateFmt:   *outdir_fmt,
	}

	if *scan {
		logger.Info().Msg("Scanning and generating md5sum files")
		var wg sync.WaitGroup
		stopChan := make(chan struct{})

		wg.Add(1)
		go showSpinner(stopChan, &wg)
		icopy.ScanFiles(ctx, *indir, *outdir)
		matchedFiles = icopy.ValidateMd5sumFiles(ctx, "src", "dst")
		close(stopChan)
		wg.Wait()
	} else if *video {
		logger.Info().Msgf("Reading video creation time metadata")
		imageFiles, erroredFiles, skippedFiles = fp.CopyVideoFiles(ctx, *indir, *outdir)
	} else if *image {
		logger.Info().Msgf("Reading image creation time metadata")
		imageFiles, erroredFiles, skippedFiles = fp.CopyImageFiles(ctx, *indir, *outdir)
	} else {
		error(ctx, "No input specified. Exiting.")
	}

	PrintM(ctx, "Files matched", matchedFiles)
	Print(ctx, "Files copied", imageFiles)
	Print(ctx, "Skipped", skippedFiles)
	PrintE(ctx, "Errors", erroredFiles)

	if *remove_source {
		logger.Info().Msg("Removing copied source files...")
		removedFiles := icopy.RemoveSourceFile(*indir)

		Print(ctx, "Removed files:", removedFiles)
	}

	os.RemoveAll("./badger")

	fmt.Println("")
}

func showSpinner(stopChan <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	spinner := []string{"|", "/", "-", "\\"}
	i := 0
	for {
		select {
		case <-stopChan:
			fmt.Print("\r\033[K") // Clear the spinner
			return
		default:
			fmt.Printf("\rProcessing... %s", spinner[i])
			i = (i + 1) % len(spinner)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func error(ctx context.Context, msg string) {
	logger := ctx.Value("logger").(zerolog.Logger)
	logger.Info().Msg(msg)
	flag.Usage()
	os.Exit(1)
}

func Print(ctx context.Context, msg string, files []icopy.FileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	if len(files) > 0 {
		logger.Info().Msg("")
		logger.Info().Msg("------------------------------------------------------------")
		logger.Info().Msgf("%s: %d", msg, len(files))
		logger.Info().Msg("------------------------------------------------------------")
		// for _, f := range files {
		// 	logger.Info().Msgf("File %s => Date %s ", path.Join(f.Path, f.Name), f.DateTime.Format("2006-01-02"))
		// }
	}
}

func PrintM(ctx context.Context, msg string, files []icopy.MatchObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	if len(files) > 0 {
		logger.Info().Msg("")
		logger.Info().Msg("------------------------------------------------------------")
		logger.Info().Msgf("%s: %d", msg, len(files))
		logger.Info().Msg("------------------------------------------------------------")
		for _, f := range files {
			logger.Info().Msgf("Source %s => Destination %s ", f.SrcFileName, f.DstFileName)
		}
	}
}

func PrintE(ctx context.Context, msg string, files []icopy.ErroredFileObject) {
	logger := ctx.Value("logger").(zerolog.Logger)
	if len(files) > 0 {
		logger.Info().Msg("")
		logger.Info().Msg("------------------------------------------------------------")
		logger.Info().Msgf("%s: %d", msg, len(files))
		logger.Info().Msg("------------------------------------------------------------")
		for _, f := range files {
			logger.Info().Msgf("File %s => Date %s => %s ", path.Join(f.Path, f.Name), f.DateTime.Format("2006-01-02"), f.ErrorMessage)
		}
	}
}

func handleSigtem(ctx context.Context) {
	logger := ctx.Value("logger").(zerolog.Logger)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM) // SIGINT, SIGKILL, SIGTERM
	go func() {
		<-c

		logger.Info().Msg("SIGTERM received. Exiting.")
		os.Exit(0)
	}()
}
