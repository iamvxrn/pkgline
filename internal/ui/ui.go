package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"pkgline/internal/config"
	"pkgline/internal/path"
)

var (
	useColor = true
)

func init() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		useColor = false
	}
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func colorize(color, msg string) string {
	if !useColor {
		return msg
	}
	return color + msg + colorReset
}

// LogInfo prints an informational message.
func LogInfo(format string, a ...interface{}) {
	prefix := colorize(colorCyan+colorBold, "==>")
	fmt.Printf("%s %s\n", prefix, fmt.Sprintf(format, a...))
}

// LogSuccess prints a success message.
func LogSuccess(format string, a ...interface{}) {
	prefix := colorize(colorGreen+colorBold, "  [ok]")
	fmt.Printf("%s %s\n", prefix, fmt.Sprintf(format, a...))
}

// LogWarning prints a warning message.
func LogWarning(format string, a ...interface{}) {
	prefix := colorize(colorYellow+colorBold, "  [warn]")
	fmt.Printf("%s %s\n", prefix, fmt.Sprintf(format, a...))
}

// LogError prints an error message.
func LogError(format string, a ...interface{}) {
	prefix := colorize(colorRed+colorBold, "  Error:")
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, fmt.Sprintf(format, a...))
}

// CheckPathWarning checks if ~/.pkgline/bin is present in PATH environment variable.
func CheckPathWarning() {
	binDir := path.BinDir()
	if cfg, err := config.LoadConfig(); err == nil && cfg.BinDir != "" {
		binDir = cfg.BinDir
	}
	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)

	found := false
	for _, p := range paths {
		if filepath.Clean(p) == filepath.Clean(binDir) {
			found = true
			break
		}
	}

	if !found {
		LogWarning("'%s' is not in your PATH.", binDir)
		fmt.Printf("      Add it to your shell config (~/.bashrc, ~/.zshrc):\n")
		fmt.Printf("      %s\n\n", colorize(colorBold, fmt.Sprintf("export PATH=\"%s:$PATH\"", binDir)))
	}
}

// PrintTable formats tabular data cleanly.
func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Header
	headerLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "\t"
		}
		headerLine += colorize(colorBold, strings.ToUpper(h))
	}
	_, _ = fmt.Fprintln(w, headerLine)

	// Rows
	for _, r := range rows {
		rowLine := ""
		for i, cell := range r {
			if i > 0 {
				rowLine += "\t"
			}
			rowLine += cell
		}
		_, _ = fmt.Fprintln(w, rowLine)
	}

	_ = w.Flush()
}
