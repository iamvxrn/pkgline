package main

import (
	"fmt"
	"os"
	"strings"

	"pkgline/internal/core"
	"pkgline/internal/ui"
)

const version = "0.1.0"

func printUsage() {
	banner := `Pkgline - Decentralized, User-Space Package Manager

Usage:
  pkgline <command> [arguments]

Commands:
  install, i <uri>    Install package from Git URL, gh:user/repo, or local path
  list, ls            List all installed packages
  version             Print Pkgline version
  help                Show this help message
`

	fmt.Print(banner)
}

func main() {
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

	case "list", "ls":
		installer, err := core.NewInstaller()
		if err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}
		if err := installer.List(); err != nil {
			ui.LogError("%v", err)
			os.Exit(1)
		}

	case "version", "-v", "--version":
		fmt.Printf("Pkgline v%s\n", version)

	case "help", "-h", "--help":
		printUsage()

	default:
		ui.LogError("Unknown command '%s'", subcommand)
		printUsage()
		os.Exit(1)
	}
}
