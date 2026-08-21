package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var sink io.Writer = os.Stdout
	var file *os.File
	if cfg.evidencePath != "" {
		file, err = os.OpenFile(cfg.evidencePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer file.Close()
		sink = file
	}
	ev := newEvidence(sink, []string{cfg.dsn})
	if cfg.dsn == "" {
		if err := ev.emit("refused", map[string]any{"error": dsnEnvironment + " is required"}); err != nil {
			fmt.Fprintln(os.Stderr, "write evidence:", err)
		}
		os.Exit(2)
	}
	db, err := sql.Open("mysql", cfg.dsn)
	if err != nil {
		if emitErr := ev.emit("failed", map[string]any{"error": err}); emitErr != nil {
			fmt.Fprintln(os.Stderr, "write evidence:", emitErr)
		}
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := runCommand(ctx, db, cfg, ev); err != nil {
		if emitErr := ev.emit("failed", map[string]any{"command": cfg.command, "error": err}); emitErr != nil {
			fmt.Fprintln(os.Stderr, "write failure evidence:", emitErr)
		}
		fmt.Fprintln(os.Stderr, "logs_slimming:", ev.redactString(err.Error()))
		os.Exit(1)
	}
	if err := ev.emit("completed", map[string]any{"command": cfg.command, "schema": cfg.schema, "batch": cfg.batch}); err != nil {
		fmt.Fprintln(os.Stderr, "write completion evidence:", err)
		os.Exit(1)
	}
}

func runCommand(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	if err := preflight(ctx, db, c, ev); err != nil {
		return err
	}
	if c.command != "preflight" && c.command != "prepare" {
		state, err := loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
		if err := assertFrozenTableFingerprints(ctx, db, c, state); err != nil {
			return err
		}
	}
	switch c.command {
	case "preflight":
		return nil
	case "prepare":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return prepare(lockCtx, db, c) })
	case "backfill", "reconcile":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return backfill(lockCtx, db, c, ev) })
	case "install-forward-trigger":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return installForward(lockCtx, db, c) })
	case "verify":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return verify(lockCtx, db, c, ev) })
	case "recover":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return recover(lockCtx, db, c, ev) })
	case "cleanup":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return cleanup(lockCtx, db, c) })
	case "cutover":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return cutover(lockCtx, db, c, ev) })
	case "rollback":
		return withLock(ctx, db, c, ev, func(lockCtx context.Context) error { return rollback(lockCtx, db, c, ev) })
	default:
		return fmt.Errorf("unsupported command %q", c.command)
	}
}
