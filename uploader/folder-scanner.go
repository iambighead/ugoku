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

type FolderScanner struct {
	config.UploaderConfig
	started            bool
	logger             logger.Logger
	Default_sleep_time int
	LocalFolderMap     map[string]fs.FileInfo
}

type FileObj struct {
	Path string
	Stat fs.FileInfo
}

func (scanner *FolderScanner) scan(c chan FileObj, done chan int, watch_for_changes bool) {

	sleep_time := scanner.Default_sleep_time
	for {
		if scanner.started {
			var dispatched int

			// walk a directory
			filelist, err := utils.ReadFilelist(scanner.SourcePath)
			if err == nil {
				if len(filelist) > 0 {
					scanner.logger.Debug(fmt.Sprintf("found files: %d", len(filelist)))
				}
			} else {
				scanner.logger.Error(fmt.Sprintf("failed to scan source folder: %s", err.Error()))
			}

			// time.Sleep(1000 * time.Millisecond)

			for _, newfile := range filelist {

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
					oldfile_stat, ok := scanner.LocalFolderMap[newfile]
					if !ok {
						can_dispatch = true
					} else {
						last_modtime := oldfile_stat.ModTime().Unix()
						now_modtime := stat.ModTime().Unix()
						if last_modtime != now_modtime {
							// scanner.logger.Debug(fmt.Sprintf("watchFolder: %s time %d %d", newfile, last_modtime, now_modtime))
							can_dispatch = true
						}
					}

					scanner.LocalFolderMap[newfile] = stat
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
		}

		scanner.logger.Debug(fmt.Sprintf("sleep for %d seconds", sleep_time))
		time.Sleep(time.Duration(sleep_time) * time.Second)
	}
}

func (scanner *FolderScanner) init() {
	scanner.started = false
	scanner.LocalFolderMap = make(map[string]fs.FileInfo)
	scanner.logger = logger.NewLogger(fmt.Sprintf("folder-scanner[%s]", scanner.Name))
	if scanner.Default_sleep_time <= 0 {
		scanner.Default_sleep_time = 1
	}
}

func (scanner *FolderScanner) Start(c chan FileObj, done chan int) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done, false)
}

func (scanner *FolderScanner) StartWithWatcher(c chan FileObj, done chan int) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done, true)
}

func (scanner *FolderScanner) Stop(c chan string) {
	scanner.logger.Info("stopping")
	scanner.started = false
}
