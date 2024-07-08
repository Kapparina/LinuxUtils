package main

import (
	"os"
	"os/signal"
	"path/filepath"
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
		err = watcher.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(watcher)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		done <- nil
	}()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				fileName := filepath.Base(event.Name)
				if event.Has(fsnotify.Create) {
					logging.CreateLog.Info(fileName)
				}
				if event.Has(fsnotify.Write) {
					logging.ModifyLog.Info(fileName)
				}
				if event.Has(fsnotify.Remove) {
					logging.RemoveLog.Info(fileName)
				}
				if event.Has(fsnotify.Rename) {
					logging.RenameLog.Info(fileName)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("error:", err)
			case <-done:
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
