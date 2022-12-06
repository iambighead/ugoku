package downloader

import (
	"fmt"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/iambighead/ugoku/sftplibs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// --------------------------------

func init() {
}

// --------------------------------

// type FileScanner interface {
// 	Start()
// 	Stop()
// 	init()
// 	scan() []string
// }

type SftpScanner struct {
	config.DownloaderConfig
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

func (scanner *SftpScanner) scan(c chan string, done chan int) {
	// walk a directory
	sleep_time := 1
	for {
		files_found := false
		if scanner.started {
			w := scanner.sftp_client.Walk(scanner.SourcePath)
			for w.Step() {
				if scanner.started {
					if w.Err() != nil {
						scanner.logger.Debug(w.Err().Error())
						continue
					}
					if !w.Stat().IsDir() {
						files_found = true
						// filelist = append(filelist, w.Path())
						newfile := w.Path()
						scanner.logger.Debug(fmt.Sprintf("send file to channel: %s", newfile))
						c <- newfile
						// wait for done signal
						<-done
					}
					// scanner.logger.Debug(fmt.Sprintf("path=%s, isDir=%t", w.Path(), w.Stat().IsDir()))
				}
			}
		}

		if !files_found {
			if sleep_time < 16 {
				sleep_time = sleep_time * 2
			}
		} else {
			sleep_time = 1
		}
		// scanner.logger.Debug(fmt.Sprintf("sleep for %d seconds", sleep_time))
		time.Sleep(time.Duration(sleep_time) * time.Second)
	}
}

func (scanner *SftpScanner) connectAndGetClients() error {
	scanner.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", scanner.SourceServer.Ip, scanner.SourceServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(scanner.SourceServer.Ip, scanner.SourceServer.User, scanner.SourceServer.Password)
	if err != nil {
		return err
	}
	scanner.logger.Info(fmt.Sprintf("connected to server %s with user %s", scanner.SourceServer.Ip, scanner.SourceServer.User))
	scanner.ssh_client = ssh_client
	scanner.sftp_client = sftp_client
	return nil
}

func (scanner *SftpScanner) init() {
	scanner.started = false
	scanner.logger = logger.NewLogger(fmt.Sprintf("scanner[%s]", scanner.Name))

	for {
		err := scanner.connectAndGetClients()
		if err == nil {
			break
		}
		scanner.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

func (scanner *SftpScanner) Start(c chan string, done chan int) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done)
}

func (scanner *SftpScanner) Stop(c chan string) {
	scanner.started = false
}
