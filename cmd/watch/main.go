package main

import (
	"io/fs"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"

	"LinuxUtils/cmd/input"
	"LinuxUtils/cmd/watch/logging"
	"LinuxUtils/pkg/io"
)

var (
	targetDir string
	inputErr  error
)

func init() {
	targetDir, inputErr = input.ParseInput()
	if inputErr != nil {
		log.Fatal(inputErr)
	}
	if input.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
	}
	targetDir = strings.TrimSpace(targetDir)
	pathValidity, pathErr := io.ValidatePath(targetDir)
	if pathErr != nil || !pathValidity {
		log.Fatal(pathErr)
	}
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
	go watchLoop(watcher, quit)
	log.Info("Commencing watch | ", "target", io.ShortenPath(targetDir))
	watchErr := watcher.Add(targetDir)
	if watchErr != nil {
		log.Fatal(watchErr)
	}
	<-done
	log.Info("Ending watch")
}

func watchLoop(watcher *fsnotify.Watcher, quit chan struct{}) {
	var (
		waitDuration = 100 * time.Millisecond
		mu           sync.Mutex
		timers       = make(map[string]*time.Timer)
		logCallback  = func(e fsnotify.Event) {
			logging.LogEvent(targetDir, e)
			mu.Lock()
			delete(timers, e.Name)
			mu.Unlock()
		}
	)
	for {
		select {
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Errorf("Error: %v", err)
		case <-quit:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			isDir, _ := io.ValidatePath(event.Name)
			if event.Name != targetDir && isDir {
				if added, addErr := addToWatchList(event.Name, watcher); !added && addErr != nil {
					watcher.Errors <- addErr
				}
			}
			log.Debug("Event", "event", event)
			logging.LogEvent(targetDir, event)
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				if exists := itemInWatchList(event.Name, watcher); exists {
					// if err = watcher.Remove(event.Name); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
					// 	watcher.Errors <- err
					// }
				}
			}
		}
	}
}

func walkRecursively(path string, returnChan chan<- string) error {
	// files, err := os.ReadDir(path)
	// if err != nil {
	// 	return
	// }
	return fs.WalkDir(os.DirFS(path), ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			returnChan <- path
			return nil
		}
		return nil
	})
	// for _, file := range files {
	// 	if file.IsDir() {
	// 		returnChan <- file.Name()
	// 		go walkRecursively(file.Name(), returnChan)
	// 	}
	// }
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
