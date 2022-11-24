package main

import (
	"fmt"

	"github.com/iambighead/goutils/logger"
)

const VERSION = "v0.0.1"

// --------------------------------

var main_logger logger.Logger

func init() {
	logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")
}

func main() {
	fmt.Printf("ugoku %s\n\n", VERSION)

	main_logger.Info("app started")
}
