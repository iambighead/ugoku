package downloader

import (
	"fmt"
	"io/fs"
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
	started            bool
	logger             logger.Logger
	sftp_client        *sftp.Client
	ssh_client         *ssh.Client
	Default_sleep_time int
}

type FileObj struct {
	Path string
	Stat fs.FileInfo
}

func (scanner *SftpScanner) scan(c chan FileObj, done chan int) {
	// walk a directory
	sleep_time := scanner.Default_sleep_time
	for {
		if !scanner.started {
			scanner.logger.Info("sftp scanner stopped, exiting scan")
			return
		}

		files_found := false
		var dispatched int
		w := scanner.sftp_client.Walk(scanner.SourcePath)
		for w.Step() {

			if !scanner.started {
				scanner.logger.Info("sftp scanner stopped, exiting scan")
				return
			}

			if w.Err() != nil {
				scanner.logger.Debug(w.Err().Error())
				continue
			}
			if !w.Stat().IsDir() {
				files_found = true
				// filelist = append(filelist, w.Path())
				var rf FileObj
				rf.Path = w.Path()
				rf.Stat = w.Stat()

				select {
				// Put new file in the channel unless it is full
				case c <- rf:
					dispatched++
					scanner.logger.Debug(fmt.Sprintf("sent file to channel: %s, dispatched %d, ch %d/%d", rf.Path, dispatched, len(c), cap(c)))

				default:
					scanner.logger.Debug(fmt.Sprintf("channel full (%d dispatched) wait for something done first", dispatched))
					<-done
					dispatched--
					scanner.logger.Debug(fmt.Sprintf("done received, %d dispatched now", dispatched))
					c <- rf
					dispatched++
					scanner.logger.Debug(fmt.Sprintf("sent file to channel: %s, dispatched %d, ch %d/%d", rf.Path, dispatched, len(c), cap(c)))
				}
			}
			// scanner.logger.Debug(fmt.Sprintf("path=%s, isDir=%t", w.Path(), w.Stat().IsDir()))
		}

		if dispatched > 0 {
			scanner.logger.Debug(fmt.Sprintf("end of scan, wait for %d more dispatched to be done", dispatched))
			for {
				<-done
				dispatched--
				scanner.logger.Debug(fmt.Sprintf("received done, dispatched = %d", dispatched))
				if dispatched < 1 {
					break
				}
			}
		}

		if !files_found {
			if sleep_time < 16 {
				sleep_time = sleep_time * 2
			}
		} else {
			sleep_time = scanner.Default_sleep_time
		}
		// scanner.logger.Debug(fmt.Sprintf("sleep for %d seconds", sleep_time))
		time.Sleep(time.Duration(sleep_time) * time.Second)
	}
}

func (scanner *SftpScanner) connectAndGetClients() error {
	scanner.logger.Debug(fmt.Sprintf("connecting to server %s with user %s", scanner.SourceServer.Ip, scanner.SourceServer.User))
	ssh_client, sftp_client, err := sftplibs.ConnectSftpServer(
		scanner.SourceServer.Ip,
		scanner.SourceServer.Port,
		scanner.SourceServer.User,
		scanner.SourceServer.Password,
		scanner.SourceServer.KeyFile)
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
	scanner.logger = logger.NewLogger(fmt.Sprintf("sftp-scanner[%s]", scanner.Name))
	if scanner.Default_sleep_time <= 0 {
		scanner.Default_sleep_time = 1
	}

	for {
		err := scanner.connectAndGetClients()
		if err == nil {
			break
		}
		scanner.logger.Error(fmt.Sprintf("error connecting to server, will try again: %s", err.Error()))
		time.Sleep(10 * time.Second)
	}
}

func (scanner *SftpScanner) Start(c chan FileObj, done chan int) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done)
}

func (scanner *SftpScanner) Stop() {
	scanner.logger.Info("stopping")
	scanner.started = false
	scanner.sftp_client.Close()
	scanner.ssh_client.Close()
}
