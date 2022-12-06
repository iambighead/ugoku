package main

import (
	"fmt"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
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

	<-make(chan struct{})
}
