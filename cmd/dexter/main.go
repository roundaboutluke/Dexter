package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dexter/internal/app"
	"dexter/internal/logging"
)

func main() {
	ctx := context.Background()
	application := app.New()
	application.SetForceMigrations(hasArg(os.Args, "--force"))
	if err := application.Run(ctx); err != nil {
		logger := logging.Get().General
		if logger != nil {
			logger.Errorf("startup failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		}
		_ = application.Shutdown(ctx)
		os.Exit(1)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		logger := logging.Get().General
		if logger != nil {
			logger.Errorf("shutdown failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "shutdown failed: %v\n", err)
		}
		os.Exit(1)
	}
}

func hasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}
