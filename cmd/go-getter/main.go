package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	getter "github.com/hashicorp/go-getter"
	"github.com/rs/zerolog"
)

func main() {
	modeRaw := flag.String("mode", "any", "get mode (any, file, dir)")
	progress := flag.Bool("progress", false, "display terminal progress")
	logLevel := flag.String("log", "warn", "log level: debug, info, warn, err (default: warn)")
	flag.Parse()
	args := flag.Args()

	// Get the mode
	switch *logLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log := zerolog.New(output).With().Timestamp().Logger()

	// Generate file nae if not given
	fileName := ""
	if len(args) < 2 {
		fileName = filepath.Base(args[0])
	} else {
		fileName = args[1]
	}

	// Get the mode
	var mode getter.ClientMode
	switch *modeRaw {
	case "any":
		mode = getter.ClientModeAny
	case "file":
		mode = getter.ClientModeFile
	case "dir":
		mode = getter.ClientModeDir
	default:
		log.Fatal().Msgf("Invalid client mode, must be 'any', 'file', or 'dir': %s", *modeRaw)
		os.Exit(1)
	}

	// Get the pwd
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal().Msgf("Error getting wd: %s", err)
	}

	opts := []getter.ClientOption{}
	if *progress {
		opts = append(opts, getter.WithProgress(defaultProgressBar))
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Build the client
	client := &getter.Client{
		Ctx:     ctx,
		Src:     args[0],
		Dst:     fileName,
		Pwd:     pwd,
		Mode:    mode,
		Options: opts,
	}

	log.Info().Msgf("Downloading '%s' as '%s'", args[0], *modeRaw)
	wg := sync.WaitGroup{}
	wg.Add(1)
	errChan := make(chan error, 2)
	go func() {
		defer wg.Done()
		defer cancel()
		if err := client.Get(); err != nil {
			errChan <- err
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	select {
	case sig := <-c:
		signal.Reset(os.Interrupt)
		cancel()
		wg.Wait()
		log.Info().Msgf("signal %v", sig)
	case <-ctx.Done():
		wg.Wait()
		log.Info().Msgf("success!")
	case err := <-errChan:
		wg.Wait()
		log.Fatal().Msgf("Error downloading: %s", err)
	}
}
