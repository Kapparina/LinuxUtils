package logging

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"

	"LinuxUtils/cmd/input"
)

type LogStyle struct {
	Foreground lipgloss.Color
	Bold       bool
	InfoString string
}

type EventType int

const (
	Create EventType = iota
	Modify
	Rename
	Remove
)

func (e EventType) String() string {
	events := [...]string{
		"CREATE",
		"MODIFY",
		"RENAME",
		"REMOVE",
	}
	return events[e]
}

type EventLogger struct {
	logger *log.Logger
	style  *LogStyle
}

var (
	loggerOptions = log.Options{
		ReportTimestamp: true,
		Level: func() log.Level {
			if input.Debug {
				return log.DebugLevel
			} else {
				return log.InfoLevel
			}
		}(),
	}
	logMap = map[fsnotify.Op]EventLogger{
		fsnotify.Create: {
			style: &LogStyle{
				Foreground: lipgloss.Color("#00FF00"),
				Bold:       true,
				InfoString: "CREATE",
			},
		},
		fsnotify.Write: {
			style: &LogStyle{
				Foreground: lipgloss.Color("#FFFF00"),
				Bold:       true,
				InfoString: "MODIFY",
			},
		},
		fsnotify.Rename: {
			style: &LogStyle{
				Foreground: lipgloss.Color("#00FFFF"),
				Bold:       true,
				InfoString: "RENAME",
			},
		},
		fsnotify.Remove: {
			style: &LogStyle{
				Foreground: lipgloss.Color("#FF0000"),
				Bold:       true,
				InfoString: "REMOVE",
			},
		},
	}
)

func init() {
	for k, v := range logMap {
		v.logger = log.NewWithOptions(
			os.Stderr,
			loggerOptions,
		)
		logMap[k] = v
		styleLogger(v.logger, v.style)
	}
}

func styleLogger(l *log.Logger, style *LogStyle) {
	styles := log.DefaultStyles()
	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		SetString(style.InfoString).
		Foreground(style.Foreground).
		Bold(style.Bold)
	l.SetStyles(styles)
}

func LogEvent(event fsnotify.Event) {
	if v, ok := logMap[event.Op]; ok {
		v.logger.Info(event.Name)
	}
	// switch {
	// case event.Has(fsnotify.Create):
	// 	logMap[Create].logger.Info(event.Name)
	// case event.Has(fsnotify.Write):
	// 	logMap[Modify].logger.Info(event.Name)
	// case event.Has(fsnotify.Remove):
	// 	logMap[Remove].logger.Info(event.Name)
	// case event.Has(fsnotify.Rename):
	// 	logMap[Rename].logger.Info(event.Name)
	// }
}
