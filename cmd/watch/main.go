package main

import (
	"bufio"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
)

var (
	red     = color.New(color.FgRed).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
	cyan    = color.New(color.FgCyan).SprintFunc()
	white   = color.New(color.FgWhite).SprintFunc()
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
	targetDir, inputErr := getInput()
	if inputErr != nil {
		log.Fatal(inputErr)
	}
	targetDir = strings.TrimSpace(targetDir)
	pathValidity, pathErr := validatePath(targetDir)
	if pathErr != nil || !pathValidity {
		log.Fatal(pathErr)
	}
	relPath := shortenPath(targetDir)
	log.Printf("Commencing watch: %s\n", relPath)
	err = watcher.Add(targetDir)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	log.Println("Ending watch")
}

func getInput() (filePath string, inputErr error) {
	flag.Parse()
	if arg := flag.Arg(0); len(arg) > 1 {
		filePath = arg
	}
	if len(filePath) > 0 {
		filePath = strings.TrimSpace(filePath)
	} else {
		pipeInput, pipeErr := os.Stdin.Stat()
		if pipeErr != nil {
			inputErr = pipeErr
		}
		if pipeInput.Mode()&os.ModeNamedPipe != 0 {
			reader := bufio.NewReader(os.Stdin)
			input, bufferErr := reader.ReadString('\n')
			if bufferErr != nil {
				inputErr = bufferErr
			} else {
				filePath = strings.TrimSpace(input)
			}
		}
		if len(filePath) < 1 {
			cwd, err := os.Getwd()
			if err != nil {
				inputErr = err
			} else {
				filePath = cwd
			}
		}
	}
	return
}

func validatePath(path string) (bool, error) {
	if len(path) < 1 {
		return false, errors.New("path is empty")
	}
	file, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if file.IsDir() {
		return true, nil
	} else {
		return false, errors.New("path is not a directory")
	}
}

func shortenPath(path string) string {
	elements := strings.Split(path, string(filepath.Separator))
	if len(elements) <= 3 {
		return path
	} else {
		return filepath.Join(filepath.VolumeName(path), "...", filepath.Base(path))
	}
}
