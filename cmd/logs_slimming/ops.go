package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type lockOwnerContextKey struct{}
type evidenceContextKey struct{}

func ownershipMarker(c config) string {
	payload := strings.Join([]string{c.expectedProject, c.expectedInstance, c.expectedServerUUID, c.schema, c.source, c.target, c.old, c.checkpoint, c.batch, channelList(c.channelIDs), auditedSchemaContractHash()}, "|")
	return fmt.Sprintf("logs-slimming:%s:%x", c.batch, sha256.Sum256([]byte(payload)))
}

func advisoryLockName(c config) string {
	payload := strings.Join([]string{c.expectedProject, c.expectedInstance, c.schema, c.source}, "|")
	digest := sha256.Sum256([]byte(payload))
	// MySQL user-level lock names are limited to 64 characters.
	return "logs-slimming:" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func preflight(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != c.expectedProject {
		return fmt.Errorf("GOOGLE_CLOUD_PROJECT does not equal expected-project")
	}
	if os.Getenv("CLOUD_SQL_INSTANCE") != c.expectedInstance {
		return fmt.Errorf("CLOUD_SQL_INSTANCE does not equal expected-instance")
	}
	var schema, hostname, uuid, version, isolation, binlog, currentUser string
	var autoMode int
	err := db.QueryRowContext(ctx, "SELECT DATABASE(), @@hostname, @@server_uuid, VERSION(), @@transaction_isolation, @@binlog_format, @@innodb_autoinc_lock_mode,CURRENT_USER()").Scan(&schema, &hostname, &uuid, &version, &isolation, &binlog, &autoMode, &currentUser)
	if err != nil {
		return fmt.Errorf("identity query: %w", err)
	}
	if schema != c.schema || hostname != c.expectedHostname || uuid != c.expectedServerUUID || currentUser != c.expectedDatabaseUser {
		return fmt.Errorf("database identity mismatch schema=%q hostname=%q server_uuid=%q current_user=%q", schema, hostname, uuid, currentUser)
	}
	if !strings.HasPrefix(version, "8.0.") || isolation != "READ-COMMITTED" || binlog != "ROW" || autoMode != 2 {
		return fmt.Errorf("unsupported session facts version=%q isolation=%q binlog=%q autoinc_mode=%d", version, isolation, binlog, autoMode)
	}
	if err := assertCutoverObservationAccess(ctx, db); err != nil {
		return err
	}
	var sourceExists int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", c.schema, c.source).Scan(&sourceExists); err != nil {
		return err
	}
	if sourceExists != 1 {
		return fmt.Errorf("source table is missing")
	}
	sourceFingerprint, err := schemaFingerprint(ctx, db, c.schema, c.source)
	if err != nil {
		return fmt.Errorf("fingerprint source table: %w", err)
	}
	rows, err := db.QueryContext(ctx, "SELECT id FROM channels WHERE type=57 ORDER BY id")
	if err != nil {
		return fmt.Errorf("read Codex channel snapshot: %w", err)
	}
	var currentChannelIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan Codex channel snapshot: %w", err)
		}
		currentChannelIDs = append(currentChannelIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate Codex channel snapshot: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close Codex channel snapshot: %w", err)
	}
	if err := validateChannelSnapshot(c.channelIDs, currentChannelIDs); err != nil {
		return err
	}
	return ev.emit("preflight_passed", map[string]any{"schema": schema, "hostname": hostname, "server_uuid": uuid, "version": version, "project": c.expectedProject, "instance": c.expectedInstance, "batch": c.batch, "channel_ids": c.channelIDs, "source_fingerprint": sourceFingerprint})
}

func assertCutoverObservationAccess(ctx context.Context, q queryRower) error {
	return probeCutoverObservationAccess(ctx, func(ctx context.Context, table string) error {
		var count int
		return q.QueryRowContext(ctx, cutoverObservationProbeSQL(table)).Scan(&count)
	})
}

func probeCutoverObservationAccess(ctx context.Context, probe func(context.Context, string) error) error {
	var denied []string
	var failed []string
	for _, table := range cutoverObservationTables {
		if err := probe(ctx, table); err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1142 {
				denied = append(denied, "performance_schema."+table)
				continue
			}
			failed = append(failed, fmt.Sprintf("performance_schema.%s: %v", table, err))
		}
	}
	if len(denied) == 0 && len(failed) == 0 {
		return nil
	}
	parts := make([]string, 0, 2)
	if len(denied) != 0 {
		parts = append(parts, "SELECT denied on "+strings.Join(denied, ", "))
	}
	if len(failed) != 0 {
		parts = append(parts, "probe execution failed: "+strings.Join(failed, "; "))
	}
	return fmt.Errorf("cutover observation unavailable: %s", strings.Join(parts, "; "))
}

var cutoverObservationTables = []string{"metadata_locks", "threads"}

func cutoverObservationProbeSQL(table string) string {
	return "SELECT COUNT(*) FROM performance_schema." + table + " WHERE 1=0"
}

func validateChannelSnapshot(configured, current []int64) error {
	if !slices.Equal(configured, current) {
		return fmt.Errorf("frozen Codex channel IDs do not match channels.type=57: configured=%v current=%v", configured, current)
	}
	return nil
}

func withLock(ctx context.Context, db *sql.DB, c config, ev *evidence, fn func(context.Context) error) error {
	conn, facts, err := openVerifiedConn(ctx, db, c, "lock")
	if err != nil {
		return err
	}
	defer conn.Close()
	lockName := advisoryLockName(c)
	var got int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lockName).Scan(&got); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}
	if got != 1 {
		return fmt.Errorf("advisory lock unavailable: got=%d", got)
	}
	ownerID := facts.connectionID
	lockCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-lockCtx.Done():
				return
			case <-ticker.C:
				var current sql.NullInt64
				checkCtx, stop := context.WithTimeout(lockCtx, time.Second)
				_, identityErr := verifyPhysicalConn(checkCtx, conn, c, "lock-heartbeat")
				err := conn.QueryRowContext(checkCtx, "SELECT IS_USED_LOCK(?)", lockName).Scan(&current)
				stop()
				if identityErr != nil || err != nil || !current.Valid || current.Int64 != ownerID {
					cancel(fmt.Errorf("advisory lock lost: owner=%d current=%v identity=%v err=%w", ownerID, current, identityErr, err))
					return
				}
			}
		}
	}()
	if err := ev.emit("advisory_lock_acquired", map[string]any{"lock": lockName, "connection_id": ownerID}); err != nil {
		cancel(err)
		<-done
		return err
	}
	lockCtx = context.WithValue(lockCtx, lockOwnerContextKey{}, ownerID)
	lockCtx = context.WithValue(lockCtx, evidenceContextKey{}, ev)
	err = fn(lockCtx)
	lockCause := context.Cause(lockCtx)
	cancel(nil)
	<-done
	if err == nil && lockCause == nil {
		var finalOwner sql.NullInt64
		checkCtx, checkStop := context.WithTimeout(context.Background(), time.Second)
		checkErr := conn.QueryRowContext(checkCtx, "SELECT IS_USED_LOCK(?)", lockName).Scan(&finalOwner)
		checkStop()
		if checkErr != nil || !finalOwner.Valid || finalOwner.Int64 != ownerID {
			err = fmt.Errorf("advisory lock ownership lost before command completion: %w", checkErr)
		}
	}
	var released sql.NullInt64
	releaseCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	_ = conn.QueryRowContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockName).Scan(&released)
	if err == nil && lockCause != nil {
		err = lockCause
	}
	return err
}

func prepare(ctx context.Context, db *sql.DB, c config) error {
	marker := ownershipMarker(c)
	target, _ := qualified(c.schema, c.target)
	source, _ := qualified(c.schema, c.source)
	cp, _ := qualified(c.schema, c.checkpoint)
	targetExists, targetComment, err := tableIdentity(ctx, db, c.schema, c.target)
	if err != nil {
		return err
	}
	if targetExists {
		if targetComment != marker {
			return fmt.Errorf("refusing to adopt existing unowned target %s", c.target)
		}
		sourceHash, err := schemaFingerprint(ctx, db, c.schema, c.source)
		if err != nil {
			return err
		}
		targetHash, err := schemaFingerprint(ctx, db, c.schema, c.target)
		if err != nil {
			return err
		}
		if sourceHash != targetHash {
			return fmt.Errorf("owned target schema fingerprint differs from source")
		}
	} else {
		if err := runDDL(ctx, db, c, "CREATE TABLE "+target+" LIKE "+source, tableStateObserver(c.schema, c.target, true)); err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, "ALTER TABLE "+target+" COMMENT='"+marker+"'", tableCommentObserver(c.schema, c.target, marker)); err != nil {
			return err
		}
	}
	sourceHash, err := auditedLogsSchemaHash(ctx, db, c.schema, c.source)
	if err != nil {
		return err
	}
	sourceFingerprint, err := schemaFingerprint(ctx, db, c.schema, c.source)
	if err != nil {
		return err
	}
	targetFingerprint, err := schemaFingerprint(ctx, db, c.schema, c.target)
	if err != nil {
		return err
	}
	if targetFingerprint != sourceFingerprint {
		return fmt.Errorf("target full schema/index fingerprint differs from source")
	}
	var sqlMode string
	if err := db.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&sqlMode); err != nil {
		return err
	}
	// Enforce append-only semantics before the first checkpoint can authorize a
	// backfill. This avoids privileged performance_schema access and prevents an
	// UPDATE or DELETE from racing the seed copy.
	for _, event := range []string{"update", "delete"} {
		query, err := buildGuardTriggerSQL(c, event, c.source)
		if err != nil {
			return err
		}
		spec, err := expectedTriggerSpec(c, "guard_"+event, c.source, sqlMode)
		if err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, query, exactTriggerObserver(spec, true)); err != nil {
			return err
		}
		// A separately named guard is preinstalled on the compact table. RENAME
		// moves it with that table, so the new live logs table is protected
		// atomically instead of waiting for post-cutover trigger DDL.
		futureKind := "future_guard_" + event
		futureQuery, err := buildNamedGuardTriggerSQL(c, futureKind, event, c.target)
		if err != nil {
			return err
		}
		futureSpec, err := expectedTriggerSpec(c, futureKind, c.target, sqlMode)
		if err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, futureQuery, exactTriggerObserver(futureSpec, true)); err != nil {
			return err
		}
	}
	statement := "CREATE TABLE " + cp + " (id TINYINT NOT NULL PRIMARY KEY, marker VARCHAR(191) NOT NULL, source_schema_sha256 CHAR(64) NOT NULL, source_fingerprint_sha256 CHAR(64) NOT NULL, phase VARCHAR(32) NOT NULL, last_completed_end_id BIGINT NOT NULL, seed_cutoff_id BIGINT NOT NULL, final_cutoff_id BIGINT NULL, rollback_base_id BIGINT NULL, generation BIGINT UNSIGNED NOT NULL, trigger_sql_mode TEXT NOT NULL, ddl_operation VARCHAR(64) NULL, ddl_nonce VARCHAR(64) NULL, baseline_updates BIGINT UNSIGNED NOT NULL, baseline_deletes BIGINT UNSIGNED NOT NULL, updated_at DATETIME(6) NOT NULL) ENGINE=InnoDB COMMENT='" + marker + "'"
	cpExists, cpComment, err := tableIdentity(ctx, db, c.schema, c.checkpoint)
	if err != nil {
		return err
	}
	if cpExists {
		if cpComment != marker {
			return fmt.Errorf("refusing to adopt existing unowned checkpoint %s", c.checkpoint)
		}
		if err := assertCheckpointSchema(ctx, db, c); err != nil {
			return err
		}
		state, err := loadCheckpoint(ctx, db, c)
		if err == nil {
			return assertFrozenTableFingerprints(ctx, db, c, state)
		}
		if err != sql.ErrNoRows {
			return err
		}
	} else {
		if err := runDDL(ctx, db, c, statement, tableCommentObserver(c.schema, c.checkpoint, marker)); err != nil {
			return err
		}
	}
	var maxID sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+source).Scan(&maxID); err != nil {
		return err
	}
	// Baseline counters remain zero for checkpoint v1 compatibility; exact guard
	// ownership is the authoritative append-only proof.
	_, err = db.ExecContext(ctx, "INSERT INTO "+cp+" (id,marker,source_schema_sha256,source_fingerprint_sha256,phase,last_completed_end_id,seed_cutoff_id,generation,trigger_sql_mode,baseline_updates,baseline_deletes,updated_at) VALUES (1,?,?,?,'seed',0,?,0,?,0,0,CURRENT_TIMESTAMP(6))", marker, sourceHash, sourceFingerprint, maxID.Int64, sqlMode)
	return err
}

type columnSpec struct {
	name, columnType, nullable, charset, collation, extra string
}

var allowedLogCollations = []string{"utf8mb4_0900_ai_ci", "utf8mb4_unicode_ci"}

var auditedLogColumns = []columnSpec{
	{"id", "bigint", "NO", "", "", "auto_increment"},
	{"user_id", "bigint", "YES", "", "", ""}, {"created_at", "bigint", "YES", "", "", ""}, {"type", "bigint", "YES", "", "", ""},
	{"content", "longtext", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""}, {"username", "varchar(191)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""},
	{"token_name", "varchar(191)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""}, {"model_name", "varchar(191)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""},
	{"quota", "bigint", "YES", "", "", ""}, {"prompt_tokens", "bigint", "YES", "", "", ""}, {"completion_tokens", "bigint", "YES", "", "", ""},
	{"use_time", "bigint", "YES", "", "", ""}, {"is_stream", "tinyint(1)", "YES", "", "", ""}, {"channel_id", "bigint", "YES", "", "", ""},
	{"channel_name", "longtext", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""}, {"token_id", "bigint", "YES", "", "", ""},
	{"group", "varchar(191)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""}, {"ip", "varchar(191)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""},
	{"request_id", "varchar(64)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""}, {"other", "longtext", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""},
	{"upstream_request_id", "varchar(128)", "YES", "utf8mb4", "utf8mb4_unicode_ci", ""},
}

func auditedLogsSchemaHash(ctx context.Context, q queryRowerQuerier, schema, table string) (string, error) {
	hash, productionErr := auditedColumnsHash(ctx, q, schema, table, auditedLogColumns)
	if productionErr == nil {
		return hash, nil
	}
	hash, stagingErr := auditedColumnsHash(ctx, q, schema, table, auditedStagingLogColumns())
	if stagingErr == nil {
		return hash, nil
	}
	return "", fmt.Errorf("table %s matches neither production nor staging audited logs schema: production=%v staging=%v", table, productionErr, stagingErr)
}

func auditedStagingLogColumns() []columnSpec {
	expected := slices.Clone(auditedLogColumns)
	expected[19], expected[20] = expected[20], expected[19]
	return expected
}

func auditedColumnsHash(ctx context.Context, q queryRowerQuerier, schema, table string, expected []columnSpec) (string, error) {
	rows, err := q.QueryContext(ctx, "SELECT COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COALESCE(CHARACTER_SET_NAME,''),COALESCE(COLLATION_NAME,''),EXTRA FROM information_schema.columns WHERE table_schema=? AND table_name=? ORDER BY ORDINAL_POSITION", schema, table)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	h := sha256.New()
	index := 0
	for rows.Next() {
		var got columnSpec
		if err := rows.Scan(&got.name, &got.columnType, &got.nullable, &got.charset, &got.collation, &got.extra); err != nil {
			return "", err
		}
		if index >= len(expected) || !matchesAuditedColumn(got, expected[index]) {
			return "", fmt.Errorf("table %s column %d differs from audited logs schema: got=%+v", table, index+1, got)
		}
		fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s\n", got.name, got.columnType, got.nullable, got.charset, got.collation, got.extra)
		index++
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if index != len(expected) {
		return "", fmt.Errorf("table %s has %d columns, audited schema requires %d", table, index, len(expected))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func matchesAuditedColumn(got, want columnSpec) bool {
	if got.charset == "utf8mb4" && want.charset == "utf8mb4" && slices.Contains(allowedLogCollations, got.collation) {
		got.collation = want.collation
	}
	return got == want
}

type queryRowerQuerier interface {
	queryRower
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func assertCheckpointSchema(ctx context.Context, db *sql.DB, c config) error {
	expected := []string{"id|tinyint|NO", "marker|varchar(191)|NO", "source_schema_sha256|char(64)|NO", "source_fingerprint_sha256|char(64)|NO", "phase|varchar(32)|NO", "last_completed_end_id|bigint|NO", "seed_cutoff_id|bigint|NO", "final_cutoff_id|bigint|YES", "rollback_base_id|bigint|YES", "generation|bigint unsigned|NO", "trigger_sql_mode|text|NO", "ddl_operation|varchar(64)|YES", "ddl_nonce|varchar(64)|YES", "baseline_updates|bigint unsigned|NO", "baseline_deletes|bigint unsigned|NO", "updated_at|datetime(6)|NO"}
	rows, err := db.QueryContext(ctx, "SELECT CONCAT(COLUMN_NAME,'|',COLUMN_TYPE,'|',IS_NULLABLE) FROM information_schema.columns WHERE table_schema=? AND table_name=? ORDER BY ORDINAL_POSITION", c.schema, c.checkpoint)
	if err != nil {
		return err
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		got = append(got, value)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !slices.Equal(got, expected) {
		return fmt.Errorf("checkpoint schema is not the exact owned v1 shape: got=%v", got)
	}
	return nil
}

func assertRuntimeSafe(ctx context.Context, db *sql.DB, c config, state checkpoint, requireAppendOnly bool) error {
	checkCtx, cancel := context.WithTimeout(ctx, c.statementTimeout)
	defer cancel()
	var variable string
	var threads int
	if err := db.QueryRowContext(checkCtx, "SHOW GLOBAL STATUS LIKE 'Threads_running'").Scan(&variable, &threads); err != nil {
		return fmt.Errorf("read Threads_running: %w", err)
	}
	if threads > c.maxThreadsRunning {
		return fmt.Errorf("Threads_running=%d exceeds stop threshold=%d", threads, c.maxThreadsRunning)
	}
	if requireAppendOnly {
		if err := assertAppendOnlyGuards(checkCtx, db, c, state.triggerSQLMode); err != nil {
			return err
		}
	}
	return assertFrozenTableFingerprints(checkCtx, db, c, state)
}

func assertAppendOnlyGuards(ctx context.Context, q queryRower, c config, sqlMode string) error {
	for _, event := range []string{"update", "delete"} {
		spec, err := expectedTriggerSpec(c, "guard_"+event, c.source, sqlMode)
		if err != nil {
			return err
		}
		got, exists, err := readTriggerSpec(ctx, q, c.schema, spec.name)
		if err != nil || !exists || !triggerMatches(got, spec) {
			return fmt.Errorf("append-only guard %s is missing or changed: exists=%t err=%v", spec.name, exists, err)
		}
	}
	return nil
}

func tableIdentity(ctx context.Context, db *sql.DB, schema, table string) (bool, string, error) {
	var comment string
	err := db.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&comment)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	return err == nil, comment, err
}

func schemaFingerprint(ctx context.Context, db *sql.DB, schema, table string) (string, error) {
	h := sha256.New()
	// Exclude mutable metadata such as AUTO_INCREMENT and TABLE_COMMENT, but bind
	// every physical table property that CREATE TABLE LIKE must preserve.
	var engine, rowFormat, collation, createOptions string
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(ENGINE,''),COALESCE(ROW_FORMAT,''),COALESCE(TABLE_COLLATION,''),COALESCE(CREATE_OPTIONS,'') FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&engine, &rowFormat, &collation, &createOptions); err != nil {
		return "", err
	}
	fmt.Fprintf(h, "table|%s|%s|%s|%s\n", engine, rowFormat, collation, createOptions)
	rows, err := db.QueryContext(ctx, "SELECT COLUMN_NAME,ORDINAL_POSITION,COALESCE(COLUMN_DEFAULT,'<NULL>'),IS_NULLABLE,DATA_TYPE,COLUMN_TYPE,COALESCE(CHARACTER_SET_NAME,'<NULL>'),COALESCE(COLLATION_NAME,'<NULL>'),EXTRA,COALESCE(GENERATION_EXPRESSION,'') FROM information_schema.columns WHERE table_schema=? AND table_name=? ORDER BY ORDINAL_POSITION", schema, table)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var a, b, c, d, e, f, g, i, j, k string
		if err := rows.Scan(&a, &b, &c, &d, &e, &f, &g, &i, &j, &k); err != nil {
			rows.Close()
			return "", err
		}
		fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n", a, b, c, d, e, f, g, i, j, k)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	rows, err = db.QueryContext(ctx, "SELECT INDEX_NAME,NON_UNIQUE,SEQ_IN_INDEX,COALESCE(COLUMN_NAME,'<NULL>'),COALESCE(COLLATION,'<NULL>'),COALESCE(SUB_PART,-1),NULLABLE,INDEX_TYPE,COALESCE(INDEX_COMMENT,''),IS_VISIBLE,COALESCE(EXPRESSION,'<NULL>') FROM information_schema.statistics WHERE table_schema=? AND table_name=? ORDER BY INDEX_NAME,SEQ_IN_INDEX", schema, table)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var a, b, c, d, e, f, g, i, j, k, l string
		if err := rows.Scan(&a, &b, &c, &d, &e, &f, &g, &i, &j, &k, &l); err != nil {
			rows.Close()
			return "", err
		}
		fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n", a, b, c, d, e, f, g, i, j, k, l)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	rows, err = db.QueryContext(ctx, "SELECT COALESCE(PARTITION_NAME,'<NULL>'),COALESCE(SUBPARTITION_NAME,'<NULL>'),COALESCE(PARTITION_ORDINAL_POSITION,0),COALESCE(SUBPARTITION_ORDINAL_POSITION,0),COALESCE(PARTITION_METHOD,'<NULL>'),COALESCE(SUBPARTITION_METHOD,'<NULL>'),COALESCE(PARTITION_EXPRESSION,'<NULL>'),COALESCE(SUBPARTITION_EXPRESSION,'<NULL>'),COALESCE(PARTITION_DESCRIPTION,'<NULL>') FROM information_schema.partitions WHERE table_schema=? AND table_name=? ORDER BY PARTITION_ORDINAL_POSITION,SUBPARTITION_ORDINAL_POSITION", schema, table)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var a, b, c, d, e, f, g, i, j string
		if err := rows.Scan(&a, &b, &c, &d, &e, &f, &g, &i, &j); err != nil {
			rows.Close()
			return "", err
		}
		fmt.Fprintf(h, "partition|%s|%s|%s|%s|%s|%s|%s|%s|%s\n", a, b, c, d, e, f, g, i, j)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func assertFrozenTableFingerprints(ctx context.Context, db *sql.DB, c config, state checkpoint) error {
	for _, table := range []string{c.source, c.target, c.old} {
		exists, _, err := tableIdentity(ctx, db, c.schema, table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		auditedHash, err := auditedLogsSchemaHash(ctx, db, c.schema, table)
		if err != nil || auditedHash != state.sourceSchemaHash {
			return fmt.Errorf("table %s audited schema changed got=%s want=%s err=%v", table, auditedHash, state.sourceSchemaHash, err)
		}
		fingerprint, err := schemaFingerprint(ctx, db, c.schema, table)
		if err != nil || fingerprint != state.sourceFingerprint {
			return fmt.Errorf("table %s full schema/index fingerprint changed got=%s want=%s err=%v", table, fingerprint, state.sourceFingerprint, err)
		}
	}
	return nil
}

func tableStateObserver(schema, table string, want bool) ddlObserver {
	return func(ctx context.Context, conn *sql.Conn, _ string) (ddlState, error) {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&count); err != nil {
			return ddlUnknown, err
		}
		if (count == 1) == want {
			return ddlPost, nil
		}
		return ddlPre, nil
	}
}

func tableCommentObserver(schema, table, marker string) ddlObserver {
	return func(ctx context.Context, conn *sql.Conn, _ string) (ddlState, error) {
		var comment string
		err := conn.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&comment)
		if err == sql.ErrNoRows {
			return ddlPre, nil
		}
		if err != nil {
			return ddlUnknown, err
		}
		if comment == marker {
			return ddlPost, nil
		}
		return ddlUnknown, nil
	}
}

func assertOwned(ctx context.Context, db *sql.DB, c config, table string) error {
	var comment string
	err := db.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.tables WHERE table_schema=? AND table_name=?", c.schema, table).Scan(&comment)
	if err != nil {
		return err
	}
	if comment != ownershipMarker(c) {
		return fmt.Errorf("ownership marker mismatch for %s", table)
	}
	return nil
}

type checkpoint struct {
	phase             string
	last, seed        int64
	final, rollback   sql.NullInt64
	generation        uint64
	marker            string
	sourceSchemaHash  string
	sourceFingerprint string
	triggerSQLMode    string
	ddlOperation      sql.NullString
	ddlNonce          sql.NullString
	baselineUpdates   uint64
	baselineDeletes   uint64
}

func loadCheckpoint(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, c config) (checkpoint, error) {
	cp, _ := qualified(c.schema, c.checkpoint)
	var state checkpoint
	err := q.QueryRowContext(ctx, "SELECT marker,source_schema_sha256,source_fingerprint_sha256,phase,last_completed_end_id,seed_cutoff_id,final_cutoff_id,rollback_base_id,generation,trigger_sql_mode,ddl_operation,ddl_nonce,baseline_updates,baseline_deletes FROM "+cp+" WHERE id=1").Scan(&state.marker, &state.sourceSchemaHash, &state.sourceFingerprint, &state.phase, &state.last, &state.seed, &state.final, &state.rollback, &state.generation, &state.triggerSQLMode, &state.ddlOperation, &state.ddlNonce, &state.baselineUpdates, &state.baselineDeletes)
	if err == nil && state.marker != ownershipMarker(c) {
		err = fmt.Errorf("checkpoint ownership marker mismatch")
	}
	return state, err
}

func backfill(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	if err := assertOwned(ctx, db, c, c.target); err != nil {
		return err
	}
	work, _, err := openVerifiedConn(ctx, db, c, "work")
	if err != nil {
		return err
	}
	defer work.Close()
	if _, err = work.ExecContext(ctx, "SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return err
	}
	copySQL, _ := buildCopySQL(c)
	source, _ := qualified(c.schema, c.source)
	target, _ := qualified(c.schema, c.target)
	cpSQL, _ := checkpointCASSQL(c)
	initial, err := loadCheckpoint(ctx, work, c)
	if err != nil {
		return err
	}
	if err := assertRuntimeSafe(ctx, db, c, initial, true); err != nil {
		return err
	}
	if initial.phase != c.phase {
		if c.command != "reconcile" {
			return fmt.Errorf("phase transition requires reconcile command: current=%s requested=%s", initial.phase, c.phase)
		}
		if err := transitionCheckpoint(ctx, work, c, initial, ev); err != nil {
			return err
		}
	}
	for {
		if _, err := verifyPhysicalConn(ctx, work, c, "work-batch"); err != nil {
			return err
		}
		state, err := loadCheckpoint(ctx, work, c)
		if err != nil {
			return err
		}
		upper := c.upperBound
		if upper == 0 {
			if state.phase == "fresh" && state.final.Valid {
				upper = state.final.Int64
			} else {
				upper = state.seed
			}
		}
		var end sql.NullInt64
		err = work.QueryRowContext(ctx, "SELECT MAX(id) FROM (SELECT id FROM "+source+" FORCE INDEX(PRIMARY) WHERE id>? AND id<=? ORDER BY id LIMIT ?) x", state.last, upper, c.batchSize).Scan(&end)
		if err != nil {
			return err
		}
		if !end.Valid {
			return nil
		}
		batchCtx, cancel := context.WithTimeout(ctx, c.statementTimeout)
		tx, err := work.BeginTx(batchCtx, nil)
		if err != nil {
			cancel()
			return err
		}
		if _, err = tx.ExecContext(batchCtx, copySQL, state.last, end.Int64, upper); err != nil {
			_ = tx.Rollback()
			cancel()
			return err
		}
		if err = verifyWindow(batchCtx, tx, c, source, target, state.last, end.Int64); err != nil {
			_ = tx.Rollback()
			cancel()
			return err
		}
		result, err := tx.ExecContext(batchCtx, cpSQL, c.phase, end.Int64, state.seed, state.final, state.generation)
		if err != nil {
			_ = tx.Rollback()
			cancel()
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			_ = tx.Rollback()
			cancel()
			return fmt.Errorf("checkpoint CAS conflict generation=%d", state.generation)
		}
		if err = tx.Commit(); err != nil {
			cancel()
			return err
		}
		cancel()
		if err := ev.emit("checkpoint_committed", map[string]any{"phase": c.phase, "end_id": end.Int64, "generation": state.generation + 1}); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-time.After(c.batchDelay):
		}
	}
}

func transitionCheckpoint(ctx context.Context, conn *sql.Conn, c config, state checkpoint, ev *evidence) error {
	cp, _ := qualified(c.schema, c.checkpoint)
	last, final, requireForward, err := checkpointTransitionPlan(state, c.phase, c.upperBound)
	if err != nil {
		return err
	}
	if requireForward {
		spec, err := expectedTriggerSpec(c, "forward", c.source, state.triggerSQLMode)
		if err != nil {
			return err
		}
		got, exists, err := readTriggerSpec(ctx, conn, c.schema, spec.name)
		if err != nil || !exists || !triggerMatches(got, spec) {
			return fmt.Errorf("gap requires exact owned forward trigger: exists=%t err=%v", exists, err)
		}
	}
	if err := ev.emit("checkpoint_transition_intent", map[string]any{"from": state.phase, "to": c.phase, "generation": state.generation, "reset_last_id": last}); err != nil {
		return err
	}
	result, err := conn.ExecContext(ctx, "UPDATE "+cp+" SET phase=?,last_completed_end_id=?,final_cutoff_id=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND generation=?", c.phase, last, final, state.generation)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return fmt.Errorf("checkpoint transition CAS conflict generation=%d", state.generation)
	}
	return ev.emit("checkpoint_transition_committed", map[string]any{"from": state.phase, "to": c.phase, "generation": state.generation + 1, "last_id": last})
}

func checkpointTransitionPlan(state checkpoint, next string, upper int64) (int64, sql.NullInt64, bool, error) {
	final := state.final
	switch {
	case state.phase == "seed" && next == "gap":
		if state.last < state.seed {
			return 0, final, false, fmt.Errorf("seed phase is incomplete: last=%d seed=%d", state.last, state.seed)
		}
		if upper <= state.seed {
			return 0, final, false, fmt.Errorf("gap upper-bound must exceed seed cutoff")
		}
		return state.seed, sql.NullInt64{Int64: upper, Valid: true}, true, nil
	case state.phase == "gap" && next == "fresh":
		if !state.final.Valid {
			return 0, final, false, fmt.Errorf("fresh requires persisted final cutoff")
		}
		if state.last < state.final.Int64 {
			return 0, final, false, fmt.Errorf("gap phase is incomplete: last=%d final=%d", state.last, state.final.Int64)
		}
		return 0, final, false, nil
	case state.phase == "fresh" && next == "incremental":
		if !state.final.Valid || upper <= state.final.Int64 {
			return 0, final, false, fmt.Errorf("incremental upper-bound must exceed final cutoff")
		}
		if state.last < state.final.Int64 {
			return 0, final, false, fmt.Errorf("fresh phase is incomplete: last=%d final=%d", state.last, state.final.Int64)
		}
		return state.final.Int64, final, false, nil
	default:
		return 0, final, false, fmt.Errorf("illegal checkpoint phase transition %s -> %s", state.phase, next)
	}
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func verifyWindow(ctx context.Context, q queryRower, c config, source, target string, start, end int64) error {
	predicate := retainedPredicate("s", c.channelIDs)
	equal := rowEqualitySQL("s", "d")
	queries := []string{
		"SELECT COUNT(*) FROM " + source + " s LEFT JOIN " + target + " d ON d.id=s.id WHERE s.id>? AND s.id<=? AND " + predicate + " AND d.id IS NULL",
		"SELECT COUNT(*) FROM " + target + " d LEFT JOIN " + source + " s ON s.id=d.id WHERE d.id>? AND d.id<=? AND s.id IS NULL",
		"SELECT COUNT(*) FROM " + source + " s JOIN " + target + " d ON d.id=s.id WHERE s.id>? AND s.id<=? AND " + predicate + " AND NOT (" + equal + ")",
		"SELECT COUNT(*) FROM " + target + " d WHERE d.id>? AND d.id<=? AND NOT (" + retainedPredicate("d", c.channelIDs) + ")",
	}
	for i, query := range queries {
		var count int
		if err := q.QueryRowContext(ctx, query, start, end).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("window verification check %d failed count=%d range=(%d,%d]", i, count, start, end)
		}
	}
	return nil
}

func installForward(ctx context.Context, db *sql.DB, c config) error {
	if c.triggerDefiner == "" {
		return fmt.Errorf("trigger-definer is required")
	}
	if err := assertOwned(ctx, db, c, c.target); err != nil {
		return err
	}
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	if err := assertRuntimeSafe(ctx, db, c, state, true); err != nil {
		return err
	}
	for _, event := range []string{"update", "delete"} {
		query, _ := buildGuardTriggerSQL(c, event, c.source)
		spec, err := expectedTriggerSpec(c, "guard_"+event, c.source, state.triggerSQLMode)
		if err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, query, exactTriggerObserver(spec, true)); err != nil {
			return err
		}
		futureKind := "future_guard_" + event
		futureQuery, err := buildNamedGuardTriggerSQL(c, futureKind, event, c.target)
		if err != nil {
			return err
		}
		futureSpec, err := expectedTriggerSpec(c, futureKind, c.target, state.triggerSQLMode)
		if err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, futureQuery, exactTriggerObserver(futureSpec, true)); err != nil {
			return err
		}
	}
	query, err := buildForwardTriggerSQL(c)
	if err != nil {
		return err
	}
	spec, err := expectedTriggerSpec(c, "forward", c.source, state.triggerSQLMode)
	if err != nil {
		return err
	}
	return runDDL(ctx, db, c, query, exactTriggerObserver(spec, true))
}

type triggerSpec struct {
	schema, name, table, timing, event, action, definer, sqlMode string
}

func expectedTriggerSpec(c config, kind, table, sqlMode string) (triggerSpec, error) {
	name, err := triggerName(kind, c.batch)
	if err != nil {
		return triggerSpec{}, err
	}
	var statement, timing, event string
	switch kind {
	case "forward":
		statement, err = buildForwardTriggerSQL(c)
		timing, event = "AFTER", "INSERT"
	case "reverse":
		statement, err = buildStrictMirrorTriggerSQL(c, c.source, c.old)
		timing, event = "AFTER", "INSERT"
	case "guard_update":
		statement, err = buildGuardTriggerSQL(c, "update", table)
		timing, event = "BEFORE", "UPDATE"
	case "guard_delete":
		statement, err = buildGuardTriggerSQL(c, "delete", table)
		timing, event = "BEFORE", "DELETE"
	case "future_guard_update":
		statement, err = buildNamedGuardTriggerSQL(c, kind, "update", table)
		timing, event = "BEFORE", "UPDATE"
	case "future_guard_delete":
		statement, err = buildNamedGuardTriggerSQL(c, kind, "delete", table)
		timing, event = "BEFORE", "DELETE"
	default:
		return triggerSpec{}, fmt.Errorf("unsupported owned trigger kind %q", kind)
	}
	if err != nil {
		return triggerSpec{}, err
	}
	marker := "FOR EACH ROW "
	index := strings.Index(statement, marker)
	if index < 0 {
		return triggerSpec{}, fmt.Errorf("owned trigger SQL lacks action boundary")
	}
	return triggerSpec{schema: c.schema, name: name, table: table, timing: timing, event: event, action: statement[index+len(marker):], definer: c.triggerDefiner, sqlMode: sqlMode}, nil
}

func exactTriggerObserver(spec triggerSpec, want bool) ddlObserver {
	return func(ctx context.Context, conn *sql.Conn, ddlSQLMode string) (ddlState, error) {
		got, exists, err := readTriggerSpec(ctx, conn, spec.schema, spec.name)
		if err != nil {
			return ddlUnknown, err
		}
		if !exists {
			if want {
				return ddlPre, nil
			}
			return ddlPost, nil
		}
		if !want {
			return ddlPre, nil
		}
		if ddlSQLMode != spec.sqlMode || got.schema != spec.schema || got.name != spec.name || got.table != spec.table || got.timing != spec.timing || got.event != spec.event || got.definer != spec.definer || got.sqlMode != spec.sqlMode || !sameDDL(got.action, spec.action) {
			return ddlUnknown, nil
		}
		return ddlPost, nil
	}
}

func readTriggerSpec(ctx context.Context, q queryRower, schema, name string) (triggerSpec, bool, error) {
	var got triggerSpec
	err := q.QueryRowContext(ctx, "SELECT TRIGGER_SCHEMA,TRIGGER_NAME,EVENT_OBJECT_TABLE,ACTION_TIMING,EVENT_MANIPULATION,ACTION_STATEMENT,DEFINER,SQL_MODE FROM information_schema.triggers WHERE trigger_schema=? AND trigger_name=?", schema, name).Scan(&got.schema, &got.name, &got.table, &got.timing, &got.event, &got.action, &got.definer, &got.sqlMode)
	if err == sql.ErrNoRows {
		return got, false, nil
	}
	return got, err == nil, err
}

func triggerMatches(got, want triggerSpec) bool {
	return got.schema == want.schema && got.name == want.name && got.table == want.table && got.timing == want.timing && got.event == want.event && got.definer == want.definer && got.sqlMode == want.sqlMode && sameDDL(got.action, want.action)
}

func observeTopology(ctx context.Context, db *sql.DB, c config) (objectTopology, triggerTopology, error) {
	var o objectTopology
	for name, dst := range map[string]*bool{c.source: &o.source, c.target: &o.target, c.old: &o.old} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", c.schema, name).Scan(&n); err != nil {
			return o, triggerTopology{}, err
		}
		*dst = n == 1
	}
	var t triggerTopology
	checkpoint, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return o, t, err
	}
	status := classifyTopology(o)
	for kind, dst := range map[string]*bool{
		"forward": &t.forward, "reverse": &t.reverse,
		"guard_update": &t.updateGuard, "guard_delete": &t.deleteGuard,
		"future_guard_update": &t.futureUpdateGuard, "future_guard_delete": &t.futureDeleteGuard,
	} {
		name, _ := triggerName(kind, c.batch)
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=? AND trigger_name=?", c.schema, name).Scan(&n); err != nil {
			return o, t, err
		}
		*dst = n == 1
		if n == 1 {
			got, exists, observeErr := readTriggerSpec(ctx, db, c.schema, name)
			if observeErr != nil || !exists {
				return o, t, fmt.Errorf("trigger %s exists but exact ownership is not proven: err=%v", name, observeErr)
			}
			table := c.source
			switch {
			case status == topologyPostCutover && kind == "forward":
				table = c.old
			case status == topologyPostCutover && (kind == "future_guard_update" || kind == "future_guard_delete"):
				table = c.source
			case status == topologyPreCutover && (kind == "future_guard_update" || kind == "future_guard_delete"):
				table = c.target
			case status == topologyPostCutover && (kind == "guard_update" || kind == "guard_delete"):
				if got.table != c.source && got.table != c.old {
					return o, t, fmt.Errorf("POST guard %s is attached to unexpected table %s", name, got.table)
				}
				table = got.table
			case status == topologyPreCutover && (checkpoint.phase == "rollback-intent" || checkpoint.phase == "rollback-reconcile") && (kind == "guard_update" || kind == "guard_delete"):
				if got.table != c.source && got.table != c.target {
					return o, t, fmt.Errorf("rollback PRE guard %s is attached to unexpected table %s", name, got.table)
				}
				table = got.table
			}
			spec, specErr := expectedTriggerSpec(c, kind, table, checkpoint.triggerSQLMode)
			if specErr != nil {
				return o, t, specErr
			}
			if status == topologyPreCutover && (checkpoint.phase == "rollback-intent" || checkpoint.phase == "rollback-reconcile") && kind == "reverse" {
				// RENAME moves the trigger with its source table but does not rewrite
				// the stored action text. It is inactive on the compact target and is
				// dropped before the new forward trigger is installed.
				spec.table = c.target
			}
			if !triggerMatches(got, spec) {
				return o, t, fmt.Errorf("trigger %s exists but exact ownership is not proven", name)
			}
			switch kind {
			case "guard_update":
				t.updateGuardTable = table
			case "guard_delete":
				t.deleteGuardTable = table
			case "future_guard_update":
				t.futureUpdateGuardTable = table
			case "future_guard_delete":
				t.futureDeleteGuardTable = table
			}
		}
	}
	return o, t, nil
}

func verify(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	wantTopology := classifyForVerify(state)
	if wantTopology == topologyUnknown {
		return fmt.Errorf("verify refuses transitional checkpoint phase %s; run recover", state.phase)
	}
	objects, _, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	if got := classifyTopology(objects); got != wantTopology {
		return fmt.Errorf("checkpoint expects %s topology, observed %s", wantTopology, got)
	}
	if err := assertRuntimeSafe(ctx, db, c, state, wantTopology == topologyPreCutover); err != nil {
		return err
	}
	if wantTopology == topologyPostCutover {
		// After the atomic RENAME, the owned shadow table becomes the live source;
		// the original unowned source becomes the rollback table.
		if err := assertOwned(ctx, db, c, ownedTableForTopology(c, wantTopology)); err != nil {
			return err
		}
		if err := fullPostVerify(ctx, db, c); err != nil {
			return err
		}
		return ev.emit("verify_passed", map[string]any{"topology": wantTopology})
	}
	if err := assertOwned(ctx, db, c, ownedTableForTopology(c, wantTopology)); err != nil {
		return err
	}
	source, _ := qualified(c.schema, c.source)
	target, _ := qualified(c.schema, c.target)
	upper := c.upperBound
	if upper == 0 {
		upper = state.last
	}
	start := int64(0)
	for start < upper {
		var end sql.NullInt64
		if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM (SELECT id FROM "+source+" WHERE id>? AND id<=? ORDER BY id LIMIT ?) x", start, upper, c.batchSize).Scan(&end); err != nil {
			return err
		}
		if !end.Valid {
			break
		}
		if err := verifyWindow(ctx, db, c, source, target, start, end.Int64); err != nil {
			return err
		}
		start = end.Int64
	}
	return ev.emit("verify_passed", map[string]any{"topology": wantTopology, "upper_bound": upper})
}

func classifyForVerify(state checkpoint) topologyStatus {
	if state.phase == "rollback-gap" || state.phase == "rollback-ready" {
		return topologyPostCutover
	}
	if state.phase == "ddl-intent" || state.phase == "rollback-intent" || state.phase == "rollback-reconcile" {
		return topologyUnknown
	}
	return topologyPreCutover
}

func ownedTableForTopology(c config, status topologyStatus) string {
	if status == topologyPostCutover {
		return c.source
	}
	return c.target
}

func recover(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	return recoverCutover(ctx, db, c, ev)
}

func cleanup(ctx context.Context, db *sql.DB, c config) error {
	o, triggers, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	marker := ownershipMarker(c)
	if c.confirmCleanup != marker {
		return fmt.Errorf("confirm-cleanup must exactly equal ownership marker %q", marker)
	}
	if err := assertOwned(ctx, db, c, c.target); err != nil {
		return err
	}
	plan, err := cleanupPlan(classifyTopology(o), triggers, true)
	if err != nil {
		return err
	}
	_ = plan
	for _, kind := range []string{"forward", "guard_update", "guard_delete", "future_guard_update", "future_guard_delete"} {
		name, _ := triggerName(kind, c.batch)
		state, err := loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
		table := c.source
		if kind == "future_guard_update" || kind == "future_guard_delete" {
			table = c.target
		}
		spec, err := expectedTriggerSpec(c, kind, table, state.triggerSQLMode)
		if err != nil {
			return err
		}
		got, exists, err := readTriggerSpec(ctx, db, c.schema, name)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if !triggerMatches(got, spec) {
			return fmt.Errorf("refusing to drop non-owned trigger %s", name)
		}
		qname, _ := quoteIdentifier(name)
		statement := "DROP TRIGGER " + qname
		if err := runDDL(ctx, db, c, statement, exactTriggerObserver(spec, false)); err != nil {
			return err
		}
	}
	for _, table := range []string{c.target, c.checkpoint} {
		if err := assertOwned(ctx, db, c, table); err != nil {
			return err
		}
		qt, _ := qualified(c.schema, table)
		if err := runDDL(ctx, db, c, "DROP TABLE "+qt, tableStateObserver(c.schema, table, false)); err != nil {
			return err
		}
	}
	return nil
}
