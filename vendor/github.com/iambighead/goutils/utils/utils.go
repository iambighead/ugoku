package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func StringArrayContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func ReadFilelist(root_folder string) ([]string, error) {
	var files []string
	err := filepath.Walk(root_folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func GetFileSha256(path_to_file string) ([]byte, error) {
	// ------------------
	file, err := os.Open(path_to_file)
	if err != nil {
		return nil, err
	}

	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)

	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), err
}

func GetFileSha256InHex(path_to_file string) ([]byte, error) {
	hash, err := GetFileSha256(path_to_file)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	hash_in_hex := hex.EncodeToString(hash)
	return []byte(hash_in_hex), nil
}
