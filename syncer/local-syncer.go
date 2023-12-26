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
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/iambighead/ugoku/uploader"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SftpLocalSyncer struct {
	config.SyncerConfig
	id          int
	prefix      string
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

func (syncer *SftpLocalSyncer) uploadable(file_to_download string, output_file string, stat fs.FileInfo) bool {
	remote_stat, err := syncer.sftp_client.Stat(output_file)
	if err != nil {
		return true
	}
	// syncer.logger.Debug(fmt.Sprintf("uploadable: found %s", output_file))
	remote_size := remote_stat.Size()
	local_size := stat.Size()
	if local_size != remote_size {
		// syncer.logger.Debug(fmt.Sprintf("uploadable: %s size different %d %d", output_file, remote_size, local_size))
		return true
	}
	// syncer.logger.Debug(fmt.Sprintf("uploadable: %s size same %d %d", output_file, remote_size, local_size))
	remote_modtime := remote_stat.ModTime().Unix()
	local_modtime := stat.ModTime().Unix()
	// syncer.logger.Debug(fmt.Sprintf("uploadable: %s time %d %d", output_file, remote_modtime, local_modtime))
	return local_modtime != remote_modtime
}

func (syncer *SftpLocalSyncer) upload(file_to_upload string, output_file string) {
	syncer.logger.Debug(fmt.Sprintf("uploading file %s to %s:%s", file_to_upload, syncer.Server, output_file))
	output_parent_folder := strings.ReplaceAll(filepath.Dir(output_file), "\\", "/")
	err := syncer.sftp_client.MkdirAll(output_parent_folder)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("unable to create remote folder: %s: %s: %s", syncer.Server, output_parent_folder, err.Error()))
		return
	}
	// syncer.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

	start_time := time.Now().UnixMilli()
	source, err := os.OpenFile(file_to_upload, os.O_RDONLY, 0644)
	if err != nil {
		syncer.logger.Error(fmt.Sprintf("unable to open local file: %s: %s", file_to_upload, err.Error()))
		return
	}
	defer source.Close()

	target, openerr := syncer.sftp_client.Create(output_file)
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
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(
		syncer.SyncServer.Ip,
		syncer.SyncServer.Port,
		syncer.SyncServer.User,
		syncer.SyncServer.Password,
		syncer.SyncServer.KeyFile,
		syncer.SyncServer.CertFile)
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
	if syncer.sftp_client != nil {
		syncer.sftp_client.Close()
	}
	if syncer.ssh_client != nil {
		syncer.ssh_client.Close()
	}
	syncer.logger.Info("stopped")
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
