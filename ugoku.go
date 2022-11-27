package main

import (
	"fmt"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
)

const VERSION = "v0.0.1"

// --------------------------------

var main_logger logger.Logger
var master_config config.MasterConfig

func startDownloaders(master_config config.MasterConfig) {
	tempfolder = master_config.General.TempFolder
	downloader_started := 0
	for _, downloader_config := range master_config.Downloaders {
		if downloader_config.Enabled {
			// make a channel
			c := make(chan string)
			done := make(chan int)
			var dler SftpDownloader
			dler.DownloaderConfig = downloader_config
			go dler.Start(c, done)
			var scanner SftpScanner
			scanner.DownloaderConfig = downloader_config
			go scanner.Start(c, done)
			downloader_started++
		}
	}
	main_logger.Info(fmt.Sprintf("started %d downloaders", downloader_started))

}

// --------------------------

func init() {
	logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")

	master_config = config.ReadConfig("config.ini", false)
}

// --------------------------

func main() {

	main_logger.Info(fmt.Sprintf("Ugoku started. Version %s", VERSION))

	go startDownloaders(master_config)

	<-make(chan struct{})
}
