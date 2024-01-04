package siginthandler

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type callback func()

func Handle(label string, cb callback) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Printf("%s: signal received: %s\n", label, sig)
		cb()
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}
