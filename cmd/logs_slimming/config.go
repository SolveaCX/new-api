package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	dsnEnvironment   = "LOGS_SLIMMING_DSN"
	stagingSchema    = "newapi_staging"
	productionSchema = "newapi"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	allowedCommands   = []string{"preflight", "prepare", "backfill", "install-forward-trigger", "reconcile", "cutover", "rollback", "recover", "verify", "cleanup"}
)

type config struct {
	command              string
	dsn                  string
	schema               string
	source               string
	target               string
	old                  string
	checkpoint           string
	batch                string
	expectedProject      string
	expectedInstance     string
	expectedHostname     string
	expectedServerUUID   string
	expectedDatabaseUser string
	channelIDs           []int64
	batchSize            int
	batchDelay           time.Duration
	statementTimeout     time.Duration
	ddlTimeout           time.Duration
	lockWaitSeconds      int
	autoIncrementReserve uint64
	confirmCleanup       string
	evidencePath         string
	triggerDefiner       string
	phase                string
	upperBound           int64
	maxThreadsRunning    int
}

func parseConfig(args []string) (config, error) {
	if len(args) == 0 || !slices.Contains(allowedCommands, args[0]) {
		return config{}, fmt.Errorf("first argument must be one of: %s", strings.Join(allowedCommands, ", "))
	}
	cfg := config{command: args[0]}
	fs := flag.NewFlagSet("logs_slimming "+cfg.command, flag.ContinueOnError)
	fs.StringVar(&cfg.schema, "schema", stagingSchema, "database schema; staging is the default and only implicit target")
	fs.StringVar(&cfg.source, "source", "logs", "active source table")
	fs.StringVar(&cfg.target, "target", "", "owned shadow table")
	fs.StringVar(&cfg.old, "old", "", "owned rollback table")
	fs.StringVar(&cfg.checkpoint, "checkpoint-table", "", "owned durable checkpoint table")
	fs.StringVar(&cfg.batch, "batch", "", "immutable migration batch identifier")
	fs.StringVar(&cfg.expectedProject, "expected-project", "", "expected GCP project identity")
	fs.StringVar(&cfg.expectedInstance, "expected-instance", "", "expected Cloud SQL instance identity")
	fs.StringVar(&cfg.expectedHostname, "expected-hostname", "", "exact MySQL @@hostname")
	fs.StringVar(&cfg.expectedServerUUID, "expected-server-uuid", "", "exact MySQL @@server_uuid")
	fs.StringVar(&cfg.expectedDatabaseUser, "expected-db-user", "", "exact MySQL CURRENT_USER() in user@host form")
	channels := fs.String("channel-ids", "", "comma-separated frozen Codex channel IDs")
	fs.IntVar(&cfg.batchSize, "batch-size", 2000, "keyset rows per batch (1-5000)")
	fs.DurationVar(&cfg.batchDelay, "batch-delay", 250*time.Millisecond, "delay between batches")
	fs.DurationVar(&cfg.statementTimeout, "statement-timeout", 2*time.Second, "per data statement wall timeout")
	fs.DurationVar(&cfg.ddlTimeout, "ddl-timeout", 3*time.Second, "DDL wall watchdog timeout")
	fs.IntVar(&cfg.lockWaitSeconds, "lock-wait-seconds", 1, "MySQL metadata lock wait timeout (1-3)")
	reserve := fs.Uint64("auto-increment-reserve", 1_000_000, "cutover AUTO_INCREMENT safety reserve")
	fs.StringVar(&cfg.confirmCleanup, "confirm-cleanup", "", "must exactly equal ownership marker for cleanup")
	fs.StringVar(&cfg.evidencePath, "evidence", "", "append-only JSONL evidence path; defaults to stdout")
	fs.StringVar(&cfg.triggerDefiner, "trigger-definer", "", "durable MySQL account in user@host form")
	fs.StringVar(&cfg.phase, "phase", "seed", "checkpoint phase: seed, gap, fresh, incremental, rollback-gap")
	fs.Int64Var(&cfg.upperBound, "upper-bound", 0, "inclusive backfill/reconcile upper ID bound")
	fs.IntVar(&cfg.maxThreadsRunning, "max-threads-running", 32, "fail closed when Threads_running exceeds this value")
	if err := fs.Parse(args[1:]); err != nil {
		return config{}, err
	}
	cfg.dsn = os.Getenv(dsnEnvironment)
	cfg.autoIncrementReserve = *reserve
	parsed, err := parseChannelIDs(*channels)
	if err != nil {
		return config{}, err
	}
	cfg.channelIDs = parsed
	if cfg.target == "" && cfg.batch != "" {
		cfg.target = "logs_compact_" + cfg.batch
	}
	if cfg.old == "" && cfg.batch != "" {
		cfg.old = "logs_old_" + cfg.batch
	}
	if cfg.checkpoint == "" && cfg.batch != "" {
		cfg.checkpoint = "logs_slim_checkpoint_" + cfg.batch
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (c config) validate() error {
	if !slices.Contains(allowedCommands, c.command) {
		return fmt.Errorf("unsupported command %q", c.command)
	}
	if c.schema == productionSchema {
		return errors.New("this staging artifact is hard-denied from the production schema; production requires a separately reviewed build")
	} else if c.schema != stagingSchema {
		return fmt.Errorf("schema %q is denied; this artifact only permits %q", c.schema, stagingSchema)
	}
	if !regexp.MustCompile(`^[a-z0-9_]{1,32}$`).MatchString(c.batch) {
		return fmt.Errorf("unsafe or missing batch %q", c.batch)
	}
	for label, value := range map[string]string{
		"schema": c.schema, "source": c.source, "target": c.target,
		"old": c.old, "checkpoint": c.checkpoint,
	} {
		if !identifierPattern.MatchString(value) {
			return fmt.Errorf("unsafe or missing %s %q", label, value)
		}
	}
	if c.source == c.target || c.source == c.old || c.target == c.old {
		return errors.New("source, target, and old table names must be distinct")
	}
	if !strings.HasSuffix(c.target, c.batch) || !strings.HasSuffix(c.old, c.batch) || !strings.HasSuffix(c.checkpoint, c.batch) {
		return errors.New("target, old, and checkpoint names must end with the immutable batch")
	}
	if c.expectedProject == "" || c.expectedInstance == "" || c.expectedHostname == "" || c.expectedServerUUID == "" || c.expectedDatabaseUser == "" {
		return errors.New("expected project, instance, hostname, server UUID, and database user are all required")
	}
	if strings.ContainsAny(c.expectedDatabaseUser, "`'\";\\") || len(strings.Split(c.expectedDatabaseUser, "@")) != 2 {
		return errors.New("expected-db-user must be an exact simple user@host value")
	}
	if len(c.channelIDs) == 0 {
		return errors.New("at least one frozen channel ID is required")
	}
	if c.batchSize < 1 || c.batchSize > 5000 {
		return errors.New("batch-size must be between 1 and 5000")
	}
	if c.batchDelay < 0 || c.statementTimeout <= 0 || c.ddlTimeout < time.Second {
		return errors.New("timeouts must be positive and batch delay cannot be negative")
	}
	if c.lockWaitSeconds < 1 || c.lockWaitSeconds > 3 {
		return errors.New("lock-wait-seconds must be between 1 and 3")
	}
	if c.autoIncrementReserve < 1_000_000 {
		return errors.New("auto-increment-reserve cannot be less than 1000000")
	}
	if c.maxThreadsRunning < 1 || c.maxThreadsRunning > 256 {
		return errors.New("max-threads-running must be between 1 and 256")
	}
	if !slices.Contains([]string{"seed", "gap", "fresh", "incremental", "ddl-intent", "rollback-gap", "rollback-ready", "rollback-intent", "rollback-reconcile"}, c.phase) {
		return fmt.Errorf("unsupported phase %q", c.phase)
	}
	return nil
}

func parseChannelIDs(raw string) ([]int64, error) {
	seen := make(map[int64]struct{})
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid channel ID %q", part)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func quoteIdentifier(value string) (string, error) {
	if !identifierPattern.MatchString(value) {
		return "", fmt.Errorf("unsafe SQL identifier %q", value)
	}
	return "`" + value + "`", nil
}
