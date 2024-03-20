package io

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ValidatePath(path string) (bool, error) {
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

func ShortenPath(path string) string {
	if elements := strings.Split(path, string(filepath.Separator)); len(elements) <= 3 {
		return path
	} else {
		return filepath.Join(filepath.VolumeName(path), "...", filepath.Base(path))
	}
}
