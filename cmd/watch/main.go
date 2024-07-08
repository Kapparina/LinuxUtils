package main

import (
	"fmt"
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
	log.SetLevel(log.InfoLevel)
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
	shouldExit := make(chan bool, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		var fileName string
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Debug("Something isn't right...")
					return
				} else {
					fileName = filepath.Base(event.Name)
				}
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
					log.Error("Not okay:", "error", err)
					return
				}
				log.Error("An error occurred:", "error", err)
			case <-done:
				log.Debug("Received signal to stop watching...")
				shouldExit <- true
				return
			}
		}
	}()
	inputDir, inputErr := parsing.GetInput()
	if inputErr != nil {
		log.Fatal(inputErr)
	}
	pathInfo, pathErr := io.ValidatePath(strings.TrimSpace(inputDir))
	if pathErr != nil || !pathInfo.Valid {
		log.Fatal(pathErr)
	}
	logging.MinimalLog.Print(
		fmt.Sprintf("Watching '%s' for changes...", pathInfo.Truncate()),
		"type", pathInfo.Type,
	)
	err = watcher.Add(pathInfo.Path)
	if err != nil {
		log.Fatal(err)
	}
	<-shouldExit
	logging.MinimalLog.Print("Concluding watch...")
}
