package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/ugoku/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// --------------------------------

type Downloader interface {
	start()
	stop()
	scan()
	download()
}

type MyDownloader struct {
	config.Downloader
	started     bool
	logger      logger.Logger
	sftp_client *sftp.Client
	ssh_client  *ssh.Client
}

func (dler *MyDownloader) Download(filepath string, c chan int) {

	dler.logger.Info(fmt.Sprintf("Downloading file %s", filepath))

	start_time := time.Now().UnixMilli()
	source, err := dler.sftp_client.OpenFile(filepath, os.O_RDONLY)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	destination, err := os.Create(fmt.Sprintf("local-%s", filepath))
	if err != nil {
		log.Fatal(err)
	}
	defer destination.Close()

	nBytes, err := io.Copy(destination, source)
	if err != nil {
		log.Fatal(err)
	}
	end_time := time.Now().UnixMilli()

	time_taken := end_time - start_time
	dler.logger.Info(fmt.Sprintf("downloaded %d bytes in %ds, %.1f mbps", nBytes, time_taken, float64(nBytes/1000*8/time_taken)))

	c <- 1
}

func (dler *MyDownloader) Stop() {
	dler.started = false
	dler.sftp_client.Close()
	dler.ssh_client.Close()
}

func (dler *MyDownloader) Init() {
	dler.started = false
	dler.logger = logger.NewLogger(fmt.Sprintf("downloader[%s]", dler.Name))

	dler.logger.Info(fmt.Sprintf("connecting to server %s with user %s", dler.SourceServer.Ip, dler.SourceServer.User))
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
		log.Fatal("Failed to dial: ", err)
	}
	// defer ssh_client.Close()
	dler.ssh_client = ssh_client

	// open an SFTP session over an existing ssh connection.
	sftp_client, err := sftp.NewClient(ssh_client)
	if err != nil {
		log.Fatal(err)
	}
	// defer sftp_client.Close()
	dler.sftp_client = sftp_client
}

func (dler *MyDownloader) Start() {
	dler.Init()
	dler.started = true

	c := make(chan int)

	go dler.Download("amz-deno", c)

	<-c
}

func startDownloaders(master_config config.MasterConfig) {
	for _, downloader_config := range master_config.Downloaders {
		var dler MyDownloader
		dler.Downloader = downloader_config
		dler.Start()
	}
}
