package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ddlState string

const (
	ddlPre     ddlState = "PRE"
	ddlPost    ddlState = "POST"
	ddlUnknown ddlState = "UNKNOWN"
)

type ddlObserver func(context.Context, *sql.Conn, string) (ddlState, error)

func sameDDL(expected, actual string) bool {
	return strings.Join(strings.Fields(expected), " ") == strings.Join(strings.Fields(actual), " ")
}

func ddlOperation(statement string) string {
	fields := strings.Fields(strings.ToUpper(statement))
	if len(fields) == 0 {
		return "EMPTY"
	}
	if len(fields) == 1 {
		return fields[0]
	}
	return fields[0] + "_" + fields[1]
}

func ddlNonce() (string, error) {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// runDDL bounds the complete operation, including kill and two independent
// postcondition observations. If KILL does not resolve the driver call within
// the bounded grace period, the physical DDL connection is closed only by a
// background reaper after Exec returns; the caller never blocks in Close.
func runDDL(ctx context.Context, db *sql.DB, c config, statement string, observe ddlObserver) error {
	started := time.Now()
	deadline := started.Add(c.ddlTimeout)
	opCtx, cancelOperation := context.WithDeadline(ctx, deadline)
	defer cancelOperation()
	operation := ddlOperation(statement)
	nonce, err := ddlNonce()
	if err != nil {
		return fmt.Errorf("generate DDL nonce: %w", err)
	}
	ddlConn, ddlFacts, err := openVerifiedConn(opCtx, db, c, "ddl")
	if err != nil {
		return err
	}
	control, controlFacts, err := openVerifiedConn(opCtx, db, c, "control")
	if err != nil {
		_ = ddlConn.Close()
		return err
	}
	defer control.Close()
	if controlFacts.connectionID == ddlFacts.connectionID {
		_ = ddlConn.Close()
		return fmt.Errorf("DDL and control connections unexpectedly share id %d", ddlFacts.connectionID)
	}
	if _, err = ddlConn.ExecContext(opCtx, fmt.Sprintf("SET SESSION lock_wait_timeout = %d", c.lockWaitSeconds)); err != nil {
		_ = ddlConn.Close()
		return err
	}
	tagged := fmt.Sprintf("/*logs_slim batch=%s operation=%s nonce=%s*/ %s", c.batch, operation, nonce, statement)
	ev, ok := ctx.Value(evidenceContextKey{}).(*evidence)
	if !ok {
		_ = ddlConn.Close()
		return fmt.Errorf("DDL requires fail-closed evidence context")
	}
	statementHash := fmt.Sprintf("%x", sha256.Sum256([]byte(statement)))
	if err := ev.emit("ddl_intent", map[string]any{"batch": c.batch, "operation": operation, "nonce": nonce, "connection_id": ddlFacts.connectionID, "statement_sha256": statementHash}); err != nil {
		_ = ddlConn.Close()
		return fmt.Errorf("write DDL intent evidence: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		_, execErr := ddlConn.ExecContext(opCtx, tagged)
		done <- execErr
	}()
	watchdogAt := time.Until(deadline) - 600*time.Millisecond
	if watchdogAt < 50*time.Millisecond {
		watchdogAt = 50 * time.Millisecond
	}
	timer := time.NewTimer(watchdogAt)
	defer timer.Stop()
	resolved := false
	var execErr error
	var killErr error
	select {
	case execErr = <-done:
		resolved = true
	case <-timer.C:
		killCtx, cancel := boundedBackground(deadline, 250*time.Millisecond)
		killErr = killExactDDL(killCtx, control, c, ctx, ddlFacts, tagged)
		cancel()
		// Cancellation is mandatory even when the exact KILL could not be proven.
		// A failed KILL is an UNKNOWN outcome, never permission to return early
		// while the server-side statement may still complete later.
		cancelOperation()
		grace := time.NewTimer(ddlSettleGrace)
		select {
		case execErr = <-done:
			resolved = true
		case <-grace.C:
		}
		grace.Stop()
	}
	if resolved {
		_ = ddlConn.Close()
	} else {
		go reapDDLConn(done, ddlConn)
	}

	emitUnknown := func(states []ddlState, observeErr error) error {
		fields := map[string]any{
			"operation": operation, "nonce": nonce, "statement_sha256": statementHash,
			"state": fmt.Sprint(states), "result": "unknown", "exec_resolved": resolved,
			"exec_error": execErr, "kill_error": killErr, "observe_error": observeErr,
		}
		if err := ev.emit("ddl_postcondition", fields); err != nil {
			return fmt.Errorf("write unknown DDL postcondition evidence: %w", err)
		}
		return nil
	}
	observeDeadline := deadline
	states := make([]ddlState, 0, 2)
	for i := 0; i < 2; i++ {
		if positiveRemaining(observeDeadline) <= time.Nanosecond {
			err := fmt.Errorf("DDL total watchdog expired before postcondition")
			if emitErr := emitUnknown(states, err); emitErr != nil {
				return emitErr
			}
			return err
		}
		observeCtx, cancel := boundedBackground(observeDeadline, 250*time.Millisecond)
		observer, _, openErr := openVerifiedConn(observeCtx, db, c, "observer")
		if openErr != nil {
			cancel()
			if emitErr := emitUnknown(states, openErr); emitErr != nil {
				return emitErr
			}
			return openErr
		}
		state, observeErr := observe(observeCtx, observer, ddlFacts.sqlMode)
		_ = observer.Close()
		cancel()
		if observeErr != nil {
			if emitErr := emitUnknown(states, observeErr); emitErr != nil {
				return emitErr
			}
			return observeErr
		}
		states = append(states, state)
		if i == 0 {
			time.Sleep(minDuration(25*time.Millisecond, positiveRemaining(deadline)))
		}
	}
	outcome := stableDDLPostcondition(resolved, states)
	if !resolved {
		if err := emitUnknown(states, nil); err != nil {
			return err
		}
		return fmt.Errorf("DDL outcome UNKNOWN: exec_resolved=%t kill=%v states=%v", resolved, killErr, states)
	}
	if outcome == ddlUnknown {
		if err := emitUnknown(states, nil); err != nil {
			return err
		}
		return fmt.Errorf("DDL postcondition is unstable or unknown: %v", states)
	}
	if outcome == ddlPost {
		if err := ev.emit("ddl_postcondition", map[string]any{"operation": operation, "nonce": nonce, "statement_sha256": statementHash, "state": string(ddlPost), "elapsed_ms": time.Since(started).Milliseconds(), "exec_error": execErr, "kill_warning": killErr}); err != nil {
			return fmt.Errorf("write DDL postcondition evidence: %w", err)
		}
		return nil
	}
	if err := ev.emit("ddl_postcondition", map[string]any{"operation": operation, "nonce": nonce, "statement_sha256": statementHash, "state": string(ddlPre), "elapsed_ms": time.Since(started).Milliseconds(), "exec_error": execErr, "kill_warning": killErr}); err != nil {
		return fmt.Errorf("write DDL PRE postcondition evidence: %w", err)
	}
	if execErr != nil {
		return execErr
	}
	return fmt.Errorf("DDL returned but postcondition remained PRE")
}

func stableDDLPostcondition(resolved bool, states []ddlState) ddlState {
	if !resolved || len(states) != 2 || states[0] != states[1] || states[0] == ddlUnknown {
		return ddlUnknown
	}
	return states[0]
}

const ddlSettleGrace = 200 * time.Millisecond

func killExactDDL(ctx context.Context, control *sql.Conn, c config, lockCtx context.Context, ddlFacts sessionFacts, tagged string) error {
	if _, err := verifyPhysicalConn(ctx, control, c, "control-watchdog"); err != nil {
		return err
	}
	var database sql.NullString
	var user, command string
	var info sql.NullString
	if err := control.QueryRowContext(ctx, "SELECT DB,USER,COMMAND,INFO FROM information_schema.PROCESSLIST WHERE ID=?", ddlFacts.connectionID).Scan(&database, &user, &command, &info); err != nil {
		return fmt.Errorf("watchdog cannot prove DDL connection identity: %w", err)
	}
	lockOwner, ok := lockCtx.Value(lockOwnerContextKey{}).(int64)
	var liveOwner sql.NullInt64
	lockName := advisoryLockName(c)
	lockErr := control.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?)", lockName).Scan(&liveOwner)
	expectedUser := strings.SplitN(ddlFacts.currentUser, "@", 2)[0]
	if !ok || lockErr != nil || !liveOwner.Valid || liveOwner.Int64 != lockOwner || !database.Valid || database.String != c.schema || user != expectedUser || command != "Query" || !info.Valid || !sameDDL(tagged, info.String) {
		return fmt.Errorf("watchdog refused KILL: connection %d does not match exact batch/operation/nonce/DB/USER/lock owner", ddlFacts.connectionID)
	}
	if _, err := control.ExecContext(ctx, fmt.Sprintf("KILL QUERY %d", ddlFacts.connectionID)); err != nil {
		return fmt.Errorf("watchdog KILL QUERY %d: %w", ddlFacts.connectionID, err)
	}
	return nil
}

func reapDDLConn(done <-chan error, conn *sql.Conn) {
	<-done
	_ = conn.Close()
}

func boundedBackground(deadline time.Time, maximum time.Duration) (context.Context, context.CancelFunc) {
	if time.Until(deadline) <= maximum {
		return context.WithDeadline(context.Background(), deadline)
	}
	return context.WithTimeout(context.Background(), maximum)
}

func positiveRemaining(deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Nanosecond
	}
	return remaining
}
