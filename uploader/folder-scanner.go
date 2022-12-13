package uploader

import (
	"fmt"
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
	started bool
	logger  logger.Logger
}

func (scanner *FolderScanner) scan(c chan string, done chan int) {
	// walk a directory
	sleep_time := 1
	for {
		if scanner.started {
			var dispatched int

			filelist, err := utils.ReadFilelist(scanner.SourcePath)
			if err == nil {
				if len(filelist) > 0 {
					scanner.logger.Debug(fmt.Sprintf("found files: %d", len(filelist)))
				}
			} else {
				scanner.logger.Error(fmt.Sprintf("failed to scan source folder: %s", err.Error()))
			}

			time.Sleep(1000 * time.Millisecond)

			for _, newfile := range filelist {
				select {
				// Put new file in the channel unless it is full
				case c <- newfile:
					dispatched++
					scanner.logger.Debug(fmt.Sprintf("sent file to channel: %s, dispatched %d, ch %d/%d", newfile, dispatched, len(c), cap(c)))

				default:
					scanner.logger.Debug(fmt.Sprintf("channel full (%d dispatched) wait for something done first", dispatched))
					<-done
					dispatched--
					scanner.logger.Debug(fmt.Sprintf("done received, %d dispatched now", dispatched))
					c <- newfile
					dispatched++
					scanner.logger.Debug(fmt.Sprintf("sent file to channel: %s, dispatched %d, ch %d/%d", newfile, dispatched, len(c), cap(c)))
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
	scanner.logger = logger.NewLogger(fmt.Sprintf("folder-scanner[%s]", scanner.Name))
}

func (scanner *FolderScanner) Start(c chan string, done chan int) {
	scanner.init()
	scanner.started = true
	scanner.scan(c, done)
}

func (scanner *FolderScanner) Stop(c chan string) {
	scanner.started = false
}
