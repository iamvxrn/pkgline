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
	colorReset     = "\033[0m"
	colorRed       = "\033[31m"
	colorGreen     = "\033[32m"
	colorYellow    = "\033[33m"
	colorBlue      = "\033[34m"
	colorMagenta   = "\033[35m"
	colorCyan      = "\033[36m"
	colorWhite     = "\033[37m"
	colorBold      = "\033[1m"
	colorDim       = "\033[2m"
	colorUnderline = "\033[4m"
	colorBrightRed     = "\033[91m"
	colorBrightGreen   = "\033[92m"
	colorBrightYellow  = "\033[93m"
	colorBrightBlue    = "\033[94m"
	colorBrightMagenta = "\033[95m"
	colorBrightCyan    = "\033[96m"
)

func colorize(color, msg string) string {
	if !useColor {
		return msg
	}
	return color + msg + colorReset
}

// LogInfo prints an informational message.
func LogInfo(format string, a ...interface{}) {
	prefix := colorize(colorBrightCyan+colorBold, "▶")
	fmt.Printf("%s %s\n", prefix, colorize(colorBrightCyan, fmt.Sprintf(format, a...)))
}

// LogSuccess prints a success message.
func LogSuccess(format string, a ...interface{}) {
	prefix := colorize(colorBrightGreen+colorBold, "✔")
	fmt.Printf("%s %s\n", prefix, colorize(colorBrightGreen, fmt.Sprintf(format, a...)))
}

// LogWarning prints a warning message.
func LogWarning(format string, a ...interface{}) {
	prefix := colorize(colorBrightYellow+colorBold, "⚠")
	fmt.Printf("%s %s\n", prefix, colorize(colorBrightYellow, fmt.Sprintf(format, a...)))
}

// LogError prints an error message.
func LogError(format string, a ...interface{}) {
	prefix := colorize(colorBrightRed+colorBold, "✘")
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, colorize(colorBrightRed, fmt.Sprintf(format, a...)))
}

// LogPackage prints a package-specific message with magenta highlight.
func LogPackage(pkg string, format string, a ...interface{}) {
	prefix := colorize(colorBrightMagenta+colorBold, "📦 "+pkg)
	fmt.Printf("%s %s\n", prefix, fmt.Sprintf(format, a...))
}

// LogSearch prints search-related info.
func LogSearch(format string, a ...interface{}) {
	prefix := colorize(colorBrightMagenta+colorBold, "🔍")
	fmt.Printf("%s %s\n", prefix, fmt.Sprintf(format, a...))
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

// PrintTable formats tabular data cleanly with vibrant colors.
func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Header with bright cyan + bold + underline
	headerLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "\t"
		}
		headerLine += colorize(colorBrightCyan+colorBold+colorUnderline, strings.ToUpper(h))
	}
	_, _ = fmt.Fprintln(w, headerLine)
	// Separator line
	sepLine := ""
	for i := range headers {
		if i > 0 {
			sepLine += "\t"
		}
		sepLine += colorize(colorDim, strings.Repeat("─", len(headers[i])+2))
	}
	_, _ = fmt.Fprintln(w, sepLine)

	// Rows with alternating subtle dim and colorful first column
	for idx, r := range rows {
		rowLine := ""
		for i, cell := range r {
			if i > 0 {
				rowLine += "\t"
			}
			c := cell
			// First column (Name/Package) in bright magenta
			if i == 0 {
				c = colorize(colorBrightMagenta+colorBold, cell)
			} else if i == 1 {
				// Version in bright green
				c = colorize(colorBrightGreen, cell)
			} else if i == 2 {
				// Type in bright yellow
				c = colorize(colorBrightYellow, cell)
			} else if idx%2 == 1 {
				c = colorize(colorDim, cell)
			}
			rowLine += c
		}
		_, _ = fmt.Fprintln(w, rowLine)
	}

	_ = w.Flush()
	// Footer with count
	if len(rows) > 0 {
		fmt.Printf("%s\n", colorize(colorDim, fmt.Sprintf("  %d package(s)", len(rows))))
	}
}

// PrintSearchResult prints a colorful search hit.
func PrintSearchResult(repo, stars, desc, url, install string) {
	fmt.Printf("  %s %s\n", colorize(colorBrightMagenta+colorBold, repo), colorize(colorBrightYellow, stars))
	if desc != "" {
		fmt.Printf("    %s\n", colorize(colorDim, desc))
	}
	fmt.Printf("    %s\n", colorize(colorBrightCyan+colorUnderline, url))
	fmt.Printf("    %s\n", colorize(colorBrightGreen, install))
}

// HighlightMatch highlights query inside text with bold yellow.
func HighlightMatch(text, query string) string {
	if query == "" || !useColor {
		return text
	}
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerText, lowerQuery)
	if idx < 0 {
		return text
	}
	before := text[:idx]
	match := text[idx : idx+len(query)]
	after := text[idx+len(query):]
	return before + colorize(colorBrightYellow+colorBold, match) + after
}
