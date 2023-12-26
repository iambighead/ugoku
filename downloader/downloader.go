package downloader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// --------------------------------

var tempfolder string
var global_stop_channel = make(chan int, 10)

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

type SftpDownloader struct {
	config.DownloaderConfig
	id          int
	prefix      string
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

// --------------------------------
func (dler *SftpDownloader) removeSrc(file_to_download string) {
	for i := 1; i <= 3; i++ {
		time.Sleep(time.Duration(i*100) * time.Millisecond)
		err := dler.sftp_client.Remove(file_to_download)
		if err != nil {
			dler.logger.Error(fmt.Sprintf("failed to remove remote file (try %d): %s: %s: %s", i, dler.Source, file_to_download, err.Error()))
		} else {
			// no error, check file really removed
			_, staterr := dler.sftp_client.Stat(file_to_download)
			if staterr != nil {
				break
			}
		}
	}
}

func (dler *SftpDownloader) download(file_to_download string, size int64) error {
	timeout_to_use := sftplibs.CalculateTimeout(int64(dler.Throughput), size, int64(dler.MaxTimeout))
	ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout_to_use))
	defer cancel()

	done := make(chan int, 1)
	cancelled := false
	go func() {
		relative_download_path := strings.Replace(file_to_download, dler.SourcePath, "", 1)
		output_file := filepath.Join(dler.TargetPath, relative_download_path)
		dler.logger.Debug(fmt.Sprintf("downloading file %s:%s to %s, with %d seconds timeout", dler.Source, file_to_download, output_file, timeout_to_use))

		output_parent_folder := filepath.Dir(output_file)
		os.MkdirAll(output_parent_folder, fs.ModeDir|0764)
		// dler.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

		start_time := time.Now().UnixMilli()
		source, err := dler.sftp_client.OpenFile(file_to_download, os.O_RDONLY)
		if err != nil {
			dler.logger.Error(fmt.Sprintf("unable to open remote file: %s: %s: %s", dler.Source, file_to_download, err.Error()))
			done <- 0
			return
		}
		defer source.Close()

		nBytes, tempfile_path, err := sftplibs.DownloadToTemp(ctxTimeout, tempfolder, source, dler.prefix)
		if err != nil && !cancelled {
			dler.logger.Error(fmt.Sprintf("error downloading file: %s: %s", file_to_download, err.Error()))
			done <- 0
			return
		}

		if cancelled {
			dler.logger.Info("download cancelled, remove temp file")
			os.Remove(tempfile_path)
			done <- 0
			return
		}

		err = sftplibs.RenameTempfile(tempfile_path, output_file)
		if err != nil {
			dler.logger.Error(fmt.Sprintf("error renaming file: %s to %s: %s", tempfile_path, output_file, err.Error()))
			done <- 0
			return
		}

		end_time := time.Now().UnixMilli()

		time_taken := end_time - start_time
		if time_taken < 1 {
			time_taken = 1
		}
		dler.logger.Info(fmt.Sprintf("downloaded %s with %d bytes in %d ms, %.1f mbps", file_to_download, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
		done <- 1
	}()

	select {
	case <-ctxTimeout.Done():
		cancelled = true
		return fmt.Errorf("download timeout: %v", ctxTimeout.Err())
	case result := <-done:
		if result > 0 {
			return nil
		}
		return errors.New("download failed")
	case <-global_stop_channel:
		return errors.New(fmt.Sprintf("download cancelled due to stop signal: %s", file_to_download))
	}
}

// --------------------------------

func (dler *SftpDownloader) connectAndGetClients() error {
	dler.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", dler.SourceServer.Ip, dler.SourceServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(
		dler.SourceServer.Ip,
		dler.SourceServer.Port,
		dler.SourceServer.User,
		dler.SourceServer.Password,
		dler.SourceServer.KeyFile,
		dler.SourceServer.CertFile)
	if err != nil {
		return err
	}
	dler.logger.Info(fmt.Sprintf("connected to server %s with user %s", dler.SourceServer.Ip, dler.SourceServer.User))
	dler.ssh_client = ssh_client
	dler.sftp_client = sftp_client
	return nil
}

// --------------------------------

func (dler *SftpDownloader) init() {
	dler.started = false
	dler.logger = logger.NewLogger(fmt.Sprintf("downloader[%s:%d]", dler.Name, dler.id))

	for {
		err := dler.connectAndGetClients()
		if err == nil {
			break
		}
		dler.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

// --------------------------------

func (dler *SftpDownloader) Stop() {
	dler.started = false
	global_stop_channel <- 1
	if dler.sftp_client != nil {
		dler.sftp_client.Close()
	}
	if dler.ssh_client != nil {
		dler.ssh_client.Close()
	}
	dler.logger.Info("stopped")
}

// --------------------------------

func (dler *SftpDownloader) Start(c chan FileObj, done chan int) {
	dler.init()
	dler.started = true
	dler.prefix = fmt.Sprintf("%s%d", dler.Name, dler.id)
	var file_to_download string
	for {
		fo := <-c
		file_to_download = fo.Path
		dler.logger.Debug(fmt.Sprintf("received file from channel: %s", file_to_download))
		download_err := dler.download(file_to_download, fo.Stat.Size())
		if download_err != nil {
			dler.logger.Error(fmt.Sprintf("download error: %s", download_err.Error()))
		} else {
			dler.removeSrc(file_to_download)
		}
		done <- 1
	}
}

func NewDownloader(downloader_config config.DownloaderConfig, tf string) {
	tempfolder = tf
	// make a channel
	c := make(chan FileObj, downloader_config.Worker*2)
	done := make(chan int, downloader_config.Worker*2)

	downloaders := make([]*SftpDownloader, downloader_config.Worker)

	for i := 0; i < downloader_config.Worker; i++ {
		var new_downloader SftpDownloader
		new_downloader.DownloaderConfig = downloader_config
		new_downloader.id = i
		go new_downloader.Start(c, done)
		downloaders[i] = &new_downloader
	}
	var new_scanner SftpScanner
	new_scanner.DownloaderConfig = downloader_config
	go new_scanner.Start(c, done, false)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Printf("downloader: signal received: %s\n", sig)

		new_scanner.Stop()
		for _, this_downloader := range downloaders {
			this_downloader.Stop()
		}

		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func NewOneTimeDownloader(downloader_config config.DownloaderConfig, tf string) {
	tempfolder = tf
	// make a channel
	c := make(chan FileObj, downloader_config.Worker*2)
	done := make(chan int, downloader_config.Worker*2)

	downloaders := make([]*SftpDownloader, downloader_config.Worker)

	for i := 0; i < downloader_config.Worker; i++ {
		var new_downloader SftpDownloader
		new_downloader.DownloaderConfig = downloader_config
		new_downloader.id = i
		go new_downloader.Start(c, done)
		downloaders[i] = &new_downloader
	}
	var new_scanner SftpScanner
	new_scanner.DownloaderConfig = downloader_config
	go new_scanner.Start(c, done, true)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Printf("downloader: signal received: %s\n", sig)

		new_scanner.Stop()
		for _, this_downloader := range downloaders {
			this_downloader.Stop()
		}

		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func NewSimpleDownloader(downloader_config config.DownloaderConfig) {
	// make a channel
	c := make(chan FileObj, downloader_config.Worker*2)
	done := make(chan int, downloader_config.Worker*2)

	var new_downloader SftpDownloader
	new_downloader.DownloaderConfig = downloader_config
	new_downloader.id = 0
	go new_downloader.Start(c, done)

	var new_scanner SftpScanner
	new_scanner.DownloaderConfig = downloader_config
	go new_scanner.ScanOnce(c, done)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Printf("downloader: signal received: %s\n", sig)

		new_scanner.Stop()
		new_downloader.Stop()

		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

// --------------------------------
