package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func init() {
	tempindex = 10000
}

func IncrTempIndex() {
	tempindex++
	if tempindex > 90000 {
		tempindex = 10000
	}
}

func connectSftpServer(host_ip string, user string, password string) (*ssh.Client, *sftp.Client, error) {

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ipport_str := fmt.Sprintf("%s:22", host_ip)
	ssh_client, err := ssh.Dial("tcp", ipport_str, config)
	if err != nil {
		return nil, nil, err
	}
	// open an SFTP session over an existing ssh connection.
	sftp_client, err := sftp.NewClient(ssh_client)
	if err != nil {
		return nil, nil, err
	}

	return ssh_client, sftp_client, nil
}

func downloadViaStaging(temp_folder string, output_file string, source io.Reader, prefix string) (int64, error) {
	temp_filename := fmt.Sprintf("%s_%d%d", prefix, time.Now().UnixMilli(), tempindex)
	IncrTempIndex()
	tempfile_path := filepath.Join(temp_folder, temp_filename)
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
