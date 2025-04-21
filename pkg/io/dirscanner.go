package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

// DirScanner manages directory scanning operations with intelligent
// throttling and graceful shutdown handling
type DirScanner struct {
	watcher     *fsnotify.Watcher
	workQueue   chan string
	quitSignal  chan struct{}
	activeScans sync.WaitGroup
	scanCache   map[string]time.Time
	cacheMutex  sync.Mutex
	throttle    *time.Ticker
}

func NewDirScanner(watcher *fsnotify.Watcher, quit chan struct{}) *DirScanner {
	ds := &DirScanner{
		watcher:    watcher,
		workQueue:  make(chan string, 100), // Buffer for pending directories
		quitSignal: quit,
		scanCache:  make(map[string]time.Time),
		throttle:   time.NewTicker(50 * time.Millisecond), // Throttle scans
	}

	// Start the worker pool
	for i := 0; i < 3; i++ { // 3 concurrent scanners
		go ds.worker()
	}

	return ds
}

// ScheduleScan adds a directory to be scanned, with deduplication
func (ds *DirScanner) ScheduleScan(dir string) {
	// Skip if recently scanned (deduplication)
	ds.cacheMutex.Lock()
	lastScan, exists := ds.scanCache[dir]
	now := time.Now()
	if exists && now.Sub(lastScan) < 3*time.Second {
		ds.cacheMutex.Unlock()
		return
	}
	ds.scanCache[dir] = now
	ds.cacheMutex.Unlock()

	// Non-blocking send to workQueue
	select {
	case ds.workQueue <- dir:
		// Successfully scheduled
	default:
		// Queue full, log and drop
		log.Debug("Directory scan queue full, skipping path", "path", dir)
	}
}

// Shutdown gracefully stops the scanner
func (ds *DirScanner) Shutdown() {
	close(ds.workQueue)
	ds.activeScans.Wait() // Wait for active scans to finish
	ds.throttle.Stop()
}

// worker processes directories from the work queue
func (ds *DirScanner) worker() {
	for dir := range ds.workQueue {
		// Check if quit signal received before each scan
		select {
		case <-ds.quitSignal:
			return
		default:
			// Continue with scan
		}

		// Mark this scan as active
		ds.activeScans.Add(1)

		// Perform the scan with error handling
		func(scanDir string) {
			defer ds.activeScans.Done()

			// Skip if watcher is known to be closed
			if isWatcherClosed(ds.watcher) {
				return
			}

			// First, try to add the directory itself
			if err := ds.watcher.Add(scanDir); err != nil {
				if errors.Is(err, fsnotify.ErrClosed) || strings.Contains(err.Error(), "closed") {
					return
				}
				if !os.IsNotExist(err) {
					log.Debug("Failed to add directory to watcher", "error", err, "path", scanDir)
				}
			}

			// Scan for subdirectories
			entries, err := os.ReadDir(scanDir)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Debug("Failed to read directory", "error", err, "path", scanDir)
				}
				return
			}

			// Wait for throttle tick to avoid hammering the filesystem
			<-ds.throttle.C

			// Process subdirectories
			for _, entry := range entries {
				// Check quit signal periodically
				select {
				case <-ds.quitSignal:
					return
				default:
					// Continue processing
				}

				if !entry.IsDir() {
					continue
				}

				path := filepath.Join(scanDir, entry.Name())

				// Try to add to watcher
				if err = ds.watcher.Add(path); err != nil {
					if errors.Is(err, fsnotify.ErrClosed) || strings.Contains(err.Error(), "closed") {
						return
					}
					if !os.IsNotExist(err) {
						log.Debug("Failed to add subdirectory to watcher", "error", err, "path", path)
					}
					continue
				}

				// Schedule scan of this subdirectory
				ds.ScheduleScan(path)
			}
		}(dir)
	}
}

// Helper function to check if watcher is closed
func isWatcherClosed(w *fsnotify.Watcher) bool {
	// Try a non-existent path - will return ErrClosed if watcher is closed
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("nonexistent-%d", time.Now().UnixNano()))
	err := w.Add(tempDir)
	return errors.Is(err, fsnotify.ErrClosed) || strings.Contains(err.Error(), "closed")
}
