package main

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var logColumns = []string{
	"id", "user_id", "created_at", "type", "content", "username", "token_name", "model_name",
	"quota", "prompt_tokens", "completion_tokens", "use_time", "is_stream", "channel_id",
	"channel_name", "token_id", "`group`", "ip", "request_id", "other", "upstream_request_id",
}

var textColumns = []string{"content", "username", "token_name", "model_name", "channel_name", "`group`", "ip", "request_id", "other", "upstream_request_id"}

func auditedSchemaContractHash() string {
	var b strings.Builder
	for _, column := range auditedLogColumns {
		fmt.Fprintf(&b, "%s|%s|%s|%s|%s|%s\n", column.name, column.columnType, column.nullable, column.charset, column.collation, column.extra)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(b.String())))
}

func qualified(schema, table string) (string, error) {
	qs, err := quoteIdentifier(schema)
	if err != nil {
		return "", err
	}
	qt, err := quoteIdentifier(table)
	if err != nil {
		return "", err
	}
	return qs + "." + qt, nil
}

func channelList(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func retainedPredicate(alias string, ids []int64) string {
	return "NOT (" + filteredPredicate(alias, ids) + ")"
}

func filteredPredicate(alias string, ids []int64) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	return fmt.Sprintf("COALESCE(%suser_id, -1) = 1 AND COALESCE(%stoken_id, 0) > 0 AND COALESCE(%schannel_id, -1) IN (%s)", prefix, prefix, prefix, channelList(ids))
}

func buildCopySQL(c config) (string, error) {
	source, err := qualified(c.schema, c.source)
	if err != nil {
		return "", err
	}
	target, err := qualified(c.schema, c.target)
	if err != nil {
		return "", err
	}
	columns := strings.Join(logColumns, ", ")
	return fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s FORCE INDEX (PRIMARY) WHERE id > ? AND id <= ? AND id <= ? AND %s ON DUPLICATE KEY UPDATE id = %s.id", target, columns, columns, source, retainedPredicate("", c.channelIDs), target), nil
}

func buildMirrorCopySQL(c config, from, to string) (string, error) {
	source, err := qualified(c.schema, from)
	if err != nil {
		return "", err
	}
	target, err := qualified(c.schema, to)
	if err != nil {
		return "", err
	}
	columns := strings.Join(logColumns, ", ")
	return "INSERT INTO " + target + " (" + columns + ") SELECT " + columns + " FROM " + source + " WHERE id>? AND id<=? ON DUPLICATE KEY UPDATE id=" + target + ".id", nil
}

func rowEqualitySQL(left, right string) string {
	text := make(map[string]struct{}, len(textColumns))
	for _, column := range textColumns {
		text[column] = struct{}{}
	}
	parts := make([]string, 0, len(logColumns))
	for _, column := range logColumns {
		if _, ok := text[column]; ok {
			parts = append(parts, fmt.Sprintf("BINARY %s.%s <=> BINARY %s.%s", left, column, right, column))
		} else {
			parts = append(parts, fmt.Sprintf("%s.%s <=> %s.%s", left, column, right, column))
		}
	}
	return strings.Join(parts, " AND ")
}

func triggerName(kind, batch string) (string, error) {
	name := "logs_slim_" + kind + "_" + batch
	if _, err := quoteIdentifier(name); err != nil {
		return "", err
	}
	return name, nil
}

func triggerDefiner(definer string) (string, error) {
	parts := strings.Split(definer, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(definer, "`'\";\\") {
		return "", fmt.Errorf("trigger definer must be a simple user@host value")
	}
	return "`" + parts[0] + "`@`" + parts[1] + "`", nil
}

func buildForwardTriggerSQL(c config) (string, error) {
	name, err := triggerName("forward", c.batch)
	if err != nil {
		return "", err
	}
	qn, _ := quoteIdentifier(name)
	source, err := qualified(c.schema, c.source)
	if err != nil {
		return "", err
	}
	target, err := qualified(c.schema, c.target)
	if err != nil {
		return "", err
	}
	definer, err := triggerDefiner(c.triggerDefiner)
	if err != nil {
		return "", err
	}
	values := make([]string, len(logColumns))
	for i, column := range logColumns {
		values[i] = "NEW." + column
	}
	return fmt.Sprintf("CREATE DEFINER=%s TRIGGER %s AFTER INSERT ON %s FOR EACH ROW BEGIN IF %s THEN INSERT INTO %s (%s) VALUES (%s); END IF; END", definer, qn, source, retainedPredicate("NEW", c.channelIDs), target, strings.Join(logColumns, ", "), strings.Join(values, ", ")), nil
}

func buildStrictMirrorTriggerSQL(c config, from, to string) (string, error) {
	name, err := triggerName("reverse", c.batch)
	if err != nil {
		return "", err
	}
	qn, _ := quoteIdentifier(name)
	qfrom, err := qualified(c.schema, from)
	if err != nil {
		return "", err
	}
	qto, err := qualified(c.schema, to)
	if err != nil {
		return "", err
	}
	definer, err := triggerDefiner(c.triggerDefiner)
	if err != nil {
		return "", err
	}
	values := make([]string, len(logColumns))
	for i, column := range logColumns {
		values[i] = "NEW." + column
	}
	return fmt.Sprintf("CREATE DEFINER=%s TRIGGER %s AFTER INSERT ON %s FOR EACH ROW INSERT INTO %s (%s) VALUES (%s)", definer, qn, qfrom, qto, strings.Join(logColumns, ", "), strings.Join(values, ", ")), nil
}

func buildGuardTriggerSQL(c config, event, table string) (string, error) {
	return buildNamedGuardTriggerSQL(c, "guard_"+strings.ToLower(event), event, table)
}

func buildNamedGuardTriggerSQL(c config, triggerKind, event, table string) (string, error) {
	kind := strings.ToLower(event)
	if kind != "update" && kind != "delete" {
		return "", fmt.Errorf("unsupported guard event %q", event)
	}
	name, err := triggerName(triggerKind, c.batch)
	if err != nil {
		return "", err
	}
	qn, _ := quoteIdentifier(name)
	source, err := qualified(c.schema, table)
	if err != nil {
		return "", err
	}
	definer, err := triggerDefiner(c.triggerDefiner)
	if err != nil {
		return "", err
	}
	signal := "SIGNAL SQLSTATE '45000' SET MYSQL_ERRNO=1644, MESSAGE_TEXT='logs slimming append-only guard'"
	if kind == "update" {
		// INSERT ... ON DUPLICATE KEY UPDATE id=<same id> executes BEFORE UPDATE.
		// Permit only a complete OLD/NEW no-op; every real mutation is rejected.
		return fmt.Sprintf("CREATE DEFINER=%s TRIGGER %s BEFORE UPDATE ON %s FOR EACH ROW BEGIN IF NOT (%s) THEN %s; END IF; END", definer, qn, source, rowEqualitySQL("OLD", "NEW"), signal), nil
	}
	return fmt.Sprintf("CREATE DEFINER=%s TRIGGER %s BEFORE DELETE ON %s FOR EACH ROW %s", definer, qn, source, signal), nil
}

func checkpointCASSQL(c config) (string, error) {
	table, err := qualified(c.schema, c.checkpoint)
	if err != nil {
		return "", err
	}
	return "UPDATE " + table + " SET phase = ?, last_completed_end_id = ?, seed_cutoff_id = ?, final_cutoff_id = ?, generation = generation + 1, updated_at = CURRENT_TIMESTAMP(6) WHERE id = 1 AND generation = ?", nil
}

func targetAutoIncrement(sourceNext, reserve uint64) (uint64, error) {
	if reserve == 0 || sourceNext > math.MaxUint64-reserve {
		return 0, fmt.Errorf("AUTO_INCREMENT reserve overflow")
	}
	return sourceNext + reserve, nil
}
