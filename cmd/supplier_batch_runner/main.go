package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := loadConfigFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "supplier batch runner configuration error:", err)
		os.Exit(2)
	}
	runner := newRunner(config)
	status, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "supplier batch runner failed:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "supplier batch request %s completed: processed_days=%d remaining_work=%t\n", status.RequestID, status.Result.ProcessedDays, status.Result.RemainingWork)
}
