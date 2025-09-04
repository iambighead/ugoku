package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Name     string
	Ip       string
	Port     int
	User     string
	Password string
	KeyFile  string
	CertFile string
}

type DownloaderConfig struct {
	Name         string
	Source       string
	SourcePath   string
	TargetPath   string
	Enabled      bool
	Worker       int
	MaxTimeout   int
	Throughput   int
	SourceServer ServerConfig
}

type UploaderConfig struct {
	Name         string
	Target       string
	SourcePath   string
	TargetPath   string
	Enabled      bool
	Worker       int
	MaxTimeout   int
	Throughput   int
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
	MaxTimeout    int
	Throughput    int
	SyncServer    ServerConfig
}

type StreamerConfig struct {
	Name          string
	Source        string
	SourcePath    string
	Target        string
	TargetPath    string
	Enabled       bool
	SleepInterval int
	Worker        int
	SourceServer  ServerConfig
	TargetServer  ServerConfig
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

type LoggingConfig struct {
	LogLevel             string `yaml:"LogLevel"             json:"LogLevel"` // 0: Error, 1: Info, 2: Debug
	LogOutputFolder      string `yaml:"LogOutputFolder"      json:"LogOutputFolder"`
	SyslogHost           string `yaml:"SyslogHost"           json:"SyslogHost"`
	SyslogProtocol       string `yaml:"SyslogProtocol"       json:"SyslogProtocol"`
	RotationIntervalHour int    `yaml:"RotationIntervalHour" json:"RotationIntervalHour"`
	MaxFileSizeMB        int    `yaml:"MaxFileSizeMB"        json:"MaxFileSizeMB"`
	MaxLogFiles          int    `yaml:"MaxLogFiles"          json:"MaxLogFiles"` // Maximum number of log files to keep
	SyslogPort           int    `yaml:"SyslogPort"           json:"SyslogPort"`
	RotationBySize       bool   `yaml:"RotationBySize"       json:"RotationBySize"`
	EnableConsoleLog     bool   `yaml:"EnableConsoleLog"     json:"EnableConsoleLog"`
	EnableSyslog         bool   `yaml:"EnableSyslog"         json:"EnableSyslog"`
}

type MasterConfig struct {
	Servers     []ServerConfig
	Downloaders []DownloaderConfig
	Uploaders   []UploaderConfig
	Syncers     []SyncerConfig
	Streamers   []StreamerConfig
	General     GeneralConfig
	Logging     LoggingConfig
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

	for idx, server := range config.Servers {
		if server.Port == 0 {
			config.Servers[idx].Port = 22
		}
	}

	for idx, downloader := range config.Downloaders {
		if config.Downloaders[idx].Worker < 1 {
			config.Downloaders[idx].Worker = 1
		}
		if config.Downloaders[idx].MaxTimeout <= 0 {
			config.Downloaders[idx].MaxTimeout = 600
		}
		if config.Downloaders[idx].Throughput <= 0 {
			config.Downloaders[idx].Throughput = 10
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
		if config.Uploaders[idx].MaxTimeout <= 0 {
			config.Uploaders[idx].MaxTimeout = 600
		}
		if config.Uploaders[idx].Throughput <= 0 {
			config.Uploaders[idx].Throughput = 10
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
		if config.Syncers[idx].MaxTimeout <= 0 {
			config.Syncers[idx].MaxTimeout = 600
		}
		if config.Syncers[idx].Throughput <= 0 {
			config.Syncers[idx].Throughput = 10
		}
		if config.Syncers[idx].SleepInterval < 1 {
			config.Syncers[idx].SleepInterval = 1
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

	for idx, streamer := range config.Streamers {
		if config.Streamers[idx].Worker < 1 {
			config.Streamers[idx].Worker = 1
		}
		if config.Streamers[idx].SleepInterval < 1 {
			config.Streamers[idx].SleepInterval = 1
		}

		for _, server := range config.Servers {
			if server.Name == streamer.Source {
				config.Streamers[idx].SourceServer = server
			}
			if server.Name == streamer.Target {
				config.Streamers[idx].TargetServer = server
			}
		}
	}

	err = validateConfig(config)
	if err != nil {
		return config, err
	}

	return config, nil
}
