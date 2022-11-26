package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/iambighead/goutils/utils"
	"gopkg.in/ini.v1"
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
	SourceServer ServerConfig
}

type GeneralConfig struct {
	TempFolder string
}
type MasterConfig struct {
	Servers     []ServerConfig
	Downloaders []DownloaderConfig
	General     GeneralConfig
}

func readConfigString() *ini.File {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Printf("Fail to read config.ini: %v", err)
		os.Exit(1)
	}
	return cfg
}

func parseSectionGeneral(cfg *ini.File, sectionList []string, config *MasterConfig) {
	if utils.StringArrayContains(sectionList, "general") {
		config.General.TempFolder = cfg.Section("general").Key("tempfolder").String()
	}
}

func parseSectionServer(cfg *ini.File, sectionList []string, config *MasterConfig) {
	for _, section_name := range sectionList {
		if strings.Index(section_name, "server.") == 0 {
			var new_server ServerConfig
			new_server.Name = section_name[7:]
			new_server.Ip = cfg.Section(section_name).Key("ip").String()
			new_server.User = cfg.Section(section_name).Key("user").String()
			new_server.Password = cfg.Section(section_name).Key("password").String()
			config.Servers = append(config.Servers, new_server)
		}
	}
}

func parseSectionDownloader(cfg *ini.File, sectionList []string, config *MasterConfig) {
	for _, section_name := range sectionList {
		if strings.Index(section_name, "downloader.") == 0 {
			var new_downloader DownloaderConfig
			new_downloader.Name = section_name[11:]
			new_downloader.Source = cfg.Section(section_name).Key("source").String()
			new_downloader.SourcePath = cfg.Section(section_name).Key("source_path").String()
			new_downloader.TargetPath = cfg.Section(section_name).Key("target_path").String()
			downloader_enabled := strings.ToLower(cfg.Section(section_name).Key("enabled").String())
			if downloader_enabled == "true" {
				new_downloader.Enabled = true
			} else {
				new_downloader.Enabled = false
			}
			for _, server := range config.Servers {
				if server.Name == new_downloader.Source {
					new_downloader.SourceServer = server
				}
			}
			config.Downloaders = append(config.Downloaders, new_downloader)
		}
	}
}

func ReadConfig(path_to_config string, re_save bool) MasterConfig {

	var master_config MasterConfig

	cfg := readConfigString()

	sectionList := cfg.SectionStrings()

	parseSectionGeneral(cfg, sectionList, &master_config)
	parseSectionServer(cfg, sectionList, &master_config)
	parseSectionDownloader(cfg, sectionList, &master_config)

	if re_save {
		cfg.SaveTo("config.ini")
	}

	return master_config
}
