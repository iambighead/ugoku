package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
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

func main() {

	main_logger.Info(fmt.Sprintf("Ugoku-cli started. Version %s", VERSION))

	cmdFlag := flag.String("cmd", "download", "ugoku command to run, default 'download'")
	flag.Parse()

	main_logger.Info(fmt.Sprintf("Ugoku-cli command to run = %s", *cmdFlag))

	switch *cmdFlag {
	case "upload":
		go startUploaders(master_config)
		break
	case "download":
		go startDownloaders(master_config)
		break
	case "sync":
		go startSyncers(master_config)
		break
	default:
		main_logger.Info("do not know how to proceed, exiting")
		os.Exit(0)
	}

	<-make(chan struct{})
}
