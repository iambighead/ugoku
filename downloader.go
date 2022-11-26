package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// --------------------------------

var tempfolder string
var tempindex int

func init() {
	tempindex = 10000
}

func IncrTempIndex() {
	tempindex++
	if tempindex > 90000 {
		tempindex = 10000
	}
}

func downloadViaStaging(output_file string, source io.Reader) (int64, error) {
	temp_filename := fmt.Sprintf("%d%d", time.Now().UnixMilli(), tempindex)
	IncrTempIndex()
	tempfile_path := filepath.Join(tempfolder, temp_filename)
	tempfile, err := os.Create(tempfile_path)
	if err != nil {
		return 0, err
	}
	defer tempfile.Close()

	nBytes, err := io.Copy(tempfile, source)
	if err != nil {
		return 0, err
	}
	err = os.Rename(tempfile_path, output_file)
	if err != nil {
		return 0, err
	}
	return nBytes, nil
}

// --------------------------------
type FileDownloader interface {
	Start()
	Stop()
	init()
	scan() []string
	download()
}

type SftpDownloader struct {
	config.DownloaderConfig
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

// --------------------------------

func (dler *SftpDownloader) scan() []string {
	var filelist []string
	// walk a directory
	w := dler.sftp_client.Walk(dler.SourcePath)
	for w.Step() {
		if w.Err() != nil {
			continue
		}
		if !w.Stat().IsDir() {
			filelist = append(filelist, w.Path())
		}
		dler.logger.Debug(fmt.Sprintf("path=%s, isDir=%t", w.Path(), w.Stat().IsDir()))
	}

	return filelist
}

func (dler *SftpDownloader) download(file_to_download string) {

	output_file := filepath.Join(dler.TargetPath, file_to_download)
	dler.logger.Debug(fmt.Sprintf("Downloading file %s to %s", file_to_download, output_file))

	output_parent_folder := filepath.Dir(output_file)
	os.MkdirAll(output_parent_folder, fs.ModeDir|0764)
	dler.logger.Debug(fmt.Sprintf("created output folder %s", output_parent_folder))

	start_time := time.Now().UnixMilli()
	source, err := dler.sftp_client.OpenFile(file_to_download, os.O_RDONLY)
	if err != nil {
		dler.logger.Error(fmt.Sprintf("unable to open remote file: %s: %s", file_to_download, err.Error()))
		return
	}
	defer source.Close()

	// destination, err := os.Create(output_file)
	// if err != nil {
	// 	dler.logger.Error(fmt.Sprintf("unable to create local file for output: %s: %s", output_file, err.Error()))
	// 	return
	// }
	// defer destination.Close()

	// nBytes, err := io.Copy(destination, source)
	// if err != nil {
	// 	dler.logger.Error(fmt.Sprintf("error downloading file: %s: %s", file_to_download, err.Error()))
	// 	return
	// }
	nBytes, err := downloadViaStaging(output_file, source)
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

	err = dler.sftp_client.Remove(file_to_download)
	if err != nil {
		dler.logger.Error(fmt.Sprintf("failed to remove remote file: %s: %s", file_to_download, err.Error()))
	}
}

func (dler *SftpDownloader) Stop() {
	dler.started = false
	dler.sftp_client.Close()
	dler.ssh_client.Close()
}

func (dler *SftpDownloader) connectAndGetClients() error {

	dler.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", dler.SourceServer.Ip, dler.SourceServer.User))
	config := &ssh.ClientConfig{
		User: dler.SourceServer.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(dler.SourceServer.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ipport_str := fmt.Sprintf("%s:22", dler.SourceServer.Ip)
	ssh_client, err := ssh.Dial("tcp", ipport_str, config)
	if err != nil {
		return err
	}
	// open an SFTP session over an existing ssh connection.
	sftp_client, err := sftp.NewClient(ssh_client)
	if err != nil {
		return err
	}
	dler.logger.Info(fmt.Sprintf("connected to server %s with user %s", dler.SourceServer.Ip, dler.SourceServer.User))
	dler.ssh_client = ssh_client
	dler.sftp_client = sftp_client
	return nil
}

func (dler *SftpDownloader) init() {
	dler.started = false
	dler.logger = logger.NewLogger(fmt.Sprintf("downloader[%s]", dler.Name))

	for {
		err := dler.connectAndGetClients()
		if err == nil {
			break
		}
		dler.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

func (dler *SftpDownloader) Start() {
	dler.init()
	dler.started = true
	sleep_time := 1
	for {
		filelist := dler.scan()
		files_found := len(filelist)
		dler.logger.Debug(fmt.Sprintf("scan returned %d files", files_found))
		if files_found == 0 {
			if sleep_time < 16 {
				sleep_time = sleep_time * 2
			}
		} else {
			sleep_time = 1
			for _, file_to_download := range filelist {
				dler.download(file_to_download)
			}
		}
		dler.logger.Debug(fmt.Sprintf("sleep for %ds", sleep_time))
		time.Sleep(time.Duration(sleep_time) * time.Second)
	}
}

func startDownloaders(master_config config.MasterConfig) {
	tempfolder = master_config.General.TempFolder
	for _, downloader_config := range master_config.Downloaders {
		var dler SftpDownloader
		dler.DownloaderConfig = downloader_config
		go dler.Start()
	}
}
