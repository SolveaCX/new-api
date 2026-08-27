package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type sessionFacts struct {
	database, hostname, serverUUID string
	currentUser, authenticatedUser string
	connectionID                   int64
	isolation, binlog, sqlMode     string
	autoIncrementMode              int
}

func openVerifiedConn(ctx context.Context, db *sql.DB, c config, role string) (*sql.Conn, sessionFacts, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, sessionFacts{}, fmt.Errorf("open %s connection: %w", role, err)
	}
	facts, err := verifyPhysicalConn(ctx, conn, c, role)
	if err != nil {
		_ = conn.Close()
		return nil, sessionFacts{}, err
	}
	return conn, facts, nil
}

func verifyPhysicalConn(ctx context.Context, conn *sql.Conn, c config, role string) (sessionFacts, error) {
	checkCtx, cancel := context.WithTimeout(ctx, minDuration(c.statementTimeout, time.Second))
	defer cancel()
	var facts sessionFacts
	err := conn.QueryRowContext(checkCtx, "SELECT DATABASE(),@@hostname,@@server_uuid,CURRENT_USER(),USER(),CONNECTION_ID(),@@transaction_isolation,@@binlog_format,@@innodb_autoinc_lock_mode,@@SESSION.sql_mode").Scan(
		&facts.database, &facts.hostname, &facts.serverUUID, &facts.currentUser, &facts.authenticatedUser,
		&facts.connectionID, &facts.isolation, &facts.binlog, &facts.autoIncrementMode, &facts.sqlMode,
	)
	if err != nil {
		return facts, fmt.Errorf("verify %s connection: %w", role, err)
	}
	authUser := strings.SplitN(facts.authenticatedUser, "@", 2)[0]
	currentUser := strings.SplitN(facts.currentUser, "@", 2)[0]
	if facts.database != c.schema || facts.hostname != c.expectedHostname || facts.serverUUID != c.expectedServerUUID || facts.currentUser != c.expectedDatabaseUser || authUser != currentUser {
		return facts, fmt.Errorf("%s connection identity mismatch database=%q hostname=%q server_uuid=%q current_user=%q authenticated_user=%q", role, facts.database, facts.hostname, facts.serverUUID, facts.currentUser, facts.authenticatedUser)
	}
	if facts.isolation != "READ-COMMITTED" || facts.binlog != "ROW" || facts.autoIncrementMode != 2 {
		return facts, fmt.Errorf("%s connection session mismatch isolation=%q binlog=%q autoinc_mode=%d", role, facts.isolation, facts.binlog, facts.autoIncrementMode)
	}
	return facts, nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
