package uploader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/internal/sleepytime"
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

var global_stop_channel = make(chan int, 1)

// --------------------------------

func (uper *SftpUploader) removeSrc(file_to_upload string) {
	for i := 1; i <= 3; i++ {
		time.Sleep(time.Duration(i*100) * time.Millisecond)
		err := os.Remove(file_to_upload)
		if err != nil {
			uper.logger.Error(fmt.Sprintf("failed to remove local file (try %d): %s: %s", i, file_to_upload, err.Error()))
		} else {
			// no error, check file really removed
			_, staterr := os.Stat(file_to_upload)
			if staterr != nil {
				break
			}
		}
	}
}

func (uper *SftpUploader) upload(file_to_upload string, size int64) error {
	timeout_to_use := sftplibs.CalculateTimeout(int64(uper.Throughput), size, int64(uper.MaxTimeout))
	ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout_to_use))
	defer cancel()

	done := make(chan int, 1)
	cancelled := false
	go func() {

		upload_source_relative_path := strings.Replace(file_to_upload, uper.SourcePath, "", 1)
		output_file := filepath.Join(uper.TargetPath, upload_source_relative_path)
		output_file = strings.ReplaceAll(output_file, "\\", "/")
		uper.logger.Debug(fmt.Sprintf("uploading file %s to %s:%s, with %d seconds timeout", file_to_upload, uper.Target, output_file, timeout_to_use))

		output_parent_folder := strings.ReplaceAll(filepath.Dir(output_file), "\\", "/")
		err := uper.sftp_client.MkdirAll(output_parent_folder)
		if err != nil {
			uper.logger.Error(fmt.Sprintf("unable to create remote folder: %s: %s: %s", uper.Target, output_parent_folder, err.Error()))
			done <- 0
			return
		}
		// uper.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

		start_time := time.Now().UnixMilli()
		source, err := os.OpenFile(file_to_upload, os.O_RDONLY, 0644)
		if err != nil {
			uper.logger.Error(fmt.Sprintf("unable to open local file: %s: %s", file_to_upload, err.Error()))
			done <- 0
			return
		}
		defer source.Close()

		target, openerr := uper.sftp_client.Create(output_file)
		if openerr != nil {
			uper.logger.Error(fmt.Sprintf("error opening remote file: %s:%s: %s", uper.Target, output_file, err.Error()))
			done <- 0
			return
		}
		defer target.Close()

		// nBytes, err := io.Copy(target, source)
		nBytes, err := sftplibs.CopyWithCancel(ctxTimeout, target, source)
		if err != nil && !cancelled {
			uper.logger.Error(fmt.Sprintf("error uploading file: %s: %s", file_to_upload, err.Error()))
			done <- 0
			return
		}

		if cancelled {
			uper.logger.Info("upload cancelled")
			done <- 0
			return
		}

		end_time := time.Now().UnixMilli()

		time_taken := end_time - start_time
		if time_taken < 1 {
			time_taken = 1
		}
		uper.logger.Info(fmt.Sprintf("uploaded %s with %d bytes in %d ms, %.1f mbps", file_to_upload, nBytes, time_taken, float64(nBytes/1000*8/time_taken)))
		done <- 1
	}()

	select {
	case <-ctxTimeout.Done():
		return fmt.Errorf("upload timeout: %v", ctxTimeout.Err())
	case result := <-done:
		if result > 0 {
			return nil
		}
		return errors.New("upload failed")
	case <-global_stop_channel:
		return fmt.Errorf("upload cancelled due to stop signal: %s", file_to_upload)
	}

}

// --------------------------------

func (uper *SftpUploader) connectAndGetClients() error {
	uper.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", uper.TargetServer.Ip, uper.TargetServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(
		uper.TargetServer.Ip,
		uper.TargetServer.Port,
		uper.TargetServer.User,
		uper.TargetServer.Password,
		uper.TargetServer.KeyFile,
		uper.TargetServer.CertFile)
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

	var sleepy sleepytime.Sleepytime
	sleepy.Reset(2, 600)
	for {
		err := uper.connectAndGetClients()
		if err == nil {
			break
		}
		uper.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
		time.Sleep(time.Duration(sleepy.GetNextSleep()) * time.Second)
	}
}

// --------------------------------

func (uper *SftpUploader) Stop() {
	uper.started = false
	global_stop_channel <- 1
	if uper.sftp_client != nil {
		uper.sftp_client.Close()
	}
	if uper.ssh_client != nil {
		uper.ssh_client.Close()
	}
	uper.logger.Info("stopped")
}

// --------------------------------

func (uper *SftpUploader) Start(c chan FileObj, done chan int) {
	uper.init()
	uper.started = true
	uper.prefix = fmt.Sprintf("%s%d", uper.Name, uper.id)
	var file_to_upload string
	for {
		if !uper.started {
			return
		}
		fo := <-c
		file_to_upload = fo.Path
		uper.logger.Debug(fmt.Sprintf("received file from channel: %s", file_to_upload))
		upload_err := uper.upload(file_to_upload, fo.Stat.Size())
		if upload_err != nil {
			uper.logger.Error(fmt.Sprintf("upload error: %s", upload_err.Error()))
		} else {
			uper.removeSrc(file_to_upload)
		}
		done <- 1
	}
}

func NewUploader(uploaderer_config config.UploaderConfig, tf string) {
	// tempfolder = tf
	// make a channel
	c := make(chan FileObj, uploaderer_config.Worker*2)
	done := make(chan int, uploaderer_config.Worker*2)

	uploaders := make([]*SftpUploader, uploaderer_config.Worker)

	for i := 0; i < uploaderer_config.Worker; i++ {
		var new_uploader SftpUploader
		new_uploader.UploaderConfig = uploaderer_config
		new_uploader.id = i
		go new_uploader.Start(c, done)
		uploaders[i] = &new_uploader
	}
	var new_scanner FolderScanner
	new_scanner.UploaderConfig = uploaderer_config
	go new_scanner.Start(c, done, false)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)

	go func() {
		sig := <-sigs
		fmt.Printf("uploader: signal received: %s\n", sig)
		new_scanner.Stop()
		for _, this_uploader := range uploaders {
			this_uploader.Stop()
		}

		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func NewOneTimeUploader(uploaderer_config config.UploaderConfig, tf string) {
	// tempfolder = tf
	// make a channel
	c := make(chan FileObj, uploaderer_config.Worker*2)
	done := make(chan int, uploaderer_config.Worker*2)

	uploaders := make([]*SftpUploader, uploaderer_config.Worker)

	for i := 0; i < uploaderer_config.Worker; i++ {
		var new_uploader SftpUploader
		new_uploader.UploaderConfig = uploaderer_config
		new_uploader.id = i
		go new_uploader.Start(c, done)
		uploaders[i] = &new_uploader
	}
	var new_scanner FolderScanner
	new_scanner.UploaderConfig = uploaderer_config
	go new_scanner.Start(c, done, true)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)

	go func() {
		sig := <-sigs
		fmt.Printf("uploader: signal received: %s\n", sig)
		new_scanner.Stop()
		for _, this_uploader := range uploaders {
			this_uploader.Stop()
		}

		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

// --------------------------------
