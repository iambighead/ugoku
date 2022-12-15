package main

import (
	"fmt"

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
			downloader.NewDownloader(downloader_config, master_config.General.TempFolder)
			downloader_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d downloaders", downloader_started))
}

func startUploaders(master_config config.MasterConfig) {

	uploader_started := 0
	for _, uploader_config := range master_config.Uploaders {
		if uploader_config.Enabled {
			uploader.NewUploader(uploader_config, master_config.General.TempFolder)
			uploader_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d uploaders", uploader_started))
}

func startSyncers(master_config config.MasterConfig) {

	syncer_started := 0
	for _, syncer_config := range master_config.Syncers {
		if syncer_config.Enabled {
			syncer.NewSyncer(syncer_config, master_config.General.TempFolder)
			syncer_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d uploaders", syncer_started))
}

func startStreamers(master_config config.MasterConfig) {

	streamer_started := 0
	for _, streamer_config := range master_config.Streamers {
		if streamer_config.Enabled {
			streamer.NewStreamer(streamer_config)
			streamer_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d streamers", streamer_started))
}

// --------------------------

func init() {
	logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")

	var err error
	master_config, err = config.ReadConfig("config.yaml")
	if err != nil {
		main_logger.Error(fmt.Sprintf("failed to read config: %v", err))
	}

}

// --------------------------

func main() {

	main_logger.Info(fmt.Sprintf("Ugoku started. Version %s", VERSION))

	go startDownloaders(master_config)
	go startUploaders(master_config)
	go startSyncers(master_config)
	go startStreamers(master_config)

	<-make(chan struct{})
}
