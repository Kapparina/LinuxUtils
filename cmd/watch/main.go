package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"LinuxUtils/cmd/watch/logging"
	"LinuxUtils/pkg/io"
	"LinuxUtils/pkg/parsing"
	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
)

func init() {
	log.SetLevel(log.DebugLevel)
	logging.InitialiseLoggers()
}

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(watcher)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	done := make(chan bool, 1)
	quit := make(chan struct{})

	go func() {
		sig := <-sigs
		log.Debug("Received signal", "signal", sig)
		quit <- struct{}{}
		done <- true
	}()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) {
					logging.CreateLog.Info(event.Name)
				}
				if event.Has(fsnotify.Write) {
					logging.ModifyLog.Info(event.Name)
				}
				if event.Has(fsnotify.Remove) {
					logging.RemoveLog.Info(event.Name)
				}
				if event.Has(fsnotify.Rename) {
					logging.RenameLog.Info(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("error:", err)
			case <-quit:
				return
			}
		}
	}()
	targetDir, inputErr := parsing.GetInput()
	if inputErr != nil {
		log.Fatal(inputErr)
	}
	targetDir = strings.TrimSpace(targetDir)
	pathValidity, pathErr := io.ValidatePath(targetDir)
	if pathErr != nil || !pathValidity {
		log.Fatal(pathErr)
	}
	relPath := io.ShortenPath(targetDir)
	log.Info("Commencing watch | ", "target", relPath)
	err = watcher.Add(targetDir)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	log.Info("Ending watch")
}
