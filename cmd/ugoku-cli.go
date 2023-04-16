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

func startDownloaders(downloader_config config.DownloaderConfig) {

	downloader.NewSimpleDownloader(downloader_config)

	main_logger.Info(fmt.Sprintf("started downloader"))
}

// --------------------------

func init() {
	logger.Init("", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")

	// ex, err := os.Executable()
	// if err != nil {
	// 	main_logger.Error("unable to get executable path")
	// 	os.Exit(1)
	// }

	// {
	// 	var err error
	// 	config_path := filepath.Join(filepath.Dir(ex), "config.yaml")
	// 	master_config, err = config.ReadConfig(config_path)
	// 	if err != nil {
	// 		main_logger.Error(fmt.Sprintf("failed to read config: %v", err))
	// 	}
	// }
}

// --------------------------

func main() {

	main_logger.Info(fmt.Sprintf("Ugoku CLI, version %s", VERSION))

	// go startDownloaders(master_config)

	// <-make(chan struct{})
}
