package io

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

const (
	// WorkQueueCapacity defines the buffer size for pending directories
	WorkQueueCapacity = 100
	// WorkerCount defines the number of concurrent directory scanners
	WorkerCount = 3
	// ThrottleInterval controls the scan rate to avoid filesystem hammering
	ThrottleInterval = 50 * time.Millisecond
	// DedupInterval prevents rescanning the same directory too frequently
	DedupInterval = 3 * time.Second
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

// NewDirScanner creates a new directory scanner with the specified watcher and quit signal
func NewDirScanner(watcher *fsnotify.Watcher, quit chan struct{}) *DirScanner {
	ds := &DirScanner{
		watcher:    watcher,
		workQueue:  make(chan string, WorkQueueCapacity),
		quitSignal: quit,
		scanCache:  make(map[string]time.Time),
		throttle:   time.NewTicker(ThrottleInterval),
	}
	// Start the worker pool
	for i := 0; i < WorkerCount; i++ {
		go ds.worker()
	}
	return ds
}

// ScheduleScan adds a directory to be scanned, with deduplication
func (ds *DirScanner) ScheduleScan(dir string) {
	if ds.recentlyScanned(dir) {
		return
	}
	// Non-blocking send to workQueue
	select {
	case ds.workQueue <- dir:
		// Successfully scheduled
	default:
		// Queue full, log and drop
		log.Debug("Directory scan queue full, skipping path", "path", dir)
	}
}

// recentlyScanned checks if the directory was scanned recently
func (ds *DirScanner) recentlyScanned(dir string) bool {
	ds.cacheMutex.Lock()
	defer ds.cacheMutex.Unlock()
	lastScan, exists := ds.scanCache[dir]
	now := time.Now()
	if exists && now.Sub(lastScan) < DedupInterval {
		return true
	}
	ds.scanCache[dir] = now
	return false
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
		ds.scanDirectory(dir)
	}
}

// scanDirectory processes a single directory, adding it and its subdirectories to the watcher
func (ds *DirScanner) scanDirectory(dir string) {
	defer ds.activeScans.Done()
	// Skip if watcher is closed
	if ds.isWatcherClosed() {
		return
	}
	// First, try to add the directory itself
	if err := ds.watcher.Add(dir); err != nil && !ds.handleWatchError(err, dir, "Failed to add directory to watcher") {
		return
	}
	// Scan for subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Debug("Failed to read directory", "error", err, "path", dir)
		}
		return
	}
	// Wait for throttle tick to avoid hammering the filesystem
	<-ds.throttle.C
	// Process subdirectories
	ds.processSubdirectories(dir, entries)
}

// processSubdirectories handles the scanning of subdirectories
func (ds *DirScanner) processSubdirectories(parentDir string, entries []os.DirEntry) {
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
		path := filepath.Join(parentDir, entry.Name())
		// Try to add to watcher
		if err := ds.watcher.Add(path); err != nil && !ds.handleWatchError(err, path, "Failed to add subdirectory to watcher") {
			continue
		}
		// Schedule scan of this subdirectory
		ds.ScheduleScan(path)
	}
}

// handleWatchError processes watcher errors, returns false if processing should stop
func (ds *DirScanner) handleWatchError(err error, path, message string) bool {
	if errors.Is(err, fsnotify.ErrClosed) || strings.Contains(err.Error(), "closed") {
		return false
	}
	if !os.IsNotExist(err) {
		log.Debug(message, "error", err, "path", path)
	}
	return true
}

// isWatcherClosed checks if the watcher is closed
func (ds *DirScanner) isWatcherClosed() bool {
	select {
	case <-ds.watcher.Events:
		return true
	case <-ds.watcher.Errors:
		return true
	default:
		// Non-blocking check, watcher appears to be open
		return false
	}
}
