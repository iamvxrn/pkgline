package pkglinefile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Entry is one package spec from Pkglinefile.
type Entry struct {
	Spec string // raw spec, e.g. "gh:user/repo@v1.0" or "gh:user/repo --lang go"
	URI  string // package URI without --lang/--exec flags
	Lang string
	Exec string
	Line int // 1-indexed source line for diagnostics
}

// File represents a parsed Pkglinefile.
type File struct {
	Path    string
	Entries []Entry
}

// ResolvedURI returns URI resolved relative to the file's directory for local paths.
// Shorthand (gh:/gl:/cb:/sh:) and URLs are returned unchanged.
// Relative paths like ./tool or ../repo are joined with the Pkglinefile dir.
func (e Entry) ResolvedURI(baseDir string) string {
	uri := strings.TrimSpace(e.URI)
	if uri == "" {
		return uri
	}
	// Shorthand or URL: leave as is.
	if strings.Contains(uri, "://") || strings.HasPrefix(uri, "git@") {
		return uri
	}
	if strings.HasPrefix(uri, "gh:") || strings.HasPrefix(uri, "gl:") || strings.HasPrefix(uri, "cb:") || strings.HasPrefix(uri, "sh:") {
		return uri
	}
	// Bare owner/repo (github shorthand) contains single slash without dot prefix and no path separators at start.
	// Leave it; config will resolve to github.
	// Local path: starts with . / ~ or contains / with leading .
	if strings.HasPrefix(uri, ".") || strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "~") {
		if !filepath.IsAbs(uri) {
			// Expand ~ ?
			if strings.HasPrefix(uri, "~/") || uri == "~" {
				return uri // let config/path handle ~ later
			}
			return filepath.Join(baseDir, uri)
		}
		return uri
	}
	// If uri looks like a local relative path without prefix but the file exists relative to baseDir, resolve it.
	// Heuristic: contains "/" and no colon, and file exists as dir.
	if strings.Contains(uri, "/") && !strings.Contains(uri, ":") {
		candidate := filepath.Join(baseDir, uri)
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	return uri
}

const (
	PrimaryName  = "Pkglinefile"
	FallbackName = "Pkglinefile.toml"
	LegacyName   = ".pkglinefile"
)

// Discover walks up from dir to find a Pkglinefile.
// Checks Pkglinefile, then Pkglinefile.toml, then .pkglinefile at each level.
// Returns empty string if not found.
func Discover(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		for _, name := range []string{PrimaryName, FallbackName, LegacyName} {
			p := filepath.Join(abs, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, nil
			}
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return "", nil
}

// ParseFile reads and parses a Pkglinefile at path.
// For *.toml files, tries TOML array of strings/tables as fallback to plain parsing.
func ParseFile(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseReader(f, path)
}

// ParseReader parses Pkglinefile content from r.
// Format v1: plain text, one package spec per line.
// - blank lines ignored
// - lines where trimmed left starts with # are comments
// - inline flags --lang/--exec supported per line
// - trailing inline comment " # ..." is stripped unless inside quoted value
func ParseReader(r io.Reader, path string) (*File, error) {
	if strings.HasSuffix(strings.ToLower(path), ".toml") {
		// Try TOML parse first; if it fails, fall back to plain lines.
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		if f, tomlErr := parseToml(data, path); tomlErr == nil && f != nil && len(f.Entries) > 0 {
			return f, nil
		}
		// fallback: treat TOML content as plain lines (so comments still work)
		return parseLines(string(data), path)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parseLines(string(data), path)
}

func parseLines(content, path string) (*File, error) {
	f := &File{Path: path}
	sc := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		// full-line comment
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// strip inline comment: " # " outside quotes
		lineCore := stripInlineComment(raw)
		lineCore = strings.TrimSpace(lineCore)
		if lineCore == "" || strings.HasPrefix(lineCore, "#") {
			continue
		}
		entry, err := parseLine(lineCore, lineNo)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		f.Entries = append(f.Entries, entry)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return f, nil
}

// stripInlineComment removes " # ..." not inside quotes.
func stripInlineComment(s string) string {
	inSingle, inDouble := false, false
	for i, ch := range s {
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				// require preceding space to be inline comment
				if i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
					return strings.TrimSpace(s[:i])
				}
			}
		}
	}
	return s
}

func parseLine(line string, lineNo int) (Entry, error) {
	// Tokenize respecting quotes for --lang="go mod" like values.
	fields, err := splitFields(line)
	if err != nil {
		return Entry{}, err
	}
	if len(fields) == 0 {
		return Entry{}, fmt.Errorf("empty spec")
	}
	// Reuse install arg parsing semantics: flags --lang/--language, --exec/--executable.
	var positional []string
	var lang, execName string
	for i := 0; i < len(fields); i++ {
		a := fields[i]
		take := func(flag string) (string, error) {
			if i+1 >= len(fields) {
				return "", fmt.Errorf("missing value for %s", flag)
			}
			i++
			return fields[i], nil
		}
		switch {
		case a == "--lang" || a == "--language":
			v, err := take(a)
			if err != nil {
				return Entry{}, err
			}
			lang = v
		case strings.HasPrefix(a, "--lang="):
			lang = strings.TrimPrefix(a, "--lang=")
		case strings.HasPrefix(a, "--language="):
			lang = strings.TrimPrefix(a, "--language=")
		case a == "--exec" || a == "--executable":
			v, err := take(a)
			if err != nil {
				return Entry{}, err
			}
			execName = v
		case strings.HasPrefix(a, "--exec="):
			execName = strings.TrimPrefix(a, "--exec=")
		case strings.HasPrefix(a, "--executable="):
			execName = strings.TrimPrefix(a, "--executable=")
		case strings.HasPrefix(a, "-") && a != "-":
			return Entry{}, fmt.Errorf("unknown flag %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 {
		return Entry{}, fmt.Errorf("missing URI")
	}
	if len(positional) > 1 {
		return Entry{}, fmt.Errorf("unexpected extra argument %q", positional[1])
	}
	uri := positional[0]
	if strings.TrimSpace(uri) == "" {
		return Entry{}, fmt.Errorf("empty URI")
	}
	// Reconstruct spec for display (preserve original line trimmed)
	spec := line
	return Entry{
		Spec: spec,
		URI:  uri,
		Lang: lang,
		Exec: execName,
		Line: lineNo,
	}, nil
}

// splitFields splits line into tokens, honoring single/double quotes.
func splitFields(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	escaped := false
	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				continue
			}
			cur.WriteRune(r)
		case '"':
			if !inSingle {
				inDouble = !inDouble
				continue
			}
			cur.WriteRune(r)
		case ' ', '\t':
			if inSingle || inDouble {
				cur.WriteRune(r)
			} else {
				if cur.Len() > 0 {
					out = append(out, cur.String())
					cur.Reset()
				}
			}
		default:
			cur.WriteRune(r)
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote")
	}
	if escaped {
		return nil, fmt.Errorf("trailing escape")
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, nil
}

// parseToml handles Pkglinefile.toml with either:
// packages = ["gh:a/b", "gh:c/d --lang go"]
// or [[packages]] uri="..." lang="..."
func parseToml(data []byte, path string) (*File, error) {
	// Shape 1: packages = ["..."] (array of strings, each may contain flags)
	var s1 struct {
		Packages []string `toml:"packages"`
	}
	if err := toml.Unmarshal(data, &s1); err == nil && len(s1.Packages) > 0 {
		f := &File{Path: path}
		for i, spec := range s1.Packages {
			spec = strings.TrimSpace(spec)
			if spec == "" || strings.HasPrefix(spec, "#") {
				continue
			}
			e, err := parseLine(spec, i+1)
			if err != nil {
				return nil, fmt.Errorf("packages[%d]: %w", i, err)
			}
			f.Entries = append(f.Entries, e)
		}
		if len(f.Entries) > 0 {
			return f, nil
		}
	}
	// Shape 2: [[packages]] tables with uri/spec + optional lang/exec
	var s2 struct {
		Packages []struct {
			URI        string `toml:"uri"`
			URL        string `toml:"url"`
			Spec       string `toml:"spec"`
			Lang       string `toml:"lang"`
			Language   string `toml:"language"`
			Exec       string `toml:"exec"`
			Executable string `toml:"executable"`
		} `toml:"packages"`
	}
	if err := toml.Unmarshal(data, &s2); err == nil && len(s2.Packages) > 0 {
		f := &File{Path: path}
		for i, p := range s2.Packages {
			raw := p.Spec
			if raw == "" {
				raw = p.URI
			}
			if raw == "" {
				raw = p.URL
			}
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return nil, fmt.Errorf("packages[%d]: missing uri", i)
			}
			// If spec already contains flags, parse it; else use explicit fields.
			var e Entry
			if strings.Contains(raw, " --") {
				var err error
				e, err = parseLine(raw, i+1)
				if err != nil {
					return nil, err
				}
				// explicit fields override flags if present
				if p.Lang != "" {
					e.Lang = p.Lang
				} else if p.Language != "" {
					e.Lang = p.Language
				}
				if p.Exec != "" {
					e.Exec = p.Exec
				} else if p.Executable != "" {
					e.Exec = p.Executable
				}
			} else {
				e = Entry{
					Spec: raw,
					URI:  raw,
					Lang: p.Lang,
					Exec: p.Exec,
					Line: i + 1,
				}
				if e.Lang == "" {
					e.Lang = p.Language
				}
				if e.Exec == "" {
					e.Exec = p.Executable
				}
			}
			f.Entries = append(f.Entries, e)
		}
		if len(f.Entries) > 0 {
			return f, nil
		}
	}
	// Fallback: heuristic for informal TOML where BurntSushi strict fails (e.g. commented arrays)
	text := string(data)
	if !strings.Contains(text, "packages") {
		return nil, fmt.Errorf("no packages key")
	}
	f := &File{Path: path}
	lines := strings.Split(text, "\n")
	for idx, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		quoted := extractQuoted(trim)
		for _, q := range quoted {
			if q == "packages" {
				continue
			}
			if strings.Contains(q, "/") || strings.Contains(q, ":") {
				entry, err := parseLine(q, idx+1)
				if err != nil {
					continue
				}
				f.Entries = append(f.Entries, entry)
			}
		}
	}
	if len(f.Entries) == 0 {
		return nil, fmt.Errorf("no entries in toml")
	}
	return f, nil
}

func extractQuoted(s string) []string {
	var out []string
	inSingle, inDouble := false, false
	var cur strings.Builder
	for _, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				if inSingle {
					out = append(out, cur.String())
					cur.Reset()
					inSingle = false
				} else {
					inSingle = true
				}
				continue
			}
			if inSingle {
				cur.WriteRune(r)
			}
		case '"':
			if !inSingle {
				if inDouble {
					out = append(out, cur.String())
					cur.Reset()
					inDouble = false
				} else {
					inDouble = true
				}
				continue
			}
			if inDouble {
				cur.WriteRune(r)
			}
		default:
			if inSingle || inDouble {
				cur.WriteRune(r)
			}
		}
	}
	return out
}
