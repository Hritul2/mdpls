// Command mdpls previews a repo's markdown in the browser, live.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Hritul2/mdpls/internal/app"
	"github.com/Hritul2/mdpls/internal/browser"
)

// version is set at release time: -ldflags "-X main.version=1.0.0".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a := &app.App{Stdout: os.Stdout, Stderr: os.Stderr, Version: version, Open: browser.Open}
	code := a.Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}
