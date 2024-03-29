package streamer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	siginthandler "github.com/iambighead/ugoku/internal/sigintHandler"
	"github.com/iambighead/ugoku/internal/sleepytime"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// --------------------------------
var term_signal bool
var stream_manager_logger logger.Logger

// var tempfolder string

func init() {
	stream_manager_logger = logger.NewLogger("stream-manager")
}

// --------------------------------
// type FileStreamer interface {
// 	Start()
// 	Stop()
// 	init()
// 	scan() []string
// 	download()
// }

type SftpStreamer struct {
	config.StreamerConfig
	id                 int
	prefix             string
	started            bool
	logger             logger.Logger
	sftp_client_source *sftp.Client
	ssh_client_source  *ssh.Client
	sftp_client_target *sftp.Client
	ssh_client_target  *ssh.Client
	streamer_to_exit   bool
}

// --------------------------------
func (streamer *SftpStreamer) removeSrc(file_to_download string) {
	for i := 0; i < 3; i++ {
		err := streamer.sftp_client_source.Remove(file_to_download)
		if err != nil {
			streamer.logger.Error(fmt.Sprintf("failed to remove remote file: %s: %s: %s", streamer.Source, file_to_download, err.Error()))
		} else {
			// no error, check file really removed
			_, staterr := streamer.sftp_client_source.Stat(file_to_download)
			if staterr != nil {
				break
			}
		}
	}
}

func (streamer *SftpStreamer) stream(file_to_download string) bool {

	upload_source_relative_path := strings.Replace(file_to_download, streamer.SourcePath, "", 1)
	output_file := filepath.Join(streamer.TargetPath, upload_source_relative_path)
	output_file = strings.ReplaceAll(output_file, "\\", "/")

	streamer.logger.Debug(fmt.Sprintf("streaming file %s to %s:%s", file_to_download, streamer.Target, output_file))
	output_parent_folder := strings.ReplaceAll(filepath.Dir(output_file), "\\", "/")

	err := streamer.sftp_client_target.MkdirAll(output_parent_folder)
	if err != nil {
		streamer.logger.Error(fmt.Sprintf("unable to create remote folder: %s: %s: %s", streamer.Target, output_parent_folder, err.Error()))
		return false
	}

	start_time := time.Now().UnixMilli()
	source, err := streamer.sftp_client_source.OpenFile(file_to_download, os.O_RDONLY)
	if err != nil {
		streamer.logger.Error(fmt.Sprintf("unable to open source file: %s: %s: %s", streamer.Source, file_to_download, err.Error()))
		return false
	}
	defer source.Close()

	target, openerr := streamer.sftp_client_target.Create(output_file)
	if openerr != nil {
		streamer.logger.Error(fmt.Sprintf("error opening target file: %s:%s: %s", streamer.Target, output_file, err.Error()))
		return false
	}
	defer target.Close()

	nBytes, err := io.Copy(target, source)
	if err != nil {
		streamer.logger.Error(fmt.Sprintf("error streaming file: %s: %s", file_to_download, err.Error()))
		return false
	}
	end_time := time.Now().UnixMilli()

	time_taken := end_time - start_time
	if time_taken < 1 {
		time_taken = 1
	}
	streamer.logger.Info(fmt.Sprintf("streamed %s with %d bytes in %d ms, %.1f mbps", file_to_download, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
	return true
}

// --------------------------------

func (streamer *SftpStreamer) connectAndGetClients() error {
	streamer.logger.Debug(fmt.Sprintf("connecting to source server %s with user %s", streamer.SourceServer.Ip, streamer.SourceServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(
		streamer.SourceServer.Ip,
		streamer.SourceServer.Port,
		streamer.SourceServer.User,
		streamer.SourceServer.Password,
		streamer.SourceServer.KeyFile,
		streamer.SourceServer.CertFile)
	if err != nil {
		return err
	}
	streamer.logger.Info(fmt.Sprintf("connected to source server %s with user %s", streamer.SourceServer.Ip, streamer.SourceServer.User))
	streamer.ssh_client_source = ssh_client
	streamer.sftp_client_source = sftp_client

	streamer.logger.Debug(fmt.Sprintf("connecting to target server %s with user %s", streamer.TargetServer.Ip, streamer.TargetServer.User))
	ssh_client_target, sftp_client_target, err := sftplibs.ConnectSftpServer(
		streamer.TargetServer.Ip,
		streamer.TargetServer.Port,
		streamer.TargetServer.User,
		streamer.TargetServer.Password,
		streamer.TargetServer.KeyFile,
		streamer.TargetServer.CertFile)
	if err != nil {
		return err
	}
	streamer.logger.Info(fmt.Sprintf("connected to target server %s with user %s", streamer.TargetServer.Ip, streamer.TargetServer.User))
	streamer.ssh_client_target = ssh_client_target
	streamer.sftp_client_target = sftp_client_target
	return nil
}

// --------------------------------

func (streamer *SftpStreamer) init() {
	streamer.started = false
	streamer.streamer_to_exit = false
	streamer.logger = logger.NewLogger(fmt.Sprintf("streamer[%s:%d]", streamer.Name, streamer.id))

	var sleepy sleepytime.Sleepytime
	sleepy.Reset(2, 600)
	for {
		err := streamer.connectAndGetClients()
		if err == nil {
			break
		}
		streamer.Stop()
		streamer.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(time.Duration(sleepy.GetNextSleep()) * time.Second)
	}
}

// --------------------------------

func (streamer *SftpStreamer) Stop() {
	streamer.started = false
	streamer.streamer_to_exit = true
	if streamer.sftp_client_source != nil {
		streamer.sftp_client_source.Close()
	}
	if streamer.ssh_client_source != nil {
		streamer.ssh_client_source.Close()
	}
	if streamer.sftp_client_target != nil {
		streamer.sftp_client_target.Close()
	}
	if streamer.ssh_client_target != nil {
		streamer.ssh_client_target.Close()
	}
	streamer.logger.Info("stopped")
}

// --------------------------------

func (streamer *SftpStreamer) Start(c chan downloader.FileObj, done chan int) {
	streamer.init()
	streamer.started = true
	streamer.prefix = fmt.Sprintf("%s%d", streamer.Name, streamer.id)
	var file_to_download string
	go func() {
		for {
			if !streamer.started {
				return
			}
			file_to_download = (<-c).Path
			streamer.logger.Debug(fmt.Sprintf("received file from channel: %s", file_to_download))
			if streamer.stream(file_to_download) {
				streamer.removeSrc(file_to_download)
			} else {
				streamer.streamer_to_exit = true
			}
			done <- 1
			if streamer.streamer_to_exit {
				return
			}
		}
	}()

	for {
		time.Sleep(1 * time.Second)
		if streamer.streamer_to_exit {
			return
		}
	}
}

func setupSigHandler(new_scanner **downloader.SftpScanner, streamers []*SftpStreamer) {
	siginthandler.Handle("streamer", func() {
		term_signal = true
		if new_scanner != nil {
			(*new_scanner).Stop()
		}
		for _, this_streamer := range streamers {
			if this_streamer != nil {
				this_streamer.Stop()
			}
		}
	})
}

func NewStreamer(streamer_config config.StreamerConfig) {
	// tempfolder = tf
	streamers := make([]*SftpStreamer, streamer_config.Worker)
	var new_scanner *downloader.SftpScanner

	setupSigHandler(&new_scanner, streamers)

	// make a channel
	c := make(chan downloader.FileObj, streamer_config.Worker*2)
	done := make(chan int, streamer_config.Worker*2)

	for i := 0; i < streamer_config.Worker; i++ {
		go func(myid int) {
			for {
				var new_streamer SftpStreamer
				new_streamer.StreamerConfig = streamer_config
				new_streamer.id = myid
				streamers[myid] = &new_streamer
				new_streamer.Start(c, done)
				stream_manager_logger.Debug("return from start and calling streamer stop")
				fmt.Printf("NewStreamer calling stop\n")
				new_streamer.Stop()
				streamers[myid] = nil
				if term_signal {
					return
				}
				stream_manager_logger.Info(fmt.Sprintf("streamer [%d] exited, will recreate", myid))
			}
		}(i)
	}

	var proxyconfig config.DownloaderConfig
	proxyconfig.Name = streamer_config.Name
	proxyconfig.Source = streamer_config.Source
	proxyconfig.SourceServer = streamer_config.SourceServer
	proxyconfig.SourcePath = streamer_config.SourcePath

	go func() {
		for {
			new_scanner = new(downloader.SftpScanner)
			new_scanner.DownloaderConfig = proxyconfig
			new_scanner.Default_sleep_time = 60
			if streamer_config.SleepInterval > 0 {
				new_scanner.Default_sleep_time = streamer_config.SleepInterval
			}
			new_scanner.Start(c, done, false)
			new_scanner.Stop()
			new_scanner = nil
			if term_signal {
				return
			}
			stream_manager_logger.Info("scanner exited, will recreate")
		}
	}()

}

func NewOneTimeStreamer(streamer_config config.StreamerConfig) {
	// tempfolder = tf
	streamers := make([]*SftpStreamer, streamer_config.Worker)
	var new_scanner *downloader.SftpScanner

	setupSigHandler(&new_scanner, streamers)

	// make a channel
	c := make(chan downloader.FileObj, streamer_config.Worker*2)
	done := make(chan int, streamer_config.Worker*2)

	for i := 0; i < streamer_config.Worker; i++ {
		go func(myid int) {
			var new_streamer SftpStreamer
			new_streamer.StreamerConfig = streamer_config
			new_streamer.id = myid
			streamers[myid] = &new_streamer
			new_streamer.Start(c, done)
			new_streamer.Stop()
			streamers[myid] = nil
		}(i)
	}

	var proxyconfig config.DownloaderConfig
	proxyconfig.Name = streamer_config.Name
	proxyconfig.Source = streamer_config.Source
	proxyconfig.SourceServer = streamer_config.SourceServer
	proxyconfig.SourcePath = streamer_config.SourcePath

	new_scanner = new(downloader.SftpScanner)
	new_scanner.DownloaderConfig = proxyconfig
	new_scanner.Default_sleep_time = 60
	if streamer_config.SleepInterval > 0 {
		new_scanner.Default_sleep_time = streamer_config.SleepInterval
	}
	new_scanner.Start(c, done, true)
	new_scanner.Stop()
	new_scanner = nil
	os.Exit(0)
}

// --------------------------------
