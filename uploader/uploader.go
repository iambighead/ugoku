package uploader

import (
	"fmt"
	"io"
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

// var tempfolder string

func init() {
}

// --------------------------------
// type FileUploader interface {
// 	Start()
// 	Stop()
// 	init()
// 	scan() []string
// 	upload()
// }

type SftpUploader struct {
	config.UploaderConfig
	id          int
	prefix      string
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

// --------------------------------

func (uper *SftpUploader) removeSrc(file_to_upload string) {
	for i := 0; i < 3; i++ {
		err := os.Remove(file_to_upload)
		if err != nil {
			uper.logger.Error(fmt.Sprintf("failed to remove local file: %s: %s", file_to_upload, err.Error()))
		} else {
			// no error, check file really removed
			_, staterr := os.Stat(file_to_upload)
			if staterr != nil {
				break
			}
		}
	}
}

func (uper *SftpUploader) upload(file_to_upload string) {

	upload_source_relative_path := strings.Replace(file_to_upload, uper.SourcePath, "", 1)
	output_file := filepath.Join(uper.TargetPath, upload_source_relative_path)
	uper.logger.Debug(fmt.Sprintf("Uploading file %s to %s:%s", file_to_upload, uper.Target, output_file))

	output_parent_folder := strings.ReplaceAll(filepath.Dir(output_file), "\\", "/")
	output_file = strings.ReplaceAll(output_file, "\\", "/")
	err := uper.sftp_client.MkdirAll(output_parent_folder)
	if err != nil {
		uper.logger.Error(fmt.Sprintf("unable to create remote folder: %s: %s: %s", uper.Target, output_parent_folder, err.Error()))
		return
	}
	// uper.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

	start_time := time.Now().UnixMilli()
	source, err := os.OpenFile(file_to_upload, os.O_RDONLY, 0644)
	if err != nil {
		uper.logger.Error(fmt.Sprintf("unable to open local file: %s: %s", file_to_upload, err.Error()))
		return
	}
	defer source.Close()

	// nBytes, err := sftplibs.DownloadViaStaging(tempfolder, output_file, source, uper.prefix)
	// target, openerr := uper.sftp_client.OpenFile(output_file, os.O_CREATE|os.O_WRONLY)
	target, openerr := uper.sftp_client.Create(output_file)
	if openerr != nil {
		uper.logger.Error(fmt.Sprintf("error opening remote file: %s:%s: %s", uper.Target, output_file, err.Error()))
		return
	}
	defer target.Close()

	nBytes, err := io.Copy(target, source)
	if err != nil {
		uper.logger.Error(fmt.Sprintf("error uploading file: %s: %s", file_to_upload, err.Error()))
		return
	}
	end_time := time.Now().UnixMilli()

	time_taken := end_time - start_time
	if time_taken < 1 {
		time_taken = 1
	}
	uper.logger.Info(fmt.Sprintf("uploaded %s with %d bytes in %d ms, %.1f mbps", file_to_upload, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
}

// --------------------------------

func (uper *SftpUploader) connectAndGetClients() error {
	uper.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", uper.TargetServer.Ip, uper.TargetServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(uper.TargetServer.Ip, uper.TargetServer.User, uper.TargetServer.Password)
	if err != nil {
		return err
	}
	uper.logger.Info(fmt.Sprintf("connected to server %s with user %s", uper.TargetServer.Ip, uper.TargetServer.User))
	uper.ssh_client = ssh_client
	uper.sftp_client = sftp_client
	return nil
}

// --------------------------------

func (uper *SftpUploader) init() {
	uper.started = false
	uper.logger = logger.NewLogger(fmt.Sprintf("uploader[%s:%d]", uper.Name, uper.id))

	for {
		err := uper.connectAndGetClients()
		if err == nil {
			break
		}
		uper.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

// --------------------------------

func (uper *SftpUploader) Stop() {
	uper.started = false
	uper.sftp_client.Close()
	uper.ssh_client.Close()
}

// --------------------------------

func (uper *SftpUploader) Start(c chan FileObj, done chan int) {
	uper.init()
	uper.started = true
	uper.prefix = fmt.Sprintf("%s%d", uper.Name, uper.id)
	var file_to_upload string
	for {
		file_to_upload = (<-c).Path
		uper.logger.Debug(fmt.Sprintf("received file from channel: %s", file_to_upload))
		uper.upload(file_to_upload)
		uper.removeSrc(file_to_upload)
		done <- 1
	}
}

func NewUploader(uploaderer_config config.UploaderConfig, tf string) {
	// tempfolder = tf
	// make a channel
	c := make(chan FileObj, uploaderer_config.Worker*2)
	done := make(chan int, uploaderer_config.Worker*2)

	for i := 0; i < uploaderer_config.Worker; i++ {
		var new_uploader SftpUploader
		new_uploader.UploaderConfig = uploaderer_config
		new_uploader.id = i
		go new_uploader.Start(c, done)
	}
	var new_scanner FolderScanner
	new_scanner.UploaderConfig = uploaderer_config
	go new_scanner.Start(c, done)
}

// --------------------------------
