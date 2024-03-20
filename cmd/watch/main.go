package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"LinuxUtils/pkg/io"
	"LinuxUtils/pkg/parsing"
	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
)

var (
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	// blue    = color.New(color.FgBlue).SprintFunc()
	// magenta = color.New(color.FgMagenta).SprintFunc()
	cyan = color.New(color.FgCyan).SprintFunc()
	// white   = color.New(color.FgWhite).SprintFunc()
)

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
		log.Printf("Received signal: %v\n", sig)
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
					log.Printf("%s %15s\n", green("CREATE:"), filepath.Base(event.Name))
				}
				if event.Has(fsnotify.Write) {
					log.Printf("%s %15s\n", yellow("MODIFY:"), filepath.Base(event.Name))
				}
				if event.Has(fsnotify.Remove) {
					log.Printf("%s %15s\n", red("DELETE:"), filepath.Base(event.Name))
				}
				if event.Has(fsnotify.Rename) {
					log.Printf("%s %15s\n", cyan("RENAME:"), filepath.Base(event.Name))
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
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
	log.Printf("Commencing watch: %s\n", relPath)
	err = watcher.Add(targetDir)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	log.Println("Ending watch")
}
