package main

import (
	"math"
	"os"
	"os/signal"
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

	// Create a work manager for directory scanning
	dirScanner := io.NewDirScanner(watcher, quit)

	// Initial scan of the root directory
	dirScanner.ScheduleScan(targetDir)

	// Main event loop
	for {
		select {
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("Watcher error", "error", err)
		case <-quit:
			// Tell the scanner to stop and wait for it
			dirScanner.Shutdown()
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Handle directory creation
			if event.Has(fsnotify.Create) {
				fi, err := os.Stat(event.Name)
				if err == nil && fi.IsDir() {
					log.Debug("New directory created", "path", event.Name)
					dirScanner.ScheduleScan(event.Name)
				}
			}

			// Handle removals and renames
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				if exists := itemInWatchList(event.Name, watcher); exists {
					if err := watcher.Remove(event.Name); err != nil &&
						!errors.Is(err, fsnotify.ErrNonExistentWatch) {
						log.Warn("Failed to remove from watcher", "error", err, "path", event.Name)
					}
				}
			}

			// Debounce events
			mu.Lock()
			o, exists := occurrences[event.Name]
			if !exists {
				o.timer = time.AfterFunc(math.MaxInt64, func() { logCallback(event) })
				occurrences[event.Name] = o
			}
			mu.Unlock()
			o.timer.Reset(waitDuration)
		}
	}
}

func itemInWatchList(item string, watcher *fsnotify.Watcher) bool {
	watchList := watcher.WatchList()
	slices.Sort(watchList)
	_, found := slices.BinarySearch(watchList, item)
	return found
}
