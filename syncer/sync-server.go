package syncer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/iambighead/ugoku/uploader"
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

type SftpServerSyncer struct {
	config.SyncerConfig
	id          int
	prefix      string
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

type SftpLocalSyncer struct {
	config.SyncerConfig
	id          int
	prefix      string
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

// --------------------------------
// func (syncer *SftpServerSyncer) removeSrc(file_to_download string) {
// 	for i := 0; i < 3; i++ {
// 		err := syncer.sftp_client.Remove(file_to_download)
// 		if err != nil {
// 			syncer.logger.Error(fmt.Sprintf("failed to remove remote file: %s: %s: %s", syncer.Source, file_to_download, err.Error()))
// 		} else {
// 			// no error, check file really removed
// 			_, staterr := syncer.sftp_client.Stat(file_to_download)
// 			if staterr != nil {
// 				break
// 			}
// 		}
// 	}
// }

func (syncer *SftpServerSyncer) downloadable(file_to_download string, output_file string, stat fs.FileInfo) bool {
	local_stat, err := os.Stat(output_file)
	if err != nil {
		return true
	}
	local_size := local_stat.Size()
	remote_size := stat.Size()
	if local_size != remote_size {
		return true
	}
	local_modtime := local_stat.ModTime()
	remote_modtime := stat.ModTime()
	return local_modtime != remote_modtime
}

func (syncer *SftpServerSyncer) download(file_to_download string, output_file string) {

	syncer.logger.Debug(fmt.Sprintf("Downloading file %s to %s", file_to_download, output_file))

	output_parent_folder := filepath.Dir(output_file)
	os.MkdirAll(output_parent_folder, fs.ModeDir|0764)
	syncer.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

	start_time := time.Now().UnixMilli()
	source, err := syncer.sftp_client.OpenFile(file_to_download, os.O_RDONLY)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("unable to open remote file: %s: %s: %s", syncer.Server, file_to_download, err.Error()))
		return
	}
	defer source.Close()

	nBytes, err := sftplibs.DownloadViaStaging(tempfolder, output_file, source, syncer.prefix)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("error downloading file: %s: %s", file_to_download, err.Error()))
		return
	}
	end_time := time.Now().UnixMilli()

	time_taken := end_time - start_time
	if time_taken < 1 {
		time_taken = 1
	}
	syncer.logger.Info(fmt.Sprintf("downloaded %s with %d bytes in %d ms, %.1f mbps", file_to_download, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
}

// --------------------------------

func (syncer *SftpServerSyncer) connectAndGetClients() error {
	syncer.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", syncer.SyncServer.Ip, syncer.SyncServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(syncer.SyncServer.Ip, syncer.SyncServer.User, syncer.SyncServer.Password)
	if err != nil {
		return err
	}
	syncer.logger.Info(fmt.Sprintf("connected to server %s with user %s", syncer.SyncServer.Ip, syncer.SyncServer.User))
	syncer.ssh_client = ssh_client
	syncer.sftp_client = sftp_client
	return nil
}

// --------------------------------

func (syncer *SftpServerSyncer) init() {
	syncer.started = false
	syncer.logger = logger.NewLogger(fmt.Sprintf("server-syncer[%s:%d]", syncer.Name, syncer.id))

	for {
		err := syncer.connectAndGetClients()
		if err == nil {
			break
		}
		syncer.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

// --------------------------------

func (syncer *SftpServerSyncer) Stop() {
	syncer.logger.Info("stopping")
	syncer.started = false
	syncer.sftp_client.Close()
	syncer.ssh_client.Close()
}

// --------------------------------

func (syncer *SftpServerSyncer) updateModTime(output_file string, stat fs.FileInfo) {
	modtime := stat.ModTime()
	err := os.Chtimes(output_file, modtime, modtime)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("failed to update modified time: %s: %s", output_file, err.Error()))
	}
}

// --------------------------------

func (syncer *SftpServerSyncer) Start(c chan downloader.FileObj, done chan int) {
	syncer.init()
	syncer.started = true
	syncer.prefix = fmt.Sprintf("%s%d", syncer.Name, syncer.id)

	for {
		fo := <-c
		syncer.logger.Debug(fmt.Sprintf("received file from channel: %s", fo.Path))
		relative_download_path := strings.Replace(fo.Path, syncer.ServerPath, "", 1)
		output_file := filepath.Join(syncer.LocalPath, relative_download_path)
		if syncer.downloadable(fo.Path, output_file, fo.Stat) {
			syncer.download(fo.Path, output_file)
			syncer.updateModTime(output_file, fo.Stat)
		}
		done <- 1
	}
}

// =============================================

func (syncer *SftpLocalSyncer) uploadable(file_to_download string, output_file string, stat fs.FileInfo) bool {
	remote_stat, err := syncer.sftp_client.Stat(output_file)
	if err != nil {
		return true
	}
	remote_size := remote_stat.Size()
	local_size := stat.Size()
	if local_size != remote_size {
		return true
	}
	remote_modtime := remote_stat.ModTime()
	local_modtime := stat.ModTime()
	return local_modtime != remote_modtime
}

func (syncer *SftpLocalSyncer) upload(file_to_upload string, output_file string) {
	syncer.logger.Debug(fmt.Sprintf("Uploading file %s to %s:%s", file_to_upload, syncer.Server, output_file))
	output_parent_folder := filepath.Dir(output_file)
	err := syncer.sftp_client.MkdirAll(output_parent_folder)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("unable to create remote folder: %s: %s: %s", syncer.Server, output_parent_folder, err.Error()))
		return
	}
	syncer.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

	start_time := time.Now().UnixMilli()
	source, err := os.OpenFile(file_to_upload, os.O_RDONLY, 0644)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("unable to open local file: %s: %s", file_to_upload, err.Error()))
		return
	}
	defer source.Close()

	// nBytes, err := sftplibs.DownloadViaStaging(tempfolder, output_file, source, syncer.prefix)
	target, openerr := syncer.sftp_client.OpenFile(output_file, os.O_CREATE|os.O_WRONLY)
	if openerr != nil {
		syncer.logger.Error(fmt.Sprintf("error opening remote file: %s:%s: %s", syncer.Server, output_file, err.Error()))
		return
	}
	defer target.Close()

	nBytes, err := io.Copy(target, source)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("error uploading file: %s: %s", file_to_upload, err.Error()))
		return
	}
	end_time := time.Now().UnixMilli()

	time_taken := end_time - start_time
	if time_taken < 1 {
		time_taken = 1
	}
	syncer.logger.Info(fmt.Sprintf("uploaded %s with %d bytes in %d ms, %.1f mbps", file_to_upload, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
}

// --------------------------------

func (syncer *SftpLocalSyncer) connectAndGetClients() error {
	syncer.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", syncer.SyncServer.Ip, syncer.SyncServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(syncer.SyncServer.Ip, syncer.SyncServer.User, syncer.SyncServer.Password)
	if err != nil {
		return err
	}
	syncer.logger.Info(fmt.Sprintf("connected to server %s with user %s", syncer.SyncServer.Ip, syncer.SyncServer.User))
	syncer.ssh_client = ssh_client
	syncer.sftp_client = sftp_client
	return nil
}

// --------------------------------

func (syncer *SftpLocalSyncer) init() {
	syncer.started = false
	syncer.logger = logger.NewLogger(fmt.Sprintf("local-syncer[%s:%d]", syncer.Name, syncer.id))

	for {
		err := syncer.connectAndGetClients()
		if err == nil {
			break
		}
		syncer.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

// --------------------------------

func (syncer *SftpLocalSyncer) Stop() {
	syncer.logger.Info("stopping")
	syncer.started = false
	syncer.sftp_client.Close()
	syncer.ssh_client.Close()
}

// --------------------------------

func (syncer *SftpLocalSyncer) updateModTime(output_file string, stat fs.FileInfo) {
	modtime := stat.ModTime()
	file_to_update := strings.ReplaceAll(output_file, "\\", "/")
	err := syncer.sftp_client.Chtimes(file_to_update, modtime, modtime)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("failed to update modified time: %s: %s", output_file, err.Error()))
	}
}

// --------------------------------

func (syncer *SftpLocalSyncer) Start(c chan uploader.FileObj, done chan int) {
	syncer.init()
	syncer.started = true
	syncer.prefix = fmt.Sprintf("%s%d", syncer.Name, syncer.id)
	defer syncer.Stop()
	for {
		fo := <-c
		syncer.logger.Debug(fmt.Sprintf("received file from channel: %s", fo.Path))
		upload_source_relative_path := strings.Replace(fo.Path, syncer.LocalPath, "", 1)
		output_file := filepath.Join(syncer.ServerPath, upload_source_relative_path)
		output_file = strings.ReplaceAll(output_file, "\\", "/")
		if syncer.uploadable(fo.Path, output_file, fo.Stat) {
			syncer.upload(fo.Path, output_file)
			syncer.updateModTime(output_file, fo.Stat)
		}
		done <- 1
	}
}

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

// --------------------------------
