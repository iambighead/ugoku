package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const VERSION = "v0.0.1"

// --------------------------------

var main_logger logger.Logger

func init() {
	logger.Init("ugoku.log", "UGOKU_LOG_LEVEL")
	main_logger = logger.NewLogger("main")
}

func doDownload(sftp_client *sftp.Client, filename string, c chan int) {

	start_time := time.Now().UnixMilli()
	source, err := sftp_client.OpenFile(filename, os.O_RDONLY)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	destination, err := os.Create(fmt.Sprintf("local-%s", filename))
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
	main_logger.Info(fmt.Sprintf("downloaded %d bytes in %ds, %.1f mbps", nBytes, time_taken, float64(nBytes/1000*8/time_taken)))

	c <- 1
}

func main() {
	fmt.Printf("ugoku %s\n\n", VERSION)

	main_logger.Info("app started")

	// var hostKey ssh.PublicKey
	// An SSH client is represented with a ClientConn.
	//
	// To authenticate with the remote server you must pass at least one
	// implementation of AuthMethod via the Auth field in ClientConfig,
	// and provide a HostKeyCallback.

	config := &ssh.ClientConfig{
		User: "st",
		Auth: []ssh.AuthMethod{
			ssh.Password("P@55w0rd"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ssh_client, err := ssh.Dial("tcp", "192.168.55.162:22", config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}
	defer ssh_client.Close()

	// // Each ClientConn can support multiple interactive sessions,
	// // represented by a Session.
	// session, err := client.NewSession()
	// if err != nil {
	// 	log.Fatal("Failed to create session: ", err)
	// }
	// defer session.Close()

	// // Once a Session is created, you can execute a single command on
	// // the remote side using the Run method.
	// var b bytes.Buffer
	// session.Stdout = &b
	// if err := session.Run("/usr/bin/whoami"); err != nil {
	// 	log.Fatal("Failed to run: " + err.Error())
	// }
	// fmt.Println(b.String())

	// open an SFTP session over an existing ssh connection.
	sftp_client, err := sftp.NewClient(ssh_client)
	if err != nil {
		log.Fatal(err)
	}
	defer sftp_client.Close()

	c := make(chan int)

	go doDownload(sftp_client, "amz-deno", c)
	go doDownload(sftp_client, "amz-deno2", c)

	<-c
	<-c

	// walk a directory
	// w := sftp_client.Walk("/home/st")
	// for w.Step() {
	// 	if w.Err() != nil {
	// 		continue
	// 	}
	// 	log.Println(w.Path())
	// }
}
