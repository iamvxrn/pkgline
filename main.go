package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"pkgline/internal/config"
	"pkgline/internal/core"
	"pkgline/internal/manifest"
	"pkgline/internal/path"
	"pkgline/internal/pkglinefile"
	"pkgline/internal/search"
	"pkgline/internal/ui"
)

const version = "0.4.0"

func printUsage() {
	banner := `Pkgline - Decentralized, User-Space Package Manager

Usage:
  pkgline [--json] <command> [arguments]

Commands:
  install, i <uri>    Install package from Git URL, gh:/gl:/cb:/sh: shorthand, alias, or local path
                      Flags: --lang <language>  --exec <name>
  run, r <uri> [-- --] Run package without installing (temp build, like npx)
                      Flags: --lang <language>  --exec <name>  -- <args> forwarded to binary
  publish             Generate pkgline.toml interactively for current directory
                      Flags: --force  --yes
  bootstrap           Install all packages from Pkglinefile
                      Flags: --file <path>  --dry-run
  search <query>      Search GitHub for repos containing pkgline.toml
                      Flags: --limit <n> (default 10)
  remove, rm <name>   Remove an installed package and its linked binary
  rollback <name>     Restore previous backup executable (.bak) for package
  sync, update [name] Pull latest changes and rebuild package(s) if updated
  list, ls            List all installed packages
  doctor              Check Pkgline installation status and shell environment
  completion <shell>  Generate shell auto-completion script (bash, zsh, fish)
  version             Print Pkgline version
  help                Show this help message

Global flags:
  --json              Machine-readable output (list, doctor, version, search)
`

	fmt.Print(banner)
}

func runPublish(force, yes bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	target := filepath.Join(cwd, manifest.ManifestFileName)
	if _, err := os.Stat(target); err == nil && !force {
		return fmt.Errorf("pkgline.toml already exists at %s (use --force to overwrite)", target)
	}

	// Infer defaults.
	defName := filepath.Base(cwd)
	if defName == "." || defName == "/" || defName == "" {
		defName = "my-package"
	}
	// Try to improve name from go.mod if present.
	if data, err := os.ReadFile(filepath.Join(cwd, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if rest, ok := strings.CutPrefix(line, "module "); ok {
				mod := strings.TrimSpace(rest)
				if idx := strings.LastIndex(mod, "/"); idx >= 0 {
					mod = mod[idx+1:]
				}
				if mod != "" {
					defName = mod
				}
				break
			}
		}
	}
	defVersion := "0.1.0"
	defLang := ""
	// Infer language like manifest does.
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		defLang = "go"
	} else if _, err := os.Stat(filepath.Join(cwd, "Cargo.toml")); err == nil {
		defLang = "rust"
	} else if _, err := os.Stat(filepath.Join(cwd, "cbld.toml")); err == nil {
		defLang = "cbld"
	} else if _, err := os.Stat(filepath.Join(cwd, "build.zig")); err == nil {
		defLang = "zig"
	} else if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		defLang = "node"
	} else if _, err := os.Stat(filepath.Join(cwd, "CMakeLists.txt")); err == nil {
		defLang = "cmake"
	} else if _, err := os.Stat(filepath.Join(cwd, "Makefile")); err == nil {
		defLang = "make"
	} else if _, err := os.Stat(filepath.Join(cwd, "makefile")); err == nil {
		defLang = "make"
	}
	defExec := defName

	reader := bufio.NewReader(os.Stdin)
	prompt := func(label, def string) string {
		if yes {
			return def
		}
		if def != "" {
			fmt.Printf("%s [%s]: ", label, def)
		} else {
			fmt.Printf("%s: ", label)
		}
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			return def
		}
		return text
	}

	fmt.Println("Generating pkgline.toml — press Enter to accept defaults.")
	name := prompt("Package name", defName)
	version := prompt("Version", defVersion)
	lang := prompt("Language (go, rust, cbld, c, cpp, make, cmake, zig, node, or empty for script)", defLang)
	execName := prompt("Executable name", defExec)

	// Basic validation.
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("package name is required")
	}
	if strings.TrimSpace(lang) != "" {
		switch strings.ToLower(strings.TrimSpace(lang)) {
		case "go", "rust", "cbld", "c", "cpp", "make", "cmake", "zig", "node", "nodejs", "js", "ts":
		default:
			return fmt.Errorf("unsupported language %q", lang)
		}
	}

	m := manifest.Manifest{
		Package: manifest.PackageConfig{
			Name:       strings.TrimSpace(name),
			Version:    strings.TrimSpace(version),
			Language:   strings.ToLower(strings.TrimSpace(lang)),
			Executable: strings.TrimSpace(execName),
		},
	}
	if m.Package.Executable == m.Package.Name {
		m.Package.Executable = ""
	}
	if err := m.Validate(); err != nil {
		// Allow script fallback: if language empty and no script, still require install script.
		if strings.TrimSpace(lang) == "" {
			// Prompt for install script if needed.
			script := prompt("Install script (e.g. install.sh, empty to skip)", "")
			if strings.TrimSpace(script) != "" {
				m.Scripts.Install = strings.TrimSpace(script)
				if err := m.Validate(); err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	}

	// Encode to TOML
	var buf strings.Builder
	buf.WriteString("[package]\n")
	fmt.Fprintf(&buf, "name = %q\n", m.Package.Name)
	fmt.Fprintf(&buf, "version = %q\n", m.Package.Version)
	if strings.TrimSpace(m.Package.Language) != "" {
		fmt.Fprintf(&buf, "language = %q\n", m.Package.Language)
	}
	if strings.TrimSpace(m.Package.Executable) != "" {
		fmt.Fprintf(&buf, "executable = %q\n", m.Package.Executable)
	}
	if strings.TrimSpace(m.Scripts.Install) != "" {
		buf.WriteString("\n[scripts]\n")
		fmt.Fprintf(&buf, "install = %q\n", m.Scripts.Install)
	}
	content := buf.String()
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", target, err)
	}
	ui.LogSuccess("Wrote %s", target)
	fmt.Println("\nNext: git add pkgline.toml && git commit -m \"Add pkgline.toml\" && git push")
	fmt.Println("Others can then install via: pkgline install gh:<owner>/<repo>")
	return nil
}

func isHelpArg(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

func stripJSONFlag(args []string) (bool, []string) {
	jsonOut := false
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		out = append(out, a)
	}
	return jsonOut, out
}

func parseInstallArgs(args []string) (uri, lang, execName string, err error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		take := func(flag string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for %s", flag)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--lang" || a == "--language":
			lang, err = take(a)
			if err != nil {
				return "", "", "", err
			}
		case strings.HasPrefix(a, "--lang="):
			lang = strings.TrimPrefix(a, "--lang=")
		case strings.HasPrefix(a, "--language="):
			lang = strings.TrimPrefix(a, "--language=")
		case a == "--exec" || a == "--executable":
			execName, err = take(a)
			if err != nil {
				return "", "", "", err
			}
		case strings.HasPrefix(a, "--exec="):
			execName = strings.TrimPrefix(a, "--exec=")
		case strings.HasPrefix(a, "--executable="):
			execName = strings.TrimPrefix(a, "--executable=")
		case a == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			return "", "", "", fmt.Errorf("unknown flag %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 {
		return "", "", "", fmt.Errorf("missing URI or package specification")
	}
	if len(positional) > 1 {
		return "", "", "", fmt.Errorf("unexpected extra argument %q", positional[1])
	}
	return positional[0], lang, execName, nil
}

func parseBootstrapArgs(args []string) (filePath string, dryRun bool, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--file" || a == "-f":
			if i+1 >= len(args) {
				return "", false, fmt.Errorf("missing value for %s", a)
			}
			i++
			filePath = args[i]
		case strings.HasPrefix(a, "--file="):
			filePath = strings.TrimPrefix(a, "--file=")
		case a == "--dry-run" || a == "-n":
			dryRun = true
		case strings.HasPrefix(a, "-") && a != "-":
			return "", false, fmt.Errorf("unknown flag %s", a)
		default:
			return "", false, fmt.Errorf("unexpected argument %q", a)
		}
	}
	return filePath, dryRun, nil
}

func parseSearchArgs(args []string) (query string, limit int, err error) {
	limit = 10
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--limit" || a == "-n":
			if i+1 >= len(args) {
				return "", 0, fmt.Errorf("missing value for %s", a)
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v <= 0 {
				return "", 0, fmt.Errorf("invalid limit %q", args[i])
			}
			limit = v
		case strings.HasPrefix(a, "--limit="):
			vStr := strings.TrimPrefix(a, "--limit=")
			v, err := strconv.Atoi(vStr)
			if err != nil || v <= 0 {
				return "", 0, fmt.Errorf("invalid limit %q", vStr)
			}
			limit = v
		case a == "--help" || a == "-h":
			return "", 0, fmt.Errorf("help")
		case strings.HasPrefix(a, "-") && a != "-":
			return "", 0, fmt.Errorf("unknown flag %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 {
		return "", 0, fmt.Errorf("missing search query")
	}
	query = strings.Join(positional, " ")
	return query, limit, nil
}

func parseRunArgs(args []string) (uri, lang, execName string, binArgs []string, err error) {
	var positional []string
	binArgs = []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		take := func(flag string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for %s", flag)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--lang" || a == "--language":
			lang, err = take(a)
			if err != nil {
				return "", "", "", nil, err
			}
		case strings.HasPrefix(a, "--lang="):
			lang = strings.TrimPrefix(a, "--lang=")
		case strings.HasPrefix(a, "--language="):
			lang = strings.TrimPrefix(a, "--language=")
		case a == "--exec" || a == "--executable":
			execName, err = take(a)
			if err != nil {
				return "", "", "", nil, err
			}
		case strings.HasPrefix(a, "--exec="):
			execName = strings.TrimPrefix(a, "--exec=")
		case strings.HasPrefix(a, "--executable="):
			execName = strings.TrimPrefix(a, "--executable=")
		case a == "--":
			binArgs = append(binArgs, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			return "", "", "", nil, fmt.Errorf("unknown flag %s", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 {
		return "", "", "", nil, fmt.Errorf("missing URI or package specification")
	}
	if len(positional) > 1 {
		return "", "", "", nil, fmt.Errorf("unexpected extra argument %q", positional[1])
	}
	return positional[0], lang, execName, binArgs, nil
}

func main() {
	jsonOut, args := stripJSONFlag(os.Args)
	os.Args = args

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	subcommand := strings.ToLower(os.Args[1])

	switch subcommand {
	case "install", "i":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			printUsage()
			return
		}
		if len(os.Args) < 3 {
			ui.LogError("Missing URI or package specification.")
			fmt.Println("Usage: pkgline install [--lang <language>] [--exec <name>] <uri>")
			os.Exit(1)
		}
		uri, lang, execName, err := parseInstallArgs(os.Args[2:])
		if err != nil {
			ui.LogError("%v", err)
			fmt.Println("Usage: pkgline install [--lang <language>] [--exec <name>] <uri>")
			os.Exit(1)
		}
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.InstallWith(uri, core.InstallOpts{Language: lang, Executable: execName}); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "run", "r":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			printUsage()
			return
		}
		if len(os.Args) < 3 {
			ui.LogError("Missing URI or package specification.")
			fmt.Println("Usage: pkgline run [--lang <language>] [--exec <name>] <uri> [-- <args>]")
			os.Exit(1)
		}
		uri, lang, execName, binArgs, err := parseRunArgs(os.Args[2:])
		if err != nil {
			ui.LogError("%v", err)
			fmt.Println("Usage: pkgline run [--lang <language>] [--exec <name>] <uri> [-- <args>]")
			os.Exit(1)
		}
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.Run(uri, core.InstallOpts{Language: lang, Executable: execName}, binArgs); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "publish":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			fmt.Println("Usage: pkgline publish [--force] [--yes]")
			fmt.Println("  Interactively generate pkgline.toml for the current directory.")
			return
		}
		force := false
		yes := false
		for _, a := range os.Args[2:] {
			switch a {
			case "--force", "-f":
				force = true
			case "--yes", "-y":
				yes = true
			case "--help", "-h":
				fmt.Println("Usage: pkgline publish [--force] [--yes]")
				return
			default:
				ui.LogError("Unknown flag %q for publish", a)
				os.Exit(1)
			}
		}
		if err := runPublish(force, yes); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "bootstrap":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			fmt.Println("Usage: pkgline bootstrap [--file <path>] [--dry-run]")
			fmt.Println("  Install all packages from Pkglinefile (walks up from cwd).")
			return
		}
		bFile, dryRun, err := parseBootstrapArgs(os.Args[2:])
		if err != nil {
			ui.LogError("%v", err)
			fmt.Println("Usage: pkgline bootstrap [--file <path>] [--dry-run]")
			os.Exit(1)
		}
		if bFile != "" && isHelpArg(bFile) {
			fmt.Println("Usage: pkgline bootstrap [--file <path>] [--dry-run]")
			return
		}
		var pfPath string
		if bFile != "" {
			pfPath = bFile
			if _, err := os.Stat(pfPath); err != nil {
				ui.LogError("Pkglinefile not found at %s", pfPath)
				os.Exit(1)
			}
		} else {
			cwd, _ := os.Getwd()
			pfPath, err = pkglinefile.Discover(cwd)
			if err != nil {
				ui.LogError("%v", err)
				os.Exit(1)
			}
			if pfPath == "" {
				ui.LogError("No Pkglinefile found (searched up from %s). Create one with one package per line.", cwd)
				fmt.Println("Example Pkglinefile:")
				fmt.Println("  gh:user/repo")
				fmt.Println("  gh:user/other@v1.2.0 --lang go --exec mybin")
				os.Exit(1)
			}
		}
		pf, err := pkglinefile.ParseFile(pfPath)
		if err != nil {
			ui.LogError("Failed to parse %s: %v", pfPath, err)
			os.Exit(1)
		}
		if len(pf.Entries) == 0 {
			ui.LogWarning("No packages in %s", pfPath)
			return
		}
		ui.LogInfo("Found %d package(s) in %s", len(pf.Entries), pfPath)
		if dryRun {
			for _, e := range pf.Entries {
				extra := ""
				if e.Lang != "" {
					extra += " --lang " + e.Lang
				}
				if e.Exec != "" {
					extra += " --exec " + e.Exec
				}
				fmt.Printf("  would install: %s%s\n", e.URI, extra)
			}
			return
		}
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		baseDir := filepath.Dir(pfPath)
		failed := 0
		for _, e := range pf.Entries {
			uri := e.ResolvedURI(baseDir)
			ui.LogInfo("Bootstrapping %s ...", e.URI)
			if err := installer.InstallWith(uri, core.InstallOpts{Language: e.Lang, Executable: e.Exec}); err != nil {
				ui.LogError("Failed %s: %v", e.URI, err)
				failed++
				continue
			}
		}
		if failed > 0 {
			ui.LogWarning("Bootstrap complete with %d failure(s) / %d total", failed, len(pf.Entries))
			os.Exit(1)
		}
		ui.LogSuccess("Bootstrap complete: %d package(s) from %s", len(pf.Entries), pfPath)

	case "search":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			fmt.Println("Usage: pkgline search [--limit <n>] <query>")
			fmt.Println("  Search GitHub for repos containing pkgline.toml.")
			return
		}
		if len(os.Args) < 3 {
			ui.LogError("Missing search query.")
			fmt.Println("Usage: pkgline search [--limit <n>] <query>")
			os.Exit(1)
		}
		query, limit, err := parseSearchArgs(os.Args[2:])
		if err != nil {
			if err.Error() == "help" {
				fmt.Println("Usage: pkgline search [--limit <n>] <query>")
				return
			}
			ui.LogError("%v", err)
			fmt.Println("Usage: pkgline search [--limit <n>] <query>")
			os.Exit(1)
		}
		results, err := search.SearchGitHub(search.SearchOptions{Query: query, Limit: limit})
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if len(results) == 0 {
			ui.LogInfo("No results for %q", query)
			if !jsonOut {
				fmt.Println("Try a broader query or check GITHUB_TOKEN for higher rate limits.")
			} else {
				_ = json.NewEncoder(os.Stdout).Encode([]search.Result{})
			}
			return
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(results)
			return
		}
		ui.LogSuccess("Found %d result(s) for %q:", len(results), query)
		for _, r := range results {
			stars := ""
			if r.Stars > 0 {
				stars = fmt.Sprintf(" ★ %d", r.Stars)
			}
			desc := ""
			if r.Description != "" {
				desc = " — " + r.Description
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
			}
			fmt.Printf("  %-30s%s%s\n", r.Repo, stars, desc)
			fmt.Printf("    %s\n", r.URL)
			fmt.Printf("    install: pkgline install gh:%s\n", r.Repo)
		}

	case "remove", "rm", "uninstall":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			printUsage()
			return
		}
		if len(os.Args) < 3 {
			ui.LogError("Missing package name.")
			fmt.Println("Usage: pkgline remove <package-name>")
			os.Exit(1)
		}
		pkgName := os.Args[2]
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.Remove(pkgName); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "rollback":
		if len(os.Args) >= 3 && isHelpArg(os.Args[2]) {
			printUsage()
			return
		}
		if len(os.Args) < 3 {
			ui.LogError("Missing package name.")
			fmt.Println("Usage: pkgline rollback <package-name>")
			os.Exit(1)
		}
		pkgName := os.Args[2]
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.Rollback(pkgName); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "sync", "update":
		pkgName := ""
		if len(os.Args) >= 3 {
			pkgName = os.Args[2]
		}
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.Sync(pkgName); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "list", "ls":
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.List(jsonOut); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "doctor":
		runDoctor(jsonOut)

	case "completion":
		shell := "zsh"
		if len(os.Args) >= 3 {
			shell = os.Args[2]
		}
		printCompletion(shell)

	case "version", "-v", "--version":
		if jsonOut {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"version": version})
		} else {
			fmt.Printf("Pkgline v%s\n", version)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		ui.LogError("Unknown command '%s'", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func runDoctor(jsonOut bool) {
	binDir := path.BinDir()
	appsDir := path.AppsDir()
	if cfg, err := config.LoadConfig(); err == nil {
		binDir = cfg.BinDir
		appsDir = cfg.AppsDir
	}
	pathEnv := os.Getenv("PATH")
	inPath := false
	for _, p := range filepath.SplitList(pathEnv) {
		if filepath.Clean(p) == filepath.Clean(binDir) {
			inPath = true
			break
		}
	}

	if jsonOut {
		payload := map[string]any{
			"version":     version,
			"root":        path.PkglineRoot(),
			"bin_dir":     binDir,
			"apps_dir":    appsDir,
			"config":      path.ConfigPath(),
			"bin_in_path": inPath,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return
	}

	ui.LogInfo("Running Pkgline system diagnostics...")
	fmt.Printf("  • Pkgline Version: v%s\n", version)
	fmt.Printf("  • Pkgline Root:    %s\n", path.PkglineRoot())
	fmt.Printf("  • Bin Dir:      %s\n", binDir)
	fmt.Printf("  • Apps Dir:     %s\n", appsDir)
	fmt.Printf("  • Config File:  %s\n", path.ConfigPath())

	if inPath {
		ui.LogSuccess("PATH configuration is correct (%s is in PATH).", binDir)
	} else {
		ui.LogWarning("PATH configuration missing. '%s' is NOT in $PATH.", binDir)
		fmt.Printf("    To fix, add this line to your ~/.bashrc or ~/.zshrc:\n")
		fmt.Printf("    export PATH=\"%s:$PATH\"\n", binDir)
	}
}

func printCompletion(shell string) {
	switch strings.ToLower(shell) {
	case "bash":
		fmt.Print(`# pkgline bash completion
_pkgline_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local cmds="install run publish bootstrap search remove rollback sync list doctor version help completion"
    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${cmds}" -- ${cur}) )
    fi
}
complete -F _pkgline_completions pkgline
`)
	case "zsh":
		fmt.Print(`#compdef pkgline
_pkgline() {
    local -a commands
    commands=(
        'install:Install package from Git URL or spec'
        'run:Build and run without installing'
        'publish:Generate pkgline.toml for current directory'
        'bootstrap:Install all packages from Pkglinefile'
        'search:Search GitHub for repos with pkgline.toml'
        'remove:Remove installed package'
        'rollback:Restore previous backup executable'
        'sync:Pull latest changes and rebuild package'
        'list:List all installed packages'
        'doctor:Check Pkgline installation status'
        'version:Print Pkgline version'
        'completion:Generate shell auto-completion script'
    )
    _describe 'command' commands
}
_pkgline "$@"
`)
	case "fish":
		fmt.Print(`# pkgline fish completion
complete -c pkgline -f
complete -c pkgline -n "not __fish_seen_subcommand_from install run publish bootstrap search remove rollback sync list doctor version completion" -a "install run publish bootstrap search remove rollback sync list doctor version completion"
`)
	default:
		ui.LogError("Unsupported shell '%s'. Supported shells: bash, zsh, fish", shell)
		os.Exit(1)
	}
}
