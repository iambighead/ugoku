package syncer

import (
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/uploader"
)

// --------------------------------

var tempfolder string

func init() {
}

// --------------------------------
// type FileDownloader interface {
// 	Start()
// 	Stop()
// 	init()
// 	scan() []string
// 	download()
// }

// -------------------------

func startSyncServer(syncer_config config.SyncerConfig) {

	// make a channel
	c := make(chan downloader.FileObj, syncer_config.Worker*2)
	done := make(chan int, syncer_config.Worker*2)

	for i := 0; i < syncer_config.Worker; i++ {
		var new_server_syncer SftpServerSyncer
		new_server_syncer.SyncerConfig = syncer_config
		new_server_syncer.id = i
		go new_server_syncer.Start(c, done)
	}

	var proxyconfig config.DownloaderConfig
	proxyconfig.Name = syncer_config.Name
	proxyconfig.Source = syncer_config.Server
	proxyconfig.SourceServer = syncer_config.SyncServer
	proxyconfig.SourcePath = syncer_config.ServerPath

	var new_scanner downloader.SftpScanner

	new_scanner.Default_sleep_time = 60
	if syncer_config.SleepInterval > 0 {
		new_scanner.Default_sleep_time = syncer_config.SleepInterval
	}

	new_scanner.DownloaderConfig = proxyconfig
	go new_scanner.Start(c, done)
}

func startSyncLocal(syncer_config config.SyncerConfig) {

	// make a channel
	c := make(chan uploader.FileObj, syncer_config.Worker*2)
	done := make(chan int, syncer_config.Worker*2)

	for i := 0; i < syncer_config.Worker; i++ {
		var new_server_syncer SftpLocalSyncer
		new_server_syncer.SyncerConfig = syncer_config
		new_server_syncer.id = i
		go new_server_syncer.Start(c, done)
	}

	var proxyconfig config.UploaderConfig
	proxyconfig.Name = syncer_config.Name
	proxyconfig.Target = syncer_config.Server
	proxyconfig.TargetServer = syncer_config.SyncServer
	proxyconfig.TargetPath = syncer_config.ServerPath
	proxyconfig.SourcePath = syncer_config.LocalPath

	var new_scanner uploader.FolderScanner
	new_scanner.Default_sleep_time = 60
	if syncer_config.SleepInterval > 0 {
		new_scanner.Default_sleep_time = syncer_config.SleepInterval
	}
	new_scanner.UploaderConfig = proxyconfig
	go new_scanner.Start(c, done)
}

func NewSyncer(syncer_config config.SyncerConfig, tf string) {
	tempfolder = tf

	switch syncer_config.Mode {
	case "server":
		startSyncServer(syncer_config)
	case "local":
		startSyncLocal(syncer_config)
	case "both":
	default:

	}
}
