package syncer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/downloader"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SftpServerSyncer struct {
	config.SyncerConfig
	id          int
	prefix      string
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

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
	local_modtime := local_stat.ModTime().Unix()
	remote_modtime := stat.ModTime().Unix()
	return local_modtime != remote_modtime
}

func (syncer *SftpServerSyncer) download(file_to_download string, output_file string) {

	syncer.logger.Debug(fmt.Sprintf("Downloading file %s to %s", file_to_download, output_file))

	output_parent_folder := filepath.Dir(output_file)
	os.MkdirAll(output_parent_folder, fs.ModeDir|0764)
	// syncer.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

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
