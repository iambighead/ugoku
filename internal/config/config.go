package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Name     string
	Ip       string
	User     string
	Password string
}

type DownloaderConfig struct {
	Name         string
	Source       string
	SourcePath   string
	TargetPath   string
	Enabled      bool
	Worker       int
	SourceServer ServerConfig
}

type UploaderConfig struct {
	Name         string
	Target       string
	SourcePath   string
	TargetPath   string
	Enabled      bool
	Worker       int
	TargetServer ServerConfig
}

type SyncerConfig struct {
	Name          string
	Server        string
	ServerPath    string
	LocalPath     string
	Mode          string
	Enabled       bool
	SleepInterval int
	Worker        int
	SyncServer    ServerConfig
}

// type DownloaderDedupConfig struct {
// 	Name         string
// 	Source       []string
// 	SourcePath   []string
// 	TargetPath   string
// 	Enabled      bool
// 	SourceServer []ServerConfig
// }

type GeneralConfig struct {
	TempFolder string
}
type MasterConfig struct {
	Servers     []ServerConfig
	Downloaders []DownloaderConfig
	Uploaders   []UploaderConfig
	Syncers     []SyncerConfig
	General     GeneralConfig
}

func validateConfig(cfg MasterConfig) error {
	return nil
}

func ReadConfig(path_to_config string) (MasterConfig, error) {

	config := MasterConfig{}
	yfile, err := os.ReadFile(path_to_config)

	if err != nil {
		return config, err
	}

	err2 := yaml.Unmarshal(yfile, &config)
	if err2 != nil {
		return config, err2
	}

	for idx, downloader := range config.Downloaders {
		if config.Downloaders[idx].Worker < 1 {
			config.Downloaders[idx].Worker = 1
		}
		for _, server := range config.Servers {
			if server.Name == downloader.Source {
				config.Downloaders[idx].SourceServer = server
			}
		}
	}

	for idx, uploader := range config.Uploaders {
		if config.Uploaders[idx].Worker < 1 {
			config.Uploaders[idx].Worker = 1
		}
		for _, server := range config.Servers {
			if server.Name == uploader.Target {
				config.Uploaders[idx].TargetServer = server
			}
		}
	}

	for idx, syncer := range config.Syncers {
		if config.Syncers[idx].Worker < 1 {
			config.Syncers[idx].Worker = 1
		}

		config.Syncers[idx].Mode = strings.ToLower(config.Syncers[idx].Mode)
		switch config.Syncers[idx].Mode {
		case "server":
		case "local":
		case "twoway":
		default:
			config.Syncers[idx].Mode = "server"
		}

		for _, server := range config.Servers {
			if server.Name == syncer.Server {
				config.Syncers[idx].SyncServer = server
			}
		}
	}

	err = validateConfig(config)
	if err != nil {
		return config, err
	}

	return config, nil
}
