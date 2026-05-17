package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/khicago/supermover/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(cli.RunContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
