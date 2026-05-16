package cli

import (
	"fmt"
	"io"

	"github.com/khicago/supermover/internal/buildinfo"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "%s %s\n", buildinfo.Name, buildinfo.Version)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n", buildinfo.Name, args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `%s - %s

Usage:
  supermover <command> [flags]

Core commands:
  profile     Manage profile SSOT configuration
  serve       Run a trusted target receiver
  discover    Find local targets without trusting them
  pair        Pair with a target by explicit verification
  push        Push source roots to a paired target
  status      Show local profile/session status
  health      Inspect target control-plane health
  recover     Resume or repair incomplete sessions
  verify      Verify manifests and restored files
  deleted     Review and apply source-side soft deletes
  drift       Review target-local drift
  prune       Reclaim history after review and policy checks

Use "supermover help" for this overview.
`, buildinfo.Name, buildinfo.Description)
}
