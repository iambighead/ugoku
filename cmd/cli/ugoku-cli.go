package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/streamer"
	"github.com/iambighead/ugoku/syncer"
	"github.com/iambighead/ugoku/uploader"
)

const VERSION = "v0.0.1"

// --------------------------------

var main_logger logger.Logger
var master_config config.MasterConfig

func startDownloaders(master_config config.MasterConfig) {

	downloader_started := 0
	for _, downloader_config := range master_config.Downloaders {
		if downloader_config.Enabled {
			downloader.NewOneTimeDownloader(downloader_config, master_config.General.TempFolder)
			downloader_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d downloaders", downloader_started))

	if downloader_started == 0 {
		os.Exit(0)
	}
}

func startUploaders(master_config config.MasterConfig) {

	uploader_started := 0
	for _, uploader_config := range master_config.Uploaders {
		if uploader_config.Enabled {
			uploader.NewOneTimeUploader(uploader_config, master_config.General.TempFolder)
			uploader_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d uploaders", uploader_started))

	if uploader_started == 0 {
		os.Exit(0)
	}
}

func startSyncers(master_config config.MasterConfig) {

	syncer_started := 0
	for _, syncer_config := range master_config.Syncers {
		if syncer_config.Enabled {
			syncer.NewOneTimeSyncer(syncer_config, master_config.General.TempFolder)
			syncer_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d syncers", syncer_started))

	if syncer_started == 0 {
		os.Exit(0)
	}
}

func startStreamers(master_config config.MasterConfig) {

	streamer_started := 0
	for _, streamer_config := range master_config.Streamers {
		if streamer_config.Enabled {
			streamer.NewOneTimeStreamer(streamer_config)
			streamer_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d streamers", streamer_started))

	if streamer_started == 0 {
		os.Exit(0)
	}
}

// --------------------------

func init() {
	logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")

	ex, err := os.Executable()
	if err != nil {
		main_logger.Error("unable to get executable path")
		os.Exit(1)
	}

	{
		var err error
		config_path := filepath.Join(filepath.Dir(ex), "config.yaml")
		master_config, err = config.ReadConfig(config_path)
		if err != nil {
			main_logger.Error(fmt.Sprintf("failed to read config: %v", err))
		}
	}
}

// --------------------------

func printUsage() {
	main_logger.Info(fmt.Sprintf("Usage:"))
	main_logger.Info(fmt.Sprintf(""))
	main_logger.Info(fmt.Sprintf("  ugoku-cli <command>"))
	main_logger.Info(fmt.Sprintf(""))
	main_logger.Info(fmt.Sprintf("command can be upload, download, sync"))
	main_logger.Info(fmt.Sprintf(""))
	main_logger.Info(fmt.Sprintf("Example:"))
	main_logger.Info(fmt.Sprintf(""))
	main_logger.Info(fmt.Sprintf("  ugoku-cli sync"))
}

func main() {

	main_logger.Info(fmt.Sprintf("ugoku-cli version %s", VERSION))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "upload":
		go startUploaders(master_config)
		break
	case "download":
		go startDownloaders(master_config)
		break
	case "sync":
		go startSyncers(master_config)
		break
	case "stream":
		go startStreamers(master_config)
		break
	default:
		main_logger.Error("Missing or unknown command")
		printUsage()
		os.Exit(0)
	}

	<-make(chan struct{})
}
