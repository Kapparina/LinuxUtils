package main

import (
	"io/fs"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"

	"LinuxUtils/cmd/input"
	"LinuxUtils/cmd/watch/logging"
	"LinuxUtils/pkg/io"
)

type eventOccurrence struct {
	event fsnotify.Event
	timer *time.Timer
	op    fsnotify.Op
}

func (e eventOccurrence) Matches(v eventOccurrence) bool {
	if e.event.Name != v.event.Name || e.op&v.op != 0 {
		return false
	}
	return true
}

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
		occurrences  = make(map[string]eventOccurrence)
		logCallback  = func(e fsnotify.Event) {
			logging.LogEvent(targetDir, e)
			mu.Lock()
			delete(occurrences, e.Name)
			mu.Unlock()
		}
	)
	walkFunc := func(dir string) {
		recChan := make(chan string, 100)

		var wg sync.WaitGroup
		wg.Add(1)

		var err error
		go func() {
			defer wg.Done()
			err = walkRecursively(dir, recChan)
		}()
		go func() {
			wg.Wait()
			close(recChan)
		}()
		for p := range recChan {
			if itemInWatchList(p, watcher) {
				continue
			}
			if err = watcher.Add(p); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					log.Debug("Path does not exist", "path", p)
					continue
				}
				if errors.Is(err, os.ErrPermission) {
					log.Debug("Path is not readable", "path", p)
					continue
				}
				log.Warn("Failed to add to watcher", "error", err, "path", p)
			} else {
				log.Debug("Added to watcher", "path", p)
			}
		}
	}
	go walkFunc(targetDir)
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
			isDir, err := io.ValidatePath(event.Name)
			if event.Name != targetDir && err == nil && isDir {
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
					log.Debug("New directory detected", "path", event.Name)
					go walkFunc(event.Name)
				}
			}
			log.Debug("Event", "event", event)
			switch {
			}
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				if exists := itemInWatchList(event.Name, watcher); exists {
					if err := watcher.Remove(event.Name); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
						watcher.Errors <- err
					}
				}
			}
			mu.Lock()
			o, ok := occurrences[event.Name]
			if !ok {
				o.timer = time.AfterFunc(math.MaxInt64, func() { logCallback(event) })
				occurrences[event.Name] = o
			}
			mu.Unlock()
			o.timer.Reset(waitDuration)
		}
	}
}

func walkRecursively(rootPath string, returnChan chan<- string) error {
	fileSystem := os.DirFS(rootPath)
	return fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		if path == "." {
			return nil
		}
		if d.IsDir() {
			fullPath := filepath.Join(rootPath, path)
			if err != nil {
				return fs.SkipDir
			}
			returnChan <- fullPath
			log.Debug("Adding to watch list", "path", fullPath)
			return nil
		}
		return nil
	})
}

// addToWatchList attempts to add an item to the specified fsnotify.Watcher.
// Returns true if successful, false if the item is already in the watch list.
// Returns an error if the addition to the watcher fails.
// func addToWatchList(item string, watcher *fsnotify.Watcher) (bool, error) {
// 	if itemInWatchList(item, watcher) {
// 		return false, nil
// 	}
// 	if err := watcher.Add(item); err != nil {
// 		return false, err
// 	}
// 	return true, nil
// }

func itemInWatchList(item string, watcher *fsnotify.Watcher) bool {
	watchList := watcher.WatchList()
	slices.Sort(watchList)
	_, found := slices.BinarySearch(watchList, item)
	return found
}
