package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/syncer"
)

const VERSION = "v0.0.1"

// --------------------------------

var main_logger logger.Logger
var master_config config.MasterConfig

func startSyncers(master_config config.MasterConfig) {

	syncer_started := 0
	for _, syncer_config := range master_config.Syncers {
		if syncer_config.Enabled {
			syncer.NewOneTimeSyncer(syncer_config, master_config.General.TempFolder)
			syncer_started++
		}
	}

	main_logger.Info(fmt.Sprintf("started %d syncers", syncer_started))
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

	main_logger.Info(fmt.Sprintf("Ugoku Sync started. Version %s", VERSION))

	go startSyncers(master_config)

	<-make(chan struct{})
}
