package uploader

import (
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/iambighead/goutils/logger"
	"github.com/iambighead/goutils/utils"
	"github.com/iambighead/ugoku/internal/config"
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

type FileObj struct {
	Path string
	Stat fs.FileInfo
}

type FileLookupObj struct {
	Pass int
	Stat fs.FileInfo
}
type FolderScanner struct {
	config.UploaderConfig
	started            bool
	logger             logger.Logger
	Default_sleep_time int
	LocalFolderMap     map[string]FileLookupObj
}

func (scanner *FolderScanner) scan(c chan FileObj, done chan int, watch_for_changes bool, scan_one_time_only bool) {

	sleep_time := scanner.Default_sleep_time
	currnet_pass := 0
	for {
		currnet_pass = currnet_pass + 1%10

		if !scanner.started {
			scanner.logger.Info("folder scanner stopped, exiting scan")
			return
		}

		var dispatched int

		// walk a directory
		filelist, err := utils.ReadFilelist(scanner.SourcePath)
		if err == nil {
			// if len(filelist) > 0 {
			// 	scanner.logger.Debug(fmt.Sprintf("found files: %d", len(filelist)))
			// }
		} else {
			scanner.logger.Error(fmt.Sprintf("failed to scan source folder: %s", err.Error()))
		}

		// time.Sleep(1000 * time.Millisecond)

		for _, newfile := range filelist {

			if !scanner.started {
				scanner.logger.Info("folder scanner stopped, exiting scan")
				return
			}

			stat, err := os.Stat(newfile)
			if err != nil {
				scanner.logger.Error(fmt.Sprintf("unable to stat file: %s", newfile))
				continue
			}

			var rf FileObj
			rf.Path = newfile
			rf.Stat = stat

			can_dispatch := false
			if watch_for_changes {
				oldfile, ok := scanner.LocalFolderMap[newfile]
				if !ok {
					can_dispatch = true
				} else {
					last_modtime := oldfile.Stat.ModTime().Unix()
					now_modtime := stat.ModTime().Unix()
					if last_modtime != now_modtime {
						// scanner.logger.Debug(fmt.Sprintf("watchFolder: %s time %d %d", newfile, last_modtime, now_modtime))
						can_dispatch = true
					}
				}
				scanner.LocalFolderMap[newfile] = FileLookupObj{Pass: currnet_pass, Stat: stat}
			} else {
				can_dispatch = true
			}

			if can_dispatch {
				select {
				// Put new file in the channel unless it is full
				case c <- rf:
					dispatched++
					scanner.logger.Debug(fmt.Sprintf("sent file to channel: %s, dispatched %d, ch %d/%d", newfile, dispatched, len(c), cap(c)))

				default:
					scanner.logger.Debug(fmt.Sprintf("channel full (%d dispatched) wait for something done first", dispatched))
					<-done
					dispatched--
					scanner.logger.Debug(fmt.Sprintf("done received, %d dispatched now", dispatched))
					c <- rf
					dispatched++
					scanner.logger.Debug(fmt.Sprintf("sent file to channel: %s, dispatched %d, ch %d/%d", newfile, dispatched, len(c), cap(c)))
				}
			}
		}

		for file, fo := range scanner.LocalFolderMap {
			if fo.Pass != currnet_pass {
				delete(scanner.LocalFolderMap, file)
				scanner.logger.Debug(fmt.Sprintf("removed file %s", file))
			}
		}

		// lookup_len := len(scanner.LocalFolderMap)
		// if lookup_len > 0 {
		// 	scanner.logger.Debug(fmt.Sprintf("file lookup length is now %d", lookup_len))
		// }

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

		if scan_one_time_only {
			// scanner.logger.Info("scan only one time")
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}
		// scanner.logger.Info("sleep and scan again")
		// scanner.logger.Debug(fmt.Sprintf("sleep for %d seconds", sleep_time))
		time.Sleep(time.Duration(sleep_time) * time.Second)
	}
}

func (scanner *FolderScanner) init() {
	scanner.started = false
	scanner.LocalFolderMap = make(map[string]FileLookupObj)
	scanner.logger = logger.NewLogger(fmt.Sprintf("folder-scanner[%s]", scanner.Name))
	if scanner.Default_sleep_time <= 0 {
		scanner.Default_sleep_time = 1
	}
}

func (scanner *FolderScanner) Start(c chan FileObj, done chan int, scan_one_time_only bool) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done, false, scan_one_time_only)
}

func (scanner *FolderScanner) StartWithWatcher(c chan FileObj, done chan int, scan_one_time_only bool) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done, true, scan_one_time_only)
}

func (scanner *FolderScanner) Stop() {
	scanner.logger.Info("folder scanner stopping")
	scanner.started = false
}
