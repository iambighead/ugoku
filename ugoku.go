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

func init() {
	logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")

	master_config = config.ReadConfig("config.ini", false)
}

func main() {

	main_logger.Info(fmt.Sprintf("Ugoku started. Version %s", VERSION))

	go startDownloaders(master_config)

	<-make(chan struct{})
}
