package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const usage = "usage: supplier_admin_finalize <verify|finalize>"

const (
	expectedDatabaseIdentityEnv = "SUPPLIER_ADMIN_FINALIZE_EXPECTED_DB_IDENTITY"
	drainEvidenceReferenceEnv   = "SUPPLIER_ADMIN_FINALIZE_DRAIN_EVIDENCE_REF"
	maxDatabaseIdentityBytes    = 256
	maxDrainEvidenceRefBytes    = 512
)

type commandOutput struct {
	Action                    string `json:"action"`
	Dialect                   string `json:"dialect"`
	DatabaseIdentity          string `json:"database_identity"`
	DrainEvidenceRef          string `json:"drain_evidence_ref"`
	AdminCommandLedgerState   string `json:"admin_command_ledger_state"`
	HasRequiredSupplierSchema bool   `json:"has_required_supplier_schema"`
	HasControlOptionTable     bool   `json:"has_control_option_table"`
	Finalized                 bool   `json:"finalized"`
	HasActorDigestIndex       bool   `json:"has_actor_digest_index"`
	HasSchedulerDigestIndex   bool   `json:"has_scheduler_digest_index"`
	HasInventoryActorIndex    bool   `json:"has_inventory_actor_index"`
	LegacyScopeKeyIndex       bool   `json:"legacy_scope_key_index"`
	LegacyActorScopeKeyIndex  bool   `json:"legacy_actor_scope_key_index"`
	LegacyInventoryKeyIndex   bool   `json:"legacy_inventory_key_index"`
	InvalidDigestRows         int64  `json:"invalid_digest_rows"`
}

type openDatabaseFunc func(string) (*gorm.DB, error)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, openMaintenanceDatabase, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, openDatabase openDatabaseFunc, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "verify" && args[0] != "finalize") {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	dsn := strings.TrimSpace(getenv("SQL_DSN"))
	if dsn == "" {
		fmt.Fprintln(stderr, "SQL_DSN is required; refusing SQLite fallback")
		return 2
	}
	expectedIdentity := getenv(expectedDatabaseIdentityEnv)
	if !validAuditReference(expectedIdentity, maxDatabaseIdentityBytes) {
		fmt.Fprintf(stderr, "%s is required and must be a bounded printable identity\n", expectedDatabaseIdentityEnv)
		return 2
	}
	drainEvidenceRef := getenv(drainEvidenceReferenceEnv)
	if !validAuditReference(drainEvidenceRef, maxDrainEvidenceRefBytes) {
		fmt.Fprintf(stderr, "%s is required and must be a bounded printable reference\n", drainEvidenceReferenceEnv)
		return 2
	}
	db, err := openDatabase(dsn)
	if err != nil {
		fmt.Fprintln(stderr, "open maintenance database: connection failed")
		return 1
	}
	actualIdentity, err := model.SupplierDatabaseIdentity(db)
	if err != nil {
		fmt.Fprintf(stderr, "read maintenance database identity: %v\n", err)
		return 1
	}
	if actualIdentity != expectedIdentity {
		fmt.Fprintf(stderr, "maintenance database identity mismatch: expected %q, got %q\n", expectedIdentity, actualIdentity)
		return 1
	}
	if args[0] == "finalize" {
		if err := model.FinalizeSupplierAdminCommandLedgerMigration(db); err != nil {
			fmt.Fprintf(stderr, "finalize supplier admin command ledger migration: %v\n", err)
			writeStatus(stderr, args[0], db, actualIdentity, drainEvidenceRef)
			return 1
		}
	}
	if err := model.ValidateSupplierAdminCommandLedgerFinalized(db); err != nil {
		fmt.Fprintf(stderr, "verify supplier admin command ledger migration: %v\n", err)
		writeStatus(stderr, args[0], db, actualIdentity, drainEvidenceRef)
		return 1
	}
	if err := writeStatus(stdout, args[0], db, actualIdentity, drainEvidenceRef); err != nil {
		fmt.Fprintf(stderr, "write finalizer status: %v\n", err)
		return 1
	}
	return 0
}

func validAuditReference(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || strings.TrimSpace(value) != value {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00")
}

func openMaintenanceDatabase(dsn string) (*gorm.DB, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" || strings.HasPrefix(dsn, "local") || strings.HasPrefix(dsn, "file:") || strings.Contains(dsn, ":memory:") {
		return nil, fmt.Errorf("only an explicit MySQL or PostgreSQL SQL_DSN is accepted")
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), PrepareStmt: true}
	var dialector gorm.Dialector
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	} else {
		dialector = mysql.Open(dsn)
	}
	db, err := gorm.Open(dialector, config)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(0)
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func writeStatus(writer io.Writer, action string, db *gorm.DB, databaseIdentity, drainEvidenceRef string) error {
	status, err := model.GetSupplierAdminCommandLedgerMigrationStatus(db)
	if err != nil {
		return err
	}
	encoded, err := common.Marshal(commandOutput{
		Action:                    action,
		Dialect:                   db.Dialector.Name(),
		DatabaseIdentity:          databaseIdentity,
		DrainEvidenceRef:          drainEvidenceRef,
		AdminCommandLedgerState:   status.State(),
		HasRequiredSupplierSchema: status.HasRequiredSupplierSchema,
		HasControlOptionTable:     status.HasControlOptionTable,
		Finalized:                 status.Finalized,
		HasActorDigestIndex:       status.HasActorDigestIndex,
		HasSchedulerDigestIndex:   status.HasSchedulerDigestIndex,
		HasInventoryActorIndex:    status.HasInventoryActorIndex,
		LegacyScopeKeyIndex:       status.LegacyScopeKeyIndex,
		LegacyActorScopeKeyIndex:  status.LegacyActorScopeKeyIndex,
		LegacyInventoryKeyIndex:   status.LegacyInventoryKeyIndex,
		InvalidDigestRows:         status.InvalidDigestRows,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(writer, string(encoded))
	return err
}
