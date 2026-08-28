package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pkgline/internal/config"
	"pkgline/internal/core"
	"pkgline/internal/path"
	"pkgline/internal/ui"
)

const version = "0.3.0"

func printUsage() {
	banner := `Pkgline - Decentralized, User-Space Package Manager

Usage:
  pkgline [--json] <command> [arguments]

Commands:
  install, i <uri>    Install package from Git URL, gh:/gl:/cb:/sh: shorthand, alias, or local path
  remove, rm <name>   Remove an installed package and its linked binary
  rollback <name>     Restore previous backup executable (.bak) for package
  sync, update [name] Pull latest changes and rebuild package(s) if updated
  list, ls            List all installed packages
  doctor              Check Pkgline installation status and shell environment
  completion <shell>  Generate shell auto-completion script (bash, zsh, fish)
  version             Print Pkgline version
  help                Show this help message

Global flags:
  --json              Machine-readable output (list, doctor, version)
`

	fmt.Print(banner)
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
		if len(os.Args) < 3 {
			ui.LogError("Missing URI or package specification.")
			fmt.Println("Usage: pkgline install <uri>")
			os.Exit(1)
		}
		rawURI := os.Args[2]
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.Install(rawURI); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "remove", "rm", "uninstall":
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
    local cmds="install remove rollback sync list doctor version help completion"
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
complete -c pkgline -n "not __fish_seen_subcommand_from install remove rollback sync list doctor version completion" -a "install remove rollback sync list doctor version completion"
`)
	default:
		ui.LogError("Unsupported shell '%s'. Supported shells: bash, zsh, fish", shell)
		os.Exit(1)
	}
}
