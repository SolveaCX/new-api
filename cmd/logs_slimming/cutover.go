package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const cutoverOperation = "RENAME_TABLE"
const rollbackOperation = "ROLLBACK_RENAME_TABLE"

var autoIncrementPattern = regexp.MustCompile(`(?i)\bAUTO_INCREMENT=(\d+)\b`)

func cutover(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	state, err := assertCutoverPreconditions(ctx, db, c)
	if err != nil {
		return err
	}
	sourceNext, err := showCreateAutoIncrement(ctx, db, c.schema, c.source)
	if err != nil {
		return err
	}
	targetNext, err := plannedTargetAutoIncrement(sourceNext, c.autoIncrementReserve)
	if err != nil {
		return err
	}
	if err := setTargetAutoIncrement(ctx, db, c, targetNext); err != nil {
		return err
	}
	nonce, err := ddlNonce()
	if err != nil {
		return err
	}
	state, err = persistDDLIntent(ctx, db, c, state, nonce)
	if err != nil {
		return err
	}
	status, resolved, renameErr := renameWithMDLBarrier(ctx, db, c, ev, nonce, cutoverOperation, renameStatement(c), c.target)
	switch status {
	case topologyPreCutover:
		if !resolved {
			return fmt.Errorf("rename remained PRE but execution is unresolved; persisted DDL intent retained: %w", renameErr)
		}
		clearErr := clearPreCutoverIntent(ctx, db, c, state)
		if renameErr != nil {
			return fmt.Errorf("rename remained PRE: %w (intent cleanup: %v)", renameErr, clearErr)
		}
		if clearErr != nil {
			return clearErr
		}
		return fmt.Errorf("rename remained PRE without a driver error")
	case topologyPostCutover:
		if err := ev.emit("cutover_topology_post", map[string]any{"nonce": nonce, "rename_error": renameErr}); err != nil {
			return err
		}
		return stabilizePostCutover(ctx, db, c, ev)
	default:
		return fmt.Errorf("rename outcome UNKNOWN; manual topology diagnosis required: %w", renameErr)
	}
}

// rollback atomically restores the original full logs table as the live table.
// The compact table remains owned and receives retained rows through the
// forward trigger, so a later cutover can be retried without rebuilding it.
func rollback(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	if err := assertRollbackReady(ctx, db, c, state); err != nil {
		return err
	}
	if err := assertRuntimeSafe(ctx, db, c, state, false); err != nil {
		return err
	}
	if err := fullPostVerify(ctx, db, c); err != nil {
		return err
	}
	sourceNext, err := showCreateAutoIncrement(ctx, db, c.schema, c.source)
	if err != nil {
		return err
	}
	oldNext, err := plannedTargetAutoIncrement(sourceNext, c.autoIncrementReserve)
	if err != nil {
		return err
	}
	if err := setTableAutoIncrement(ctx, db, c, c.old, oldNext); err != nil {
		return err
	}
	source, _ := qualified(c.schema, c.source)
	var base sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+source).Scan(&base); err != nil {
		return err
	}
	nonce, err := ddlNonce()
	if err != nil {
		return err
	}
	state, err = persistRollbackIntent(ctx, db, c, state, nonce, base.Int64)
	if err != nil {
		return err
	}
	status, resolved, renameErr := renameWithMDLBarrier(ctx, db, c, ev, nonce, rollbackOperation, rollbackStatement(c), c.old)
	switch status {
	case topologyPostCutover:
		if !resolved {
			return fmt.Errorf("rollback RENAME remained POST but execution is unresolved; persisted DDL intent retained: %w", renameErr)
		}
		clearErr := clearPostCutoverRollbackIntent(ctx, db, c, state)
		if renameErr != nil {
			return fmt.Errorf("rollback RENAME remained POST: %w (intent cleanup: %v)", renameErr, clearErr)
		}
		if clearErr != nil {
			return clearErr
		}
		return fmt.Errorf("rollback RENAME remained POST without a driver error")
	case topologyPreCutover:
		if err := ev.emit("rollback_topology_pre", map[string]any{"nonce": nonce, "rename_error": renameErr}); err != nil {
			return err
		}
		return stabilizePreRollback(ctx, db, c, ev)
	default:
		return fmt.Errorf("rollback RENAME outcome UNKNOWN; persisted intent retained: %w", renameErr)
	}
}

func rollbackStatement(c config) string {
	source, _ := qualified(c.schema, c.source)
	target, _ := qualified(c.schema, c.target)
	old, _ := qualified(c.schema, c.old)
	return "RENAME TABLE " + source + " TO " + target + ", " + old + " TO " + source
}

func persistRollbackIntent(ctx context.Context, db *sql.DB, c config, state checkpoint, nonce string, base int64) (checkpoint, error) {
	cp, _ := qualified(c.schema, c.checkpoint)
	result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='rollback-intent',last_completed_end_id=?,rollback_base_id=?,ddl_operation=?,ddl_nonce=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-ready' AND generation=? AND ddl_operation IS NULL AND ddl_nonce IS NULL", base, base, rollbackOperation, nonce, state.generation)
	if err != nil {
		return state, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return state, fmt.Errorf("rollback intent CAS conflict generation=%d", state.generation)
	}
	state.phase = "rollback-intent"
	state.last = base
	state.rollback = sql.NullInt64{Int64: base, Valid: true}
	state.generation++
	state.ddlOperation = sql.NullString{String: rollbackOperation, Valid: true}
	state.ddlNonce = sql.NullString{String: nonce, Valid: true}
	return state, nil
}

func clearPostCutoverRollbackIntent(ctx context.Context, db *sql.DB, c config, state checkpoint) error {
	cp, _ := qualified(c.schema, c.checkpoint)
	result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='rollback-ready',ddl_operation=NULL,ddl_nonce=NULL,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-intent' AND generation=? AND ddl_operation=? AND ddl_nonce=?", state.generation, rollbackOperation, state.ddlNonce.String)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return fmt.Errorf("clear rollback POST intent CAS conflict generation=%d", state.generation)
	}
	return nil
}

func plannedTargetAutoIncrement(sourceNext, reserve uint64) (uint64, error) {
	if reserve > math.MaxUint64/2 {
		return 0, fmt.Errorf("AUTO_INCREMENT double reserve overflow")
	}
	return targetAutoIncrement(sourceNext, 2*reserve)
}

func assertCutoverPreconditions(ctx context.Context, db *sql.DB, c config) (checkpoint, error) {
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return state, err
	}
	if err := cutoverCheckpointReady(state); err != nil {
		return state, fmt.Errorf("cutover requires fresh completed checkpoint with no DDL intent: phase=%s last=%d final=%v", state.phase, state.last, state.final)
	}
	if err := assertOwned(ctx, db, c, c.target); err != nil {
		return state, err
	}
	if err := assertFrozenTableFingerprints(ctx, db, c, state); err != nil {
		return state, err
	}
	o, triggers, err := observeTopology(ctx, db, c)
	if err != nil {
		return state, err
	}
	if classifyTopology(o) != topologyPreCutover || !triggers.forward || triggers.reverse || !preGuardsReady(triggers, c) {
		return state, fmt.Errorf("cutover trigger/topology precondition failed objects=%+v triggers=%+v", o, triggers)
	}
	if err := assertRuntimeSafe(ctx, db, c, state, true); err != nil {
		return state, err
	}
	return state, nil
}

func cutoverCheckpointReady(state checkpoint) error {
	if state.phase != "fresh" || !state.final.Valid || state.last < state.final.Int64 || state.ddlOperation.Valid || state.ddlNonce.Valid {
		return fmt.Errorf("checkpoint is not cutover-ready")
	}
	return nil
}

func persistDDLIntent(ctx context.Context, db *sql.DB, c config, state checkpoint, nonce string) (checkpoint, error) {
	cp, _ := qualified(c.schema, c.checkpoint)
	result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='ddl-intent',ddl_operation=?,ddl_nonce=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='fresh' AND generation=? AND ddl_operation IS NULL AND ddl_nonce IS NULL", cutoverOperation, nonce, state.generation)
	if err != nil {
		return state, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return state, fmt.Errorf("DDL intent CAS conflict generation=%d", state.generation)
	}
	state.phase, state.generation = "ddl-intent", state.generation+1
	state.ddlOperation = sql.NullString{String: cutoverOperation, Valid: true}
	state.ddlNonce = sql.NullString{String: nonce, Valid: true}
	return state, nil
}

func clearPreCutoverIntent(ctx context.Context, db *sql.DB, c config, state checkpoint) error {
	cp, _ := qualified(c.schema, c.checkpoint)
	result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='fresh',ddl_operation=NULL,ddl_nonce=NULL,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='ddl-intent' AND generation=? AND ddl_operation=? AND ddl_nonce=?", state.generation, state.ddlOperation.String, state.ddlNonce.String)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return fmt.Errorf("clear PRE intent CAS conflict generation=%d", state.generation)
	}
	return nil
}

func setTargetAutoIncrement(ctx context.Context, db *sql.DB, c config, next uint64) error {
	return setTableAutoIncrement(ctx, db, c, c.target, next)
}

func setTableAutoIncrement(ctx context.Context, db *sql.DB, c config, table string, next uint64) error {
	qualifiedTable, _ := qualified(c.schema, table)
	statement := fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT=%d", qualifiedTable, next)
	observer := func(ctx context.Context, conn *sql.Conn, _ string) (ddlState, error) {
		got, err := showCreateAutoIncrement(ctx, conn, c.schema, table)
		if err != nil {
			return ddlUnknown, err
		}
		if got >= next {
			return ddlPost, nil
		}
		return ddlPre, nil
	}
	if err := runDDL(ctx, db, c, statement, observer); err != nil {
		return err
	}
	got, err := showCreateAutoIncrement(ctx, db, c.schema, table)
	if err != nil || got < next {
		return fmt.Errorf("SHOW CREATE AUTO_INCREMENT readback failed got=%d want>=%d err=%v", got, next, err)
	}
	return nil
}

type showCreateQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func showCreateAutoIncrement(ctx context.Context, q showCreateQueryer, schema, table string) (uint64, error) {
	qualifiedTable, err := qualified(schema, table)
	if err != nil {
		return 0, err
	}
	var name, create string
	if err := q.QueryRowContext(ctx, "SHOW CREATE TABLE "+qualifiedTable).Scan(&name, &create); err != nil {
		return 0, err
	}
	match := autoIncrementPattern.FindStringSubmatch(create)
	if len(match) == 2 {
		return strconv.ParseUint(match[1], 10, 64)
	}
	var next sql.NullInt64
	if err := q.QueryRowContext(ctx, "SELECT AUTO_INCREMENT FROM information_schema.tables WHERE table_schema=? AND table_name=?", schema, table).Scan(&next); err != nil {
		return 0, err
	}
	if next.Valid && next.Int64 > 0 {
		return uint64(next.Int64), nil
	}
	var maxID uint64
	if err := q.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM "+qualifiedTable).Scan(&maxID); err != nil {
		return 0, err
	}
	return safeNext(maxID)
}

func renameStatement(c config) string {
	source, _ := qualified(c.schema, c.source)
	target, _ := qualified(c.schema, c.target)
	old, _ := qualified(c.schema, c.old)
	return "RENAME TABLE " + source + " TO " + old + ", " + target + " TO " + source
}

// renameWithMDLBarrier uses ordinary reads in a short READ COMMITTED transaction.
// Their shared metadata locks block only the RENAME; LOCK TABLES is deliberately
// not used. Once the exact RENAME is the sole pending/granted foreign MDL holder,
// the final ID/AUTO_INCREMENT invariant is checked and COMMIT releases the barrier.
func renameWithMDLBarrier(ctx context.Context, db *sql.DB, c config, ev *evidence, nonce, operation, statement, futureActive string) (topologyStatus, bool, error) {
	deadline := time.Now().Add(c.ddlTimeout)
	opCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	barrier, barrierFacts, err := openVerifiedConn(opCtx, db, c, "cutover-barrier")
	if err != nil {
		return topologyUnknown, true, err
	}
	defer barrier.Close()
	tx, err := barrier.BeginTx(opCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		return topologyUnknown, true, err
	}
	barrierOpen := true
	defer func() {
		if barrierOpen {
			_ = tx.Rollback()
		}
	}()
	source, _ := qualified(c.schema, c.source)
	future, _ := qualified(c.schema, futureActive)
	for _, table := range []string{source, future} {
		if _, err := tx.ExecContext(opCtx, "SELECT id FROM "+table+" LIMIT 0"); err != nil {
			return topologyUnknown, true, fmt.Errorf("acquire shared MDL barrier on %s: %w", table, err)
		}
	}
	ddlConn, ddlFacts, err := openVerifiedConn(opCtx, db, c, "cutover-rename")
	if err != nil {
		return topologyUnknown, true, err
	}
	ddlStarted := false
	defer func() {
		if !ddlStarted {
			_ = ddlConn.Close()
		}
	}()
	control, _, err := openVerifiedConn(opCtx, db, c, "cutover-control")
	if err != nil {
		_ = ddlConn.Close()
		return topologyUnknown, true, err
	}
	defer control.Close()
	if _, err := ddlConn.ExecContext(opCtx, fmt.Sprintf("SET SESSION lock_wait_timeout=%d", c.lockWaitSeconds)); err != nil {
		_ = ddlConn.Close()
		return topologyUnknown, true, err
	}
	tagged := fmt.Sprintf("/*logs_slim batch=%s operation=%s nonce=%s*/ %s", c.batch, operation, nonce, statement)
	statementHash := fmt.Sprintf("%x", sha256.Sum256([]byte(statement)))
	emitPostcondition := func(result string, status topologyStatus, resolved bool, execErr, observeErr error) error {
		return ev.emit("rename_postcondition", map[string]any{
			"operation": operation, "nonce": nonce, "statement_sha256": statementHash,
			"result": result, "topology": status, "exec_resolved": resolved,
			"exec_error": execErr, "observe_error": observeErr,
		})
	}
	if err := ev.emit("rename_barrier_intent", map[string]any{"nonce": nonce, "connection_id": ddlFacts.connectionID, "statement_sha256": statementHash}); err != nil {
		return topologyUnknown, true, err
	}
	done := make(chan error, 1)
	ddlStarted = true
	go func() {
		_, execErr := ddlConn.ExecContext(opCtx, tagged)
		done <- execErr
	}()
	abort := func(cause error) (topologyStatus, bool, error) {
		killCtx, stop := boundedBackground(deadline, 300*time.Millisecond)
		killErr := killExactDDL(killCtx, control, c, ctx, ddlFacts, tagged)
		stop()
		cancel()
		rollbackErr := tx.Rollback()
		barrierOpen = false
		resolved := false
		select {
		case <-done:
			resolved = true
		case <-time.After(minDuration(ddlSettleGrace, positiveRemaining(deadline))):
		}
		if resolved {
			_ = ddlConn.Close()
		} else {
			go reapDDLConn(done, ddlConn)
		}
		status, topologyErr := observeStableObjectTopology(db, c, deadline)
		result := "known"
		if topologyErr != nil || status == topologyUnknown || !resolved {
			result = "unknown"
		}
		if evidenceErr := emitPostcondition(result, status, resolved, cause, topologyErr); evidenceErr != nil {
			return topologyUnknown, resolved, fmt.Errorf("%w; rename postcondition evidence: %v", cause, evidenceErr)
		}
		return status, resolved, fmt.Errorf("%w; exact_kill=%v barrier_rollback=%v topology=%v", cause, killErr, rollbackErr, topologyErr)
	}
	pendingDeadline := deadline.Add(-600 * time.Millisecond)
	if pendingDeadline.Before(time.Now()) {
		pendingDeadline = time.Now().Add(50 * time.Millisecond)
	}
	pendingCtx, stopPending := context.WithDeadline(opCtx, pendingDeadline)
	err = waitForExactPendingRename(pendingCtx, control, c, barrierFacts, ddlFacts, tagged)
	stopPending()
	if err != nil {
		return abort(fmt.Errorf("pending RENAME proof failed: %w", err))
	}
	var sourceMax, futureMax uint64
	if err := tx.QueryRowContext(opCtx, "SELECT COALESCE(MAX(id),0) FROM "+source).Scan(&sourceMax); err != nil {
		return abort(fmt.Errorf("final source MAX: %w", err))
	}
	if err := tx.QueryRowContext(opCtx, "SELECT COALESCE(MAX(id),0) FROM "+future).Scan(&futureMax); err != nil {
		return abort(fmt.Errorf("final future-active MAX: %w", err))
	}
	futureNext, err := showCreateAutoIncrement(opCtx, tx, c.schema, futureActive)
	if err != nil {
		return abort(fmt.Errorf("final future-active SHOW CREATE: %w", err))
	}
	sourceFinalNext, err := safeNext(sourceMax)
	if err != nil {
		return abort(err)
	}
	required, err := targetAutoIncrement(sourceFinalNext, c.autoIncrementReserve)
	if err != nil || futureNext < required || futureNext <= futureMax {
		return abort(fmt.Errorf("final AUTO_INCREMENT invariant failed source_max=%d future_max=%d future_next=%d required=%d err=%v", sourceMax, futureMax, futureNext, required, err))
	}
	if err := tx.Commit(); err != nil {
		return abort(fmt.Errorf("release MDL barrier: %w", err))
	}
	barrierOpen = false
	var execErr error
	resolved := false
	select {
	case execErr = <-done:
		resolved = true
		_ = ddlConn.Close()
	case <-opCtx.Done():
		killCtx, stop := boundedBackground(deadline, 250*time.Millisecond)
		killErr := killExactDDL(killCtx, control, c, ctx, ddlFacts, tagged)
		stop()
		execErr = fmt.Errorf("rename watchdog expired: %w; kill=%v", context.Cause(opCtx), killErr)
		select {
		case lateErr := <-done:
			resolved = true
			if execErr == nil {
				execErr = lateErr
			}
			_ = ddlConn.Close()
		case <-time.After(minDuration(ddlSettleGrace, positiveRemaining(deadline))):
			go reapDDLConn(done, ddlConn)
		}
	}
	status, topologyErr := observeStableObjectTopology(db, c, deadline)
	if topologyErr != nil {
		if evidenceErr := emitPostcondition("unknown", topologyUnknown, resolved, execErr, topologyErr); evidenceErr != nil {
			return topologyUnknown, resolved, fmt.Errorf("rename=%v topology=%v evidence=%w", execErr, topologyErr, evidenceErr)
		}
		return topologyUnknown, resolved, fmt.Errorf("rename=%v topology=%w", execErr, topologyErr)
	}
	if evidenceErr := emitPostcondition("known", status, resolved, execErr, nil); evidenceErr != nil {
		return topologyUnknown, resolved, fmt.Errorf("write rename postcondition evidence: %w", evidenceErr)
	}
	return status, resolved, execErr
}

func waitForExactPendingRename(ctx context.Context, control *sql.Conn, c config, barrierFacts, ddlFacts sessionFacts, tagged string) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := assertExactPendingRename(ctx, control, c, barrierFacts, ddlFacts, tagged); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
		}
	}
}

func assertExactPendingRename(ctx context.Context, control *sql.Conn, c config, barrierFacts, ddlFacts sessionFacts, tagged string) error {
	var database sql.NullString
	var user, command string
	var info sql.NullString
	if err := control.QueryRowContext(ctx, "SELECT DB,USER,COMMAND,INFO FROM information_schema.PROCESSLIST WHERE ID=?", ddlFacts.connectionID).Scan(&database, &user, &command, &info); err != nil {
		return err
	}
	expectedUser := strings.SplitN(ddlFacts.currentUser, "@", 2)[0]
	if !database.Valid || database.String != c.schema || user != expectedUser || command != "Query" || !info.Valid || !sameDDL(info.String, tagged) {
		return fmt.Errorf("pending process identity mismatch")
	}
	rows, err := control.QueryContext(ctx, "SELECT COALESCE(t.PROCESSLIST_ID,0),m.LOCK_STATUS FROM performance_schema.metadata_locks m LEFT JOIN performance_schema.threads t ON t.THREAD_ID=m.OWNER_THREAD_ID WHERE m.OBJECT_TYPE='TABLE' AND m.OBJECT_SCHEMA=? AND m.OBJECT_NAME IN (?,?,?)", c.schema, c.source, c.target, c.old)
	if err != nil {
		return err
	}
	defer rows.Close()
	pendingRename := 0
	for rows.Next() {
		var owner int64
		var status string
		if err := rows.Scan(&owner, &status); err != nil {
			return err
		}
		switch {
		case mdlOwnerAllowed(owner, status, barrierFacts.connectionID, ddlFacts.connectionID):
			if owner == ddlFacts.connectionID && status == "PENDING" {
				pendingRename++
			}
		default:
			return fmt.Errorf("unexpected MDL owner=%d status=%s", owner, status)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if pendingRename < 1 {
		return fmt.Errorf("exact RENAME has no pending table MDL")
	}
	return nil
}

func mdlOwnerAllowed(owner int64, status string, barrierID, ddlID int64) bool {
	return (owner == barrierID && status == "GRANTED") ||
		(owner == ddlID && (status == "GRANTED" || status == "PENDING"))
}

func observeStableObjectTopology(db *sql.DB, c config, deadline time.Time) (topologyStatus, error) {
	states := make([]topologyStatus, 0, 2)
	for i := 0; i < 2; i++ {
		observeCtx, cancel := boundedBackground(deadline, 300*time.Millisecond)
		conn, _, err := openVerifiedConn(observeCtx, db, c, "fresh-topology-observer")
		if err != nil {
			cancel()
			return topologyUnknown, err
		}
		var o objectTopology
		for name, dst := range map[string]*bool{c.source: &o.source, c.target: &o.target, c.old: &o.old} {
			var n int
			if err := conn.QueryRowContext(observeCtx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name=?", c.schema, name).Scan(&n); err != nil {
				_ = conn.Close()
				cancel()
				return topologyUnknown, err
			}
			*dst = n == 1
		}
		_ = conn.Close()
		cancel()
		states = append(states, classifyTopology(o))
		if i == 0 {
			time.Sleep(minDuration(25*time.Millisecond, positiveRemaining(deadline)))
		}
	}
	if states[0] != states[1] || states[0] == topologyUnknown {
		return topologyUnknown, fmt.Errorf("fresh topology observations unstable: %v", states)
	}
	return states[0], nil
}

func recoverCutover(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	deadline := time.Now().Add(maxDuration(c.ddlTimeout, 2*c.statementTimeout))
	status, err := observeStableObjectTopology(db, c, deadline)
	if err != nil {
		return err
	}
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	if err := ev.emit("recovery_diagnosis", map[string]any{"topology": status, "phase": state.phase, "generation": state.generation}); err != nil {
		return err
	}
	switch status {
	case topologyPreCutover:
		if state.phase == "rollback-intent" || state.phase == "rollback-reconcile" {
			if state.phase == "rollback-intent" {
				if !state.ddlNonce.Valid || !state.ddlOperation.Valid || state.ddlOperation.String != rollbackOperation {
					return fmt.Errorf("rollback PRE checkpoint lacks exact persisted RENAME intent")
				}
				if err := assertNoExactDDLInFlight(ctx, db, c, rollbackOperation, state.ddlNonce.String, rollbackStatement(c)); err != nil {
					return err
				}
			}
			return stabilizePreRollback(ctx, db, c, ev)
		}
		if state.phase == "fresh" {
			_, err := assertCutoverPreconditions(ctx, db, c)
			return err
		}
		if state.phase != "ddl-intent" || !state.ddlOperation.Valid || state.ddlOperation.String != cutoverOperation || !state.ddlNonce.Valid {
			return fmt.Errorf("PRE topology has unexpected checkpoint phase=%s", state.phase)
		}
		if err := assertNoExactDDLInFlight(ctx, db, c, cutoverOperation, state.ddlNonce.String, renameStatement(c)); err != nil {
			return err
		}
		if err := assertPreCutoverTriggerTopology(ctx, db, c); err != nil {
			return err
		}
		return clearPreCutoverIntent(ctx, db, c, state)
	case topologyPostCutover:
		if state.phase == "rollback-intent" {
			if !state.ddlNonce.Valid || !state.ddlOperation.Valid || state.ddlOperation.String != rollbackOperation {
				return fmt.Errorf("rollback POST checkpoint lacks exact persisted RENAME intent")
			}
			if err := assertNoExactDDLInFlight(ctx, db, c, rollbackOperation, state.ddlNonce.String, rollbackStatement(c)); err != nil {
				return err
			}
			if err := clearPostCutoverRollbackIntent(ctx, db, c, state); err != nil {
				return err
			}
			state, err = loadCheckpoint(ctx, db, c)
			if err != nil {
				return err
			}
			return assertRollbackReady(ctx, db, c, state)
		}
		return stabilizePostCutover(ctx, db, c, ev)
	default:
		return fmt.Errorf("recovery refuses UNKNOWN topology")
	}
}

func stabilizePreRollback(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	if state.phase != "rollback-intent" && state.phase != "rollback-reconcile" {
		return fmt.Errorf("PRE rollback stabilization requires rollback intent, got %s", state.phase)
	}
	if err := assertOwned(ctx, db, c, c.target); err != nil {
		return fmt.Errorf("restored compact target ownership: %w", err)
	}
	if err := assertFrozenTableFingerprints(ctx, db, c, state); err != nil {
		return err
	}
	if err := ensurePreTriggersAfterRollback(ctx, db, c, state); err != nil {
		return err
	}
	if state.phase == "rollback-intent" {
		source, _ := qualified(c.schema, c.source)
		var upper sql.NullInt64
		if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+source).Scan(&upper); err != nil {
			return err
		}
		cp, _ := qualified(c.schema, c.checkpoint)
		result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='rollback-reconcile',final_cutoff_id=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-intent' AND generation=? AND ddl_operation=? AND ddl_nonce=?", upper.Int64, state.generation, rollbackOperation, state.ddlNonce.String)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return fmt.Errorf("rollback reconcile CAS conflict generation=%d", state.generation)
		}
		state, err = loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
	}
	if err := reconcilePreRollbackGap(ctx, db, c, ev); err != nil {
		return err
	}
	state, err = loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	if !state.final.Valid || state.last < state.final.Int64 {
		return fmt.Errorf("rollback reconcile is incomplete last=%d final=%v", state.last, state.final)
	}
	if err := removeFilteredRollbackRows(ctx, db, c, ev, state.seed); err != nil {
		return err
	}
	cp, _ := qualified(c.schema, c.checkpoint)
	result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='fresh',rollback_base_id=NULL,ddl_operation=NULL,ddl_nonce=NULL,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-reconcile' AND generation=?", state.generation)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return fmt.Errorf("rollback completion CAS conflict generation=%d", state.generation)
	}
	return assertPreCutoverTriggerTopology(ctx, db, c)
}

func removeFilteredRollbackRows(ctx context.Context, db *sql.DB, c config, ev *evidence, verifiedFloor int64) (retErr error) {
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	objects, topology, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	if classifyTopology(objects) != topologyPreCutover || !topology.forward || topology.reverse {
		return fmt.Errorf("rollback filtered cleanup requires stable PRE topology: objects=%+v triggers=%+v", objects, topology)
	}
	dropGuard, err := filteredCleanupGuardPlan(state.phase, topology, c)
	if err != nil {
		return err
	}
	guardSpec, err := expectedTriggerSpec(c, "future_guard_delete", c.target, state.triggerSQLMode)
	if err != nil {
		return err
	}
	if dropGuard {
		name, _ := triggerName("future_guard_delete", c.batch)
		quoted, _ := quoteIdentifier(name)
		if err := runDDL(ctx, db, c, "DROP TRIGGER "+quoted, exactTriggerObserver(guardSpec, false)); err != nil {
			return err
		}
	}
	// The compact target is inactive throughout rollback-reconcile. Its DELETE
	// guard may be absent only during this cleanup, and is always rebuilt before
	// the checkpoint can leave rollback-reconcile. A crash after DROP is handled
	// by the same idempotent path on the next recover.
	defer func() {
		query, err := buildNamedGuardTriggerSQL(c, "future_guard_delete", "delete", c.target)
		if err == nil {
			err = runDDL(ctx, db, c, query, exactTriggerObserver(guardSpec, true))
		}
		if err == nil {
			_, topology, observeErr := observeTopology(ctx, db, c)
			if observeErr != nil {
				err = observeErr
			} else if !preGuardsReady(topology, c) {
				err = fmt.Errorf("future DELETE guard restore did not recover exact pre-cutover guards: %+v", topology)
			}
		}
		if err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("restore future DELETE guard: %w", err))
		}
	}()
	target, _ := qualified(c.schema, c.target)
	var upper sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+target).Scan(&upper); err != nil {
		return err
	}
	for start := verifiedFloor; start < upper.Int64; {
		state, err := loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
		if state.phase != "rollback-reconcile" {
			return fmt.Errorf("filtered-row cleanup requires rollback-reconcile checkpoint")
		}
		if err := assertRuntimeSafe(ctx, db, c, state, true); err != nil {
			return err
		}
		var end sql.NullInt64
		if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM (SELECT id FROM "+target+" FORCE INDEX(PRIMARY) WHERE id>? AND id<=? ORDER BY id LIMIT ?) x", start, upper.Int64, c.batchSize).Scan(&end); err != nil {
			return err
		}
		if !end.Valid {
			return nil
		}
		batchCtx, cancel := context.WithTimeout(ctx, c.statementTimeout)
		tx, err := db.BeginTx(batchCtx, nil)
		if err != nil {
			cancel()
			return err
		}
		result, err := tx.ExecContext(batchCtx, "DELETE FROM "+target+" WHERE id>? AND id<=? AND "+filteredPredicate("", c.channelIDs), start, end.Int64)
		var removed int64
		if err == nil {
			removed, _ = result.RowsAffected()
			var remaining int
			err = tx.QueryRowContext(batchCtx, "SELECT COUNT(*) FROM "+target+" WHERE id>? AND id<=? AND "+filteredPredicate("", c.channelIDs), start, end.Int64).Scan(&remaining)
			if err == nil && remaining != 0 {
				err = fmt.Errorf("filtered compact rows remain count=%d range=(%d,%d]", remaining, start, end.Int64)
			}
		}
		if err != nil {
			_ = tx.Rollback()
			cancel()
			return err
		}
		if err := tx.Commit(); err != nil {
			cancel()
			return err
		}
		cancel()
		if err := ev.emit("rollback_filtered_cleanup", map[string]any{"start_id": start, "end_id": end.Int64, "removed": removed}); err != nil {
			return err
		}
		start = end.Int64
	}
	return nil
}

func filteredCleanupGuardPlan(phase string, topology triggerTopology, c config) (bool, error) {
	if phase != "rollback-reconcile" {
		return false, fmt.Errorf("future DELETE guard may be suspended only during rollback-reconcile")
	}
	if !rollbackCleanupGuardsReady(topology, c) {
		return false, fmt.Errorf("rollback filtered cleanup guard topology is unsafe: %+v", topology)
	}
	return topology.futureDeleteGuard, nil
}

func rollbackCleanupGuardsReady(t triggerTopology, c config) bool {
	return t.updateGuard && t.deleteGuard &&
		t.updateGuardTable == c.source && t.deleteGuardTable == c.source &&
		t.futureUpdateGuard && t.futureUpdateGuardTable == c.target &&
		(t.futureDeleteGuard && t.futureDeleteGuardTable == c.target ||
			!t.futureDeleteGuard && t.futureDeleteGuardTable == "")
}

func ensurePreTriggersAfterRollback(ctx context.Context, db *sql.DB, c config, state checkpoint) error {
	o, t, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	if classifyTopology(o) != topologyPreCutover || t.forward && t.reverse {
		return fmt.Errorf("unsafe rollback PRE trigger topology objects=%+v triggers=%+v", o, t)
	}
	if t.reverse {
		spec, err := expectedTriggerSpec(c, "reverse", c.source, state.triggerSQLMode)
		if err != nil {
			return err
		}
		spec.table = c.target
		name, _ := triggerName("reverse", c.batch)
		quoted, _ := quoteIdentifier(name)
		if err := runDDL(ctx, db, c, "DROP TRIGGER "+quoted, exactTriggerObserver(spec, false)); err != nil {
			return err
		}
	}
	if !t.forward {
		query, err := buildForwardTriggerSQL(c)
		if err != nil {
			return err
		}
		spec, err := expectedTriggerSpec(c, "forward", c.source, state.triggerSQLMode)
		if err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, query, exactTriggerObserver(spec, true)); err != nil {
			return err
		}
	}
	_, final, err := observeTopology(ctx, db, c)
	guardsReady := preGuardsReady(final, c)
	if state.phase == "rollback-reconcile" {
		guardsReady = rollbackCleanupGuardsReady(final, c)
	}
	if err != nil || !final.forward || final.reverse || !guardsReady {
		return fmt.Errorf("rollback PRE trigger stabilization incomplete: triggers=%+v err=%v", final, err)
	}
	return nil
}

func reconcilePreRollbackGap(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	source, _ := qualified(c.schema, c.source)
	target, _ := qualified(c.schema, c.target)
	cp, _ := qualified(c.schema, c.checkpoint)
	copySQL, err := buildCopySQL(c)
	if err != nil {
		return err
	}
	for {
		state, err := loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
		if state.phase != "rollback-reconcile" || !state.final.Valid {
			return fmt.Errorf("rollback reconcile checkpoint is invalid")
		}
		if state.last >= state.final.Int64 {
			return nil
		}
		if err := assertRuntimeSafe(ctx, db, c, state, true); err != nil {
			return err
		}
		var end sql.NullInt64
		if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM (SELECT id FROM "+source+" FORCE INDEX(PRIMARY) WHERE id>? AND id<=? ORDER BY id LIMIT ?) x", state.last, state.final.Int64, c.batchSize).Scan(&end); err != nil {
			return err
		}
		if !end.Valid {
			return nil
		}
		batchCtx, cancel := context.WithTimeout(ctx, c.statementTimeout)
		tx, err := db.BeginTx(batchCtx, nil)
		if err != nil {
			cancel()
			return err
		}
		if _, err = tx.ExecContext(batchCtx, copySQL, state.last, end.Int64, state.final.Int64); err == nil {
			err = verifyWindow(batchCtx, tx, c, source, target, state.last, end.Int64)
		}
		if err == nil {
			var result sql.Result
			result, err = tx.ExecContext(batchCtx, "UPDATE "+cp+" SET last_completed_end_id=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-reconcile' AND generation=?", end.Int64, state.generation)
			if err == nil {
				affected, _ := result.RowsAffected()
				if affected != 1 {
					err = fmt.Errorf("rollback reconcile checkpoint CAS conflict")
				}
			}
		}
		if err != nil {
			_ = tx.Rollback()
			cancel()
			return err
		}
		if err := tx.Commit(); err != nil {
			cancel()
			return err
		}
		cancel()
		if err := ev.emit("rollback_reconcile_checkpoint", map[string]any{"end_id": end.Int64, "upper_bound": state.final.Int64}); err != nil {
			return err
		}
	}
}

func taggedDDL(c config, operation, nonce, statement string) string {
	return fmt.Sprintf("/*logs_slim batch=%s operation=%s nonce=%s*/ %s", c.batch, operation, nonce, statement)
}

func assertNoExactDDLInFlight(ctx context.Context, db *sql.DB, c config, operation, nonce, statement string) error {
	conn, facts, err := openVerifiedConn(ctx, db, c, "recover-process-observer")
	if err != nil {
		return err
	}
	defer conn.Close()
	expectedUser := strings.SplitN(facts.currentUser, "@", 2)[0]
	var count int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=? AND USER=? AND COMMAND='Query' AND INFO=?", c.schema, expectedUser, taggedDDL(c, operation, nonce, statement)).Scan(&count); err != nil {
		return fmt.Errorf("prove no exact DDL remains in flight: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("exact DDL is still in flight; persisted intent retained")
	}
	return nil
}

func assertPreCutoverTriggerTopology(ctx context.Context, db *sql.DB, c config) error {
	o, t, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	if classifyTopology(o) != topologyPreCutover || !t.forward || t.reverse || !preGuardsReady(t, c) {
		return fmt.Errorf("unsafe PRE trigger topology objects=%+v triggers=%+v", o, t)
	}
	return assertOwned(ctx, db, c, c.target)
}

func stabilizePostCutover(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	state, err := loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	if err := assertOwned(ctx, db, c, c.source); err != nil {
		return fmt.Errorf("active POST table ownership: %w", err)
	}
	if err := assertFrozenTableFingerprints(ctx, db, c, state); err != nil {
		return err
	}
	if state.phase == "rollback-ready" {
		return assertRollbackReady(ctx, db, c, state)
	}
	if state.phase == "ddl-intent" {
		if !state.ddlOperation.Valid || state.ddlOperation.String != cutoverOperation || !state.ddlNonce.Valid {
			return fmt.Errorf("POST checkpoint lacks exact persisted RENAME intent")
		}
		old, _ := qualified(c.schema, c.old)
		var base sql.NullInt64
		if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+old).Scan(&base); err != nil {
			return err
		}
		cp, _ := qualified(c.schema, c.checkpoint)
		result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='rollback-gap',rollback_base_id=?,last_completed_end_id=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='ddl-intent' AND generation=? AND ddl_operation=? AND ddl_nonce=?", base.Int64, base.Int64, state.generation, cutoverOperation, state.ddlNonce.String)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return fmt.Errorf("rollback-base CAS conflict generation=%d", state.generation)
		}
		state, err = loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
	}
	if state.phase != "rollback-gap" || !state.rollback.Valid {
		return fmt.Errorf("POST stabilization requires rollback-gap checkpoint, got %s", state.phase)
	}
	if err := ensurePostTriggers(ctx, db, c, state); err != nil {
		return err
	}
	if err := reconcileRollbackGap(ctx, db, c, ev); err != nil {
		return err
	}
	if err := fullPostVerify(ctx, db, c); err != nil {
		return err
	}
	state, err = loadCheckpoint(ctx, db, c)
	if err != nil {
		return err
	}
	cp, _ := qualified(c.schema, c.checkpoint)
	result, err := db.ExecContext(ctx, "UPDATE "+cp+" SET phase='rollback-ready',ddl_operation=NULL,ddl_nonce=NULL,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-gap' AND generation=?", state.generation)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return fmt.Errorf("ROLLBACK_READY CAS conflict generation=%d", state.generation)
	}
	state.phase = "rollback-ready"
	state.ddlOperation = sql.NullString{}
	state.ddlNonce = sql.NullString{}
	return assertRollbackReady(ctx, db, c, state)
}

func ensurePostTriggers(ctx context.Context, db *sql.DB, c config, state checkpoint) error {
	o, t, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	if classifyTopology(o) != topologyPostCutover {
		return fmt.Errorf("unsafe POST trigger topology objects=%+v triggers=%+v", o, t)
	}
	actions, err := postTriggerPlan(t)
	if err != nil {
		return err
	}
	if len(actions) > 0 && actions[0] == postTriggerDropForward {
		spec, _ := expectedTriggerSpec(c, "forward", c.old, state.triggerSQLMode)
		name, _ := triggerName("forward", c.batch)
		qn, _ := quoteIdentifier(name)
		if err := runDDL(ctx, db, c, "DROP TRIGGER "+qn, exactTriggerObserver(spec, false)); err != nil {
			return err
		}
	}
	if len(actions) > 0 {
		_, t, err = observeTopology(ctx, db, c)
		if err != nil || t.forward || t.reverse {
			return fmt.Errorf("forward drop postcondition does not prove zero mirror triggers: triggers=%+v err=%v", t, err)
		}
		query, err := buildStrictMirrorTriggerSQL(c, c.source, c.old)
		if err != nil {
			return err
		}
		spec, err := expectedTriggerSpec(c, "reverse", c.source, state.triggerSQLMode)
		if err != nil {
			return err
		}
		if err := runDDL(ctx, db, c, query, exactTriggerObserver(spec, true)); err != nil {
			return err
		}
	}
	_, final, err := observeTopology(ctx, db, c)
	if err != nil || final.forward || !final.reverse || !postGuardsReady(final, c) {
		return fmt.Errorf("POST trigger stabilization incomplete: triggers=%+v err=%v", final, err)
	}
	return nil
}

func preGuardsReady(t triggerTopology, c config) bool {
	return t.updateGuard && t.deleteGuard &&
		t.updateGuardTable == c.source && t.deleteGuardTable == c.source &&
		t.futureUpdateGuard && t.futureDeleteGuard &&
		t.futureUpdateGuardTable == c.target && t.futureDeleteGuardTable == c.target
}

func postGuardsReady(t triggerTopology, c config) bool {
	return t.updateGuard && t.deleteGuard &&
		t.updateGuardTable == c.old && t.deleteGuardTable == c.old &&
		t.futureUpdateGuard && t.futureDeleteGuard &&
		t.futureUpdateGuardTable == c.source && t.futureDeleteGuardTable == c.source
}

type postTriggerAction string

const (
	postTriggerDropForward   postTriggerAction = "drop-forward"
	postTriggerCreateReverse postTriggerAction = "create-reverse"
)

func postTriggerPlan(t triggerTopology) ([]postTriggerAction, error) {
	if t.forward && t.reverse {
		return nil, fmt.Errorf("forward and reverse triggers must never coexist")
	}
	if t.reverse {
		return nil, nil
	}
	if t.forward {
		return []postTriggerAction{postTriggerDropForward, postTriggerCreateReverse}, nil
	}
	return []postTriggerAction{postTriggerCreateReverse}, nil
}

func reconcileRollbackGap(ctx context.Context, db *sql.DB, c config, ev *evidence) error {
	source, _ := qualified(c.schema, c.source)
	old, _ := qualified(c.schema, c.old)
	cp, _ := qualified(c.schema, c.checkpoint)
	copySQL, err := buildMirrorCopySQL(c, c.source, c.old)
	if err != nil {
		return err
	}
	var upper sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+source).Scan(&upper); err != nil {
		return err
	}
	for {
		state, err := loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
		if state.last >= upper.Int64 {
			return nil
		}
		var end sql.NullInt64
		if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM (SELECT id FROM "+source+" WHERE id>? AND id<=? ORDER BY id LIMIT ?) x", state.last, upper.Int64, c.batchSize).Scan(&end); err != nil {
			return err
		}
		if !end.Valid {
			return nil
		}
		batchCtx, cancel := context.WithTimeout(ctx, c.statementTimeout)
		tx, err := db.BeginTx(batchCtx, nil)
		if err != nil {
			cancel()
			return err
		}
		if _, err = tx.ExecContext(batchCtx, copySQL, state.last, end.Int64); err == nil {
			err = verifyMirrorWindow(batchCtx, tx, source, old, state.last, end.Int64)
		}
		if err == nil {
			var result sql.Result
			result, err = tx.ExecContext(batchCtx, "UPDATE "+cp+" SET last_completed_end_id=?,generation=generation+1,updated_at=CURRENT_TIMESTAMP(6) WHERE id=1 AND phase='rollback-gap' AND generation=?", end.Int64, state.generation)
			if err == nil {
				affected, _ := result.RowsAffected()
				if affected != 1 {
					err = fmt.Errorf("rollback-gap checkpoint CAS conflict")
				}
			}
		}
		if err != nil {
			_ = tx.Rollback()
			cancel()
			return err
		}
		if err := tx.Commit(); err != nil {
			cancel()
			return err
		}
		cancel()
		if err := ev.emit("rollback_gap_checkpoint", map[string]any{"end_id": end.Int64, "upper_bound": upper.Int64}); err != nil {
			return err
		}
	}
}

func verifyMirrorWindow(ctx context.Context, q queryRower, source, old string, start, end int64) error {
	var count int
	query := "SELECT COUNT(*) FROM " + source + " s LEFT JOIN " + old + " o ON o.id=s.id WHERE s.id>? AND s.id<=? AND (o.id IS NULL OR NOT (" + rowEqualitySQL("s", "o") + "))"
	if err := q.QueryRowContext(ctx, query, start, end).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("rollback mirror verification failed count=%d range=(%d,%d]", count, start, end)
	}
	return nil
}

func fullPostVerify(ctx context.Context, db *sql.DB, c config) error {
	source, _ := qualified(c.schema, c.source)
	old, _ := qualified(c.schema, c.old)
	var oldUpper, sourceUpper sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+old).Scan(&oldUpper); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, "SELECT MAX(id) FROM "+source).Scan(&sourceUpper); err != nil {
		return err
	}
	upper := maxInt64(oldUpper.Int64, sourceUpper.Int64)
	for start := int64(0); start < upper; {
		state, err := loadCheckpoint(ctx, db, c)
		if err != nil {
			return err
		}
		if err := assertRuntimeSafe(ctx, db, c, state, false); err != nil {
			return err
		}
		var oldEnd, sourceEnd sql.NullInt64
		endSQL := "SELECT MAX(id) FROM (SELECT id FROM %s FORCE INDEX(PRIMARY) WHERE id>? AND id<=? ORDER BY id LIMIT ?) x"
		if err := db.QueryRowContext(ctx, fmt.Sprintf(endSQL, old), start, upper, c.batchSize).Scan(&oldEnd); err != nil {
			return err
		}
		if err := db.QueryRowContext(ctx, fmt.Sprintf(endSQL, source), start, upper, c.batchSize).Scan(&sourceEnd); err != nil {
			return err
		}
		end := nextPostVerifyEnd(oldEnd, sourceEnd)
		if !end.Valid {
			break
		}
		batchCtx, cancel := context.WithTimeout(ctx, c.statementTimeout)
		var oldMissing, oldMismatch, sourceMissing, sourceMismatch int
		query := "SELECT COUNT(*) FROM " + old + " o LEFT JOIN " + source + " s ON s.id=o.id WHERE o.id>? AND o.id<=? AND " + retainedPredicate("o", c.channelIDs) + " AND s.id IS NULL"
		err = db.QueryRowContext(batchCtx, query, start, end.Int64).Scan(&oldMissing)
		if err == nil {
			query = "SELECT COUNT(*) FROM " + old + " o JOIN " + source + " s ON s.id=o.id WHERE o.id>? AND o.id<=? AND " + retainedPredicate("o", c.channelIDs) + " AND NOT (" + rowEqualitySQL("o", "s") + ")"
			err = db.QueryRowContext(batchCtx, query, start, end.Int64).Scan(&oldMismatch)
		}
		if err == nil {
			query = "SELECT COUNT(*) FROM " + source + " s LEFT JOIN " + old + " o ON o.id=s.id WHERE s.id>? AND s.id<=? AND o.id IS NULL"
			err = db.QueryRowContext(batchCtx, query, start, end.Int64).Scan(&sourceMissing)
		}
		if err == nil {
			query = "SELECT COUNT(*) FROM " + source + " s JOIN " + old + " o ON o.id=s.id WHERE s.id>? AND s.id<=? AND NOT (" + rowEqualitySQL("s", "o") + ")"
			err = db.QueryRowContext(batchCtx, query, start, end.Int64).Scan(&sourceMismatch)
		}
		cancel()
		if err != nil || oldMissing != 0 || oldMismatch != 0 || sourceMissing != 0 || sourceMismatch != 0 {
			return fmt.Errorf("full POST verification failed range=(%d,%d] old_missing=%d old_mismatch=%d source_missing=%d source_mismatch=%d err=%v", start, end.Int64, oldMissing, oldMismatch, sourceMissing, sourceMismatch, err)
		}
		start = end.Int64
		if c.batchDelay > 0 {
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case <-time.After(c.batchDelay):
			}
		}
	}
	return nil
}

func nextPostVerifyEnd(oldEnd, sourceEnd sql.NullInt64) sql.NullInt64 {
	switch {
	case oldEnd.Valid && sourceEnd.Valid && oldEnd.Int64 <= sourceEnd.Int64:
		return oldEnd
	case oldEnd.Valid && sourceEnd.Valid:
		return sourceEnd
	case oldEnd.Valid:
		return oldEnd
	default:
		return sourceEnd
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func assertRollbackReady(ctx context.Context, db *sql.DB, c config, state checkpoint) error {
	if state.phase != "rollback-ready" || !state.rollback.Valid || state.ddlOperation.Valid || state.ddlNonce.Valid {
		return fmt.Errorf("checkpoint is not ROLLBACK_READY")
	}
	o, t, err := observeTopology(ctx, db, c)
	if err != nil {
		return err
	}
	if classifyTopology(o) != topologyPostCutover || t.forward || !t.reverse || !postGuardsReady(t, c) {
		return fmt.Errorf("ROLLBACK_READY topology mismatch objects=%+v triggers=%+v", o, t)
	}
	return nil
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func safeNext(max uint64) (uint64, error) {
	if max == math.MaxUint64 {
		return 0, fmt.Errorf("AUTO_INCREMENT exhausted")
	}
	return max + 1, nil
}
