package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lg "github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/internal/version"
	"github.com/iambighead/ugoku/streamer"
	"github.com/iambighead/ugoku/syncer"
	"github.com/iambighead/ugoku/uploader"
)

const VERSION = version.UGOKU_VERSION

// --------------------------------

var main_logger *lg.Logger
var master_config config.MasterConfig

func startDownloaders(master_config config.MasterConfig) {

	downloader_started := 0
	for _, downloader_config := range master_config.Downloaders {
		if downloader_config.Enabled {
			go downloader.NewOneTimeDownloader(downloader_config, master_config.General.TempFolder, main_logger)
			downloader_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d downloaders", downloader_started))

	if downloader_started == 0 {
		os.Exit(0)
	}
	<-make(chan struct{})
}

func startUploaders(master_config config.MasterConfig) {

	uploader_started := 0
	for _, uploader_config := range master_config.Uploaders {
		if uploader_config.Enabled {
			uploader.NewOneTimeUploader(uploader_config, master_config.General.TempFolder, main_logger)
			uploader_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d uploaders", uploader_started))

	if uploader_started == 0 {
		os.Exit(0)
	}
	<-make(chan struct{})
}

func startSyncers(master_config config.MasterConfig) {

	syncer_started := 0
	for _, syncer_config := range master_config.Syncers {
		if syncer_config.Enabled {
			syncer.NewOneTimeSyncer(syncer_config, master_config.General.TempFolder, main_logger)
			syncer_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d syncers", syncer_started))

	if syncer_started == 0 {
		os.Exit(0)
	}
	<-make(chan struct{})
}

func startStreamers(master_config config.MasterConfig) {

	streamer_started := 0
	for _, streamer_config := range master_config.Streamers {
		if streamer_config.Enabled {
			streamer.NewOneTimeStreamer(streamer_config, main_logger)
			streamer_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d streamers", streamer_started))

	if streamer_started == 0 {
		os.Exit(0)
	}
	<-make(chan struct{})
}

// --------------------------

func startDownloadersService(master_config config.MasterConfig) {

	downloader_started := 0
	for _, downloader_config := range master_config.Downloaders {
		if downloader_config.Enabled {
			downloader.NewDownloader(downloader_config, master_config.General.TempFolder, main_logger)
			downloader_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d downloaders", downloader_started))
}

func startUploadersService(master_config config.MasterConfig) {

	uploader_started := 0
	for _, uploader_config := range master_config.Uploaders {
		if uploader_config.Enabled {
			uploader.NewUploader(uploader_config, master_config.General.TempFolder, main_logger)
			uploader_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d uploaders", uploader_started))
}

func startSyncersService(master_config config.MasterConfig) {

	syncer_started := 0
	for _, syncer_config := range master_config.Syncers {
		if syncer_config.Enabled {
			syncer.NewSyncer(syncer_config, master_config.General.TempFolder, main_logger)
			syncer_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d syncers", syncer_started))
}

func startStreamersService(master_config config.MasterConfig) {

	streamer_started := 0
	for _, streamer_config := range master_config.Streamers {
		if streamer_config.Enabled {
			streamer.NewStreamer(streamer_config, main_logger)
			streamer_started++
		}
	}

	main_logger.Infof(fmt.Sprintf("started %d streamers", streamer_started))
}

func startServices(master_config config.MasterConfig) {
	go startDownloadersService(master_config)
	go startUploadersService(master_config)
	go startSyncersService(master_config)
	go startStreamersService(master_config)
	<-make(chan struct{})
}

// --------------------------

func init() {

	loggerName := "main"
	loggerLogLevel := "info"
	loggerLogOutputFolder := "/tmp"
	loggerRotationBySize := false
	loggerMaxFileSizeMB := 10
	loggervMaxLogFiles := 10
	loggerRotationIntervalHour := 1
	loggerEnableConsoleLog := true
	loggerEnableSyslog := false
	loggerSyslogHost := "localhost"
	loggervSyslogPort := 514
	loggerSyslogProtocol := "udp"

	main_logger = lg.InitLogger(
		loggerName,
		loggerLogLevel,
		loggerLogOutputFolder,
		loggerRotationBySize,
		loggerMaxFileSizeMB,
		loggervMaxLogFiles,
		loggerRotationIntervalHour,
		loggerEnableConsoleLog,
		loggerEnableSyslog,
		loggerSyslogHost,
		loggervSyslogPort,
		loggerSyslogProtocol,
	)

	// logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	// main_logger = logger.NewLogger("main")

	ex, err := os.Executable()
	if err != nil {
		main_logger.Errorf("unable to get executable path")
		os.Exit(1)
	}

	{
		var err error
		config_path := filepath.Join(filepath.Dir(ex), "config.yaml")
		master_config, err = config.ReadConfig(config_path)
		if err != nil {
			main_logger.Errorf("failed to read config: %v", err)
		}
	}
}

// --------------------------

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("")
	fmt.Println("  ugoku <command>")
	fmt.Println("")
	fmt.Println("command can be upload, download, sync, stream, serve")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Println("")
	fmt.Println("  ugoku sync")
}

func main() {

	main_logger.Infof(fmt.Sprintf("ugoku-cli version %s", VERSION))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "upload":
		startUploaders(master_config)
	case "download":
		startDownloaders(master_config)
	case "sync":
		startSyncers(master_config)
	case "stream":
		startStreamers(master_config)
	case "serve":
		startServices(master_config)
	default:
		main_logger.Errorf("Missing or unknown command")
		printUsage()
		os.Exit(0)
	}

}
