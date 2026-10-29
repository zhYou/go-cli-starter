package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhYou/go-cli-starter/internal/cli"
)

const appName = "go-cli-starter"

// Set by release builds with -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], cli.Options{
		Name: appName, Version: version, Commit: commit, Date: date,
		In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr,
	})
	stop()
	os.Exit(code)
}
