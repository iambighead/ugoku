package main

import (
	"fmt"
	"log"

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
	client, err := sftp.NewClient(ssh_client)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// walk a directory
	w := client.Walk("/home/st")
	for w.Step() {
		if w.Err() != nil {
			continue
		}
		log.Println(w.Path())
	}

	// leave your mark
	f, err := client.Create("hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write([]byte("Hello world!")); err != nil {
		log.Fatal(err)
	}
	f.Close()

	// check it's there
	fi, err := client.Lstat("hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(fi)
}
