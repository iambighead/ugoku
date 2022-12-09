package downloader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
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
	for i := 0; i < 3; i++ {
		err := dler.sftp_client.Remove(file_to_download)
		if err != nil {
			dler.logger.Error(fmt.Sprintf("failed to remove remote file: %s: %s: %s", dler.Source, file_to_download, err.Error()))
		} else {
			// no error, check file really removed
			_, staterr := dler.sftp_client.Stat(file_to_download)
			if staterr != nil {
				break
			}
		}
	}
}

func (dler *SftpDownloader) download(file_to_download string) {

	relative_download_path := strings.Replace(file_to_download, dler.SourcePath, "", 1)
	output_file := filepath.Join(dler.TargetPath, relative_download_path)
	dler.logger.Debug(fmt.Sprintf("Downloading file %s to %s", file_to_download, output_file))

	output_parent_folder := filepath.Dir(output_file)
	os.MkdirAll(output_parent_folder, fs.ModeDir|0764)
	dler.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

	start_time := time.Now().UnixMilli()
	source, err := dler.sftp_client.OpenFile(file_to_download, os.O_RDONLY)
	if err != nil {
		dler.logger.Error(fmt.Sprintf("unable to open remote file: %s: %s: %s", dler.Source, file_to_download, err.Error()))
		return
	}
	defer source.Close()

	nBytes, err := sftplibs.DownloadViaStaging(tempfolder, output_file, source, dler.prefix)
	if err != nil {
		dler.logger.Error(fmt.Sprintf("error downloading file: %s: %s", file_to_download, err.Error()))
		return
	}
	end_time := time.Now().UnixMilli()

	time_taken := end_time - start_time
	if time_taken < 1 {
		time_taken = 1
	}
	dler.logger.Info(fmt.Sprintf("downloaded %s with %d bytes in %d ms, %.1f mbps", file_to_download, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
}

// --------------------------------

func (dler *SftpDownloader) connectAndGetClients() error {
	dler.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", dler.SourceServer.Ip, dler.SourceServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(dler.SourceServer.Ip, dler.SourceServer.User, dler.SourceServer.Password)
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
	dler.sftp_client.Close()
	dler.ssh_client.Close()
}

// --------------------------------

func (dler *SftpDownloader) Start(c chan string, done chan int) {
	dler.init()
	dler.started = true
	dler.prefix = fmt.Sprintf("%s%d", dler.Name, dler.id)
	var file_to_download string
	for {
		file_to_download = <-c
		dler.logger.Debug(fmt.Sprintf("received file from channel: %s", file_to_download))
		dler.download(file_to_download)
		dler.removeSrc(file_to_download)
		done <- 1
	}
}

func NewDownloader(downloader_config config.DownloaderConfig, tf string) {
	tempfolder = tf
	// make a channel
	c := make(chan string, downloader_config.Worker*2)
	done := make(chan int, downloader_config.Worker*2)

	for i := 0; i < downloader_config.Worker; i++ {
		var new_downloader SftpDownloader
		new_downloader.DownloaderConfig = downloader_config
		new_downloader.id = i
		go new_downloader.Start(c, done)
	}
	var new_scanner SftpScanner
	new_scanner.DownloaderConfig = downloader_config
	go new_scanner.Start(c, done)
}

// --------------------------------