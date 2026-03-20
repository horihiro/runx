package main

import (
	"fmt"
	"os"

	"horihiro.net/runx/commands"
)

// version is overridden at build time with:
// -ldflags "-X main.version=<tag>"
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return nil
	}

	switch args[0] {
	case "exec":
		return commands.ExecCommand(args[1:])
	case "add":
		return commands.AddCommand(args[1:])
	case "remove":
		return commands.RemoveCommand(args[1:])
	case "list":
		return commands.ListCommand(args[1:])
	case "help", "--help", "-h":
		printHelp()
		return nil
	case "version", "--version", "-v":
		fmt.Printf("runx version %s\n", version)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printHelp() {
	fmt.Println("runx - run command with merged env files and proxy management")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  runx exec [--envfile=NAME ...] COMMAND [ARGS...]")
	fmt.Println("  runx add ORIGINAL_COMMAND [--alias=PROXY_NAME] [--envfile=NAME ...] [--shell=bash|zsh|fish]")
	fmt.Println("  runx remove COMMAND [--shell=bash|zsh|fish]")
	fmt.Println("  runx list [--shell=bash|zsh|fish]")
	fmt.Println("  runx help")
	fmt.Println("  runx version")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  runx exec --envfile=a.env --envfile=b.env env")
	fmt.Println("  runx add az --alias=mycmd --envfile=a.env --envfile=b.env --shell=zsh")
	fmt.Println("  runx remove mycmd --shell=fish")
	fmt.Println("  runx list --shell=bash")
	fmt.Println()
	fmt.Println("NOTES:")
	fmt.Println("  - --envfile accepts a file name or absolute path")
	fmt.Println("  - file name: searched from current dir to root, then home dir")
	fmt.Println("  - absolute path: only that file is checked")
	fmt.Println("  - for exec: env files are merged in order; later files override earlier values")
	fmt.Println("  - Linux proxy shells: auto-detected from $SHELL (bash|zsh|fish), override with --shell")
	fmt.Println("  - proxies: Linux uses shell function files/config, Windows uses %LOCALAPPDATA%\\runx\\bin")
}
