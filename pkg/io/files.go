package io

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type PathType string

type PathInfo struct {
	Path  string
	Valid bool
	Type  PathType
}

func (p *PathInfo) Truncate() string {
	return ShortenPath(p.Path)
}

func NewPathInfo(path string, valid bool, pathType PathType) PathInfo {
	return PathInfo{
		Path:  path,
		Valid: valid,
		Type:  pathType,
	}
}

const (
	DirectoryPath PathType = "Directory"
	FilePath      PathType = "File"
	InvalidPath   PathType = "None"
)

func ValidatePath(path string) (PathInfo, error) {
	if len(path) < 1 {
		return NewPathInfo(path, false, InvalidPath), errors.New("path is empty")
	}
	file, err := os.Stat(path)
	if err != nil {
		return NewPathInfo(path, false, InvalidPath), err
	}
	if file.IsDir() {
		return NewPathInfo(path, true, DirectoryPath), nil
	} else {
		return NewPathInfo(path, true, FilePath), errors.New("path is not a directory")
	}
}

func ShortenPath(path string) string {
	if elements := strings.Split(path, string(filepath.Separator)); len(elements) <= 3 {
		return path
	} else {
		return filepath.Join(filepath.VolumeName(path), "...", filepath.Base(path))
	}
}
