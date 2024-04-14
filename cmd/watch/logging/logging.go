package logging

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type LogStyle struct {
	Foreground lipgloss.Color
	Bold       bool
	InfoString string
}

var (
	CreateLog *log.Logger
	ModifyLog *log.Logger
	RenameLog *log.Logger
	RemoveLog *log.Logger
)

func init() {
	CreateLog = log.NewWithOptions(
		os.Stderr,
		log.Options{
			ReportTimestamp: true,
		},
	)
	ModifyLog = log.NewWithOptions(
		os.Stderr,
		log.Options{
			ReportTimestamp: true,
		},
	)
	RenameLog = log.NewWithOptions(
		os.Stderr,
		log.Options{
			ReportTimestamp: true,
		},
	)
	RemoveLog = log.NewWithOptions(
		os.Stderr,
		log.Options{
			ReportTimestamp: true,
		},
	)
}

func InitialiseLoggers() {
	styleLoggers()
}

func styleLoggers() {
	styleMap := map[*log.Logger]LogStyle{
		CreateLog: {
			Foreground: lipgloss.Color("#00FF00"),
			Bold:       true,
			InfoString: "CREATE",
		},
		ModifyLog: {
			Foreground: lipgloss.Color("#FFFF00"),
			Bold:       true,
			InfoString: "MODIFY",
		},
		RenameLog: {
			Foreground: lipgloss.Color("#00FFFF"),
			Bold:       true,
			InfoString: "RENAME",
		},
		RemoveLog: {
			Foreground: lipgloss.Color("#FF0000"),
			Bold:       true,
			InfoString: "REMOVE",
		},
	}
	for l, s := range styleMap {
		styleLogger(l, &s)
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
