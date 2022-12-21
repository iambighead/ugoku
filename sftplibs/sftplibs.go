package sftplibs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

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

func ConnectSftpServer(host_ip string, user string, password string) (*ssh.Client, *sftp.Client, error) {

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

func doSftpDownload(source io.Reader, tempfile_path string) (int64, error) {
	tempfile, err := os.Create(tempfile_path)
	if err != nil {
		return 0, err
	}
	defer tempfile.Close()

	nBytes, err := io.Copy(tempfile, source)
	if err != nil {
		return 0, err
	}
	return nBytes, nil
}

func DownloadToTemp(temp_folder string, source io.Reader, prefix string) (int64, string, error) {
	temp_filename := fmt.Sprintf("%s_%d%d", prefix, time.Now().UnixMilli(), tempindex)
	IncrTempIndex()
	tempfile_path := filepath.Join(temp_folder, temp_filename)

	nBytes, err := doSftpDownload(source, tempfile_path)
	if err != nil {
		return 0, "", err
	}
	return nBytes, tempfile_path, err
}

func RenameTempfile(tempfile_path string, output_file string) error {
	// retry rename up to 3 times, 1s interval
	var rename_err error
	for i := 0; i < 3; i++ {
		err := os.Rename(tempfile_path, output_file)
		if err == nil {
			rename_err = nil
			break
		}
		rename_err = err
		time.Sleep(1 * time.Second)
	}
	if rename_err != nil {
		os.Remove(tempfile_path)
		return rename_err
	}
	return nil
}

func CalculateTimeout(throughput int64, size int64, max_timeout int64) int64 {
	timeout := int64(size / (throughput * 125000))
	if timeout < max_timeout {
		return timeout
	}
	return max_timeout
}
