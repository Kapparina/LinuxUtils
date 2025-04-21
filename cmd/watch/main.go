package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"

	"LinuxUtils/cmd/watch/logging"
	"LinuxUtils/pkg/io"
	"LinuxUtils/pkg/parsing"
)

func init() {
	log.SetLevel(log.DebugLevel)
	logging.InitialiseLoggers()
}

func main() {
	watcher, newWatchErr := fsnotify.NewWatcher()
	if newWatchErr != nil {
		log.Fatal(newWatchErr)
	}
	defer func(watcher *fsnotify.Watcher) {
		newWatchErr = watcher.Close()
		if newWatchErr != nil {
			log.Fatal(newWatchErr)
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
	targetDir, inputErr := parsing.GetInput()
	if inputErr != nil {
		log.Fatal(inputErr)
	}
	targetDir = strings.TrimSpace(targetDir)
	pathValidity, pathErr := io.ValidatePath(targetDir)
	if pathErr != nil || !pathValidity {
		log.Fatal(pathErr)
	}
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				isDir, _ := io.ValidatePath(event.Name)
				if event.Name != targetDir && isDir {
					var relDir string
					var err error
					if relDir, err = filepath.Rel(targetDir, event.Name); err != nil {
						watcher.Errors <- err
						continue
					}
					if added, err := addToWatchList(relDir, watcher); !added && err != nil {
						watcher.Errors <- err
						continue
					}
				}
				switch {
				case event.Has(fsnotify.Create):
					logging.CreateLog.Info(event.Name)
				case event.Has(fsnotify.Write):
					logging.ModifyLog.Info(event.Name)
				case event.Has(fsnotify.Remove):
					logging.RemoveLog.Info(event.Name)
				case event.Has(fsnotify.Rename):
					logging.RenameLog.Info(event.Name)
					if isDir {
						if err := watcher.Remove(event.Name); err != nil {
							watcher.Errors <- err
							continue
						}
					}
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
	log.Info("Commencing watch | ", "target", io.ShortenPath(targetDir))
	watchErr := watcher.Add(targetDir)
	if watchErr != nil {
		log.Fatal(watchErr)
	}
	<-done
	log.Info("Ending watch")
}

// addToWatchList attempts to add an item to the specified fsnotify.Watcher.
// Returns true if successful, false if the item is already in the watch list.
// Returns an error if the addition to the watcher fails.
func addToWatchList(item string, watcher *fsnotify.Watcher) (bool, error) {
	if itemInWatchList(item, watcher) {
		return false, nil
	}
	if err := watcher.Add(item); err != nil {
		return false, err
	}
	return true, nil
}

func itemInWatchList(item string, watcher *fsnotify.Watcher) bool {
	watchList := watcher.WatchList()
	slices.Sort(watchList)
	_, found := slices.BinarySearch(watchList, item)
	return found
}
