package syncer

import (
	"fmt"
	"os"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	siginthandler "github.com/iambighead/ugoku/internal/sigintHandler"
	"github.com/iambighead/ugoku/uploader"
)

// --------------------------------
var term_signal bool
var sync_manager_logger *logger.Logger

var tempfolder string

func init() {
	// sync_manager_logger = logger.NewLogger("sync-manager")
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

func startSyncServer(syncer_config config.SyncerConfig, mode string, loggerInstance *logger.Logger) {

	syncers := make([]*SftpServerSyncer, syncer_config.Worker)
	var new_scanner *downloader.SftpScanner

	siginthandler.Handle("server syncer", func() {
		term_signal = true
		if new_scanner != nil {
			new_scanner.Stop()
		}
		for _, this_syncer := range syncers {
			if this_syncer != nil {
				this_syncer.Stop()
			}
		}
	})

	// make a channel
	c := make(chan downloader.FileObj, syncer_config.Worker*2)
	done := make(chan int, syncer_config.Worker*2)

	for i := 0; i < syncer_config.Worker; i++ {
		go func(myid int) {
			for {
				var new_server_syncer SftpServerSyncer
				new_server_syncer.SyncerConfig = syncer_config
				new_server_syncer.id = myid
				syncers[myid] = &new_server_syncer
				new_server_syncer.Start(c, done, loggerInstance)
				new_server_syncer.Stop()
				syncers[myid] = nil
				if mode == "onetime" || term_signal {
					return
				}
				sync_manager_logger.Infof(fmt.Sprintf("server syncer [%d] exited, will recreate", myid))
			}
		}(i)
	}

	var proxyconfig config.DownloaderConfig
	proxyconfig.Name = syncer_config.Name
	proxyconfig.Source = syncer_config.Server
	proxyconfig.SourceServer = syncer_config.SyncServer
	proxyconfig.SourcePath = syncer_config.ServerPath

	if mode == "onetime" {
		new_scanner = new(downloader.SftpScanner)
		new_scanner.Default_sleep_time = 60
		if syncer_config.SleepInterval > 0 {
			new_scanner.Default_sleep_time = syncer_config.SleepInterval
		}
		new_scanner.DownloaderConfig = proxyconfig
		new_scanner.Start(c, done, true, loggerInstance)
		new_scanner.Stop()
		new_scanner = nil
		os.Exit(0)
	} else {
		go func() {
			for {
				new_scanner = new(downloader.SftpScanner)
				new_scanner.Default_sleep_time = 60
				if syncer_config.SleepInterval > 0 {
					new_scanner.Default_sleep_time = syncer_config.SleepInterval
				}
				new_scanner.DownloaderConfig = proxyconfig
				new_scanner.Start(c, done, false, loggerInstance)
				new_scanner.Stop()
				new_scanner = nil
				if term_signal {
					return
				}
				sync_manager_logger.Infof("server syncer scanner exited, will recreate")
			}
		}()
	}
}

func startSyncLocal(syncer_config config.SyncerConfig, mode string, loggerInstance *logger.Logger) {

	syncers := make([]*SftpLocalSyncer, syncer_config.Worker)
	var new_scanner *uploader.FolderScanner

	siginthandler.Handle("local syncer", func() {
		term_signal = true
		if new_scanner != nil {
			new_scanner.Stop()
		}
		for _, this_syncer := range syncers {
			if this_syncer != nil {
				this_syncer.Stop()
			}
		}
	})

	// make a channel
	c := make(chan uploader.FileObj, syncer_config.Worker*2)
	done := make(chan int, syncer_config.Worker*2)

	for i := 0; i < syncer_config.Worker; i++ {

		go func(myid int) {
			for {
				var new_server_syncer SftpLocalSyncer
				new_server_syncer.SyncerConfig = syncer_config
				new_server_syncer.id = myid
				syncers[myid] = &new_server_syncer
				new_server_syncer.Start(c, done, loggerInstance)
				new_server_syncer.Stop()
				syncers[myid] = nil
				if mode == "onetime" || term_signal {
					return
				}
				sync_manager_logger.Infof(fmt.Sprintf("local syncer [%d] exited, will recreate", myid))
			}
		}(i)

	}

	var proxyconfig config.UploaderConfig
	proxyconfig.Name = syncer_config.Name
	proxyconfig.Target = syncer_config.Server
	proxyconfig.TargetServer = syncer_config.SyncServer
	proxyconfig.TargetPath = syncer_config.ServerPath
	proxyconfig.SourcePath = syncer_config.LocalPath

	if mode == "onetime" {
		new_scanner = new(uploader.FolderScanner)
		new_scanner.Default_sleep_time = 60
		if syncer_config.SleepInterval > 0 {
			new_scanner.Default_sleep_time = syncer_config.SleepInterval
		}
		new_scanner.UploaderConfig = proxyconfig
		new_scanner.StartWithWatcher(c, done, true, loggerInstance)
		new_scanner.Stop()
		new_scanner = nil
		os.Exit(0)
	} else {
		go func() {
			for {
				new_scanner = new(uploader.FolderScanner)
				new_scanner.Default_sleep_time = 60
				if syncer_config.SleepInterval > 0 {
					new_scanner.Default_sleep_time = syncer_config.SleepInterval
				}
				new_scanner.UploaderConfig = proxyconfig
				new_scanner.StartWithWatcher(c, done, false, loggerInstance)
				new_scanner.Stop()
				new_scanner = nil
				if term_signal {
					return
				}
				sync_manager_logger.Infof("local syncer scanner exited, will recreate")
			}
		}()
	}
}

func NewSyncer(syncer_config config.SyncerConfig, tf string, loggerFactory func(string) *logger.Logger) {
	sync_manager_logger = loggerFactory("sync-manager")
	tempfolder = tf

	switch syncer_config.Mode {
	case "server":
		startSyncServer(syncer_config, "", sync_manager_logger)
	case "local":
		startSyncLocal(syncer_config, "", sync_manager_logger)
	case "both":
	default:

	}
}

func NewOneTimeSyncer(syncer_config config.SyncerConfig, tf string, loggerFactory func(string) *logger.Logger) {
	sync_manager_logger = loggerFactory("sync-manager")
	tempfolder = tf

	switch syncer_config.Mode {
	case "server":
		startSyncServer(syncer_config, "onetime", sync_manager_logger)
	case "local":
		startSyncLocal(syncer_config, "onetime", sync_manager_logger)
	case "both":
	default:

	}
}
