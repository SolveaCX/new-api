package main

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRunRejectsUnsafeInvocationBeforeOpeningDatabase(t *testing.T) {
	opened := false
	opener := func(string) (*gorm.DB, error) {
		opened = true
		return nil, errors.New("must not open")
	}
	for _, testCase := range []struct {
		name, dsn, expectedIdentity, evidence string
		args                                  []string
		code                                  int
	}{
		{name: "missing action", code: 2},
		{name: "unknown action", args: []string{"drop"}, dsn: "dsn", code: 2},
		{name: "missing dsn", args: []string{"verify"}, code: 2},
		{name: "missing expected identity", args: []string{"verify"}, dsn: "dsn", evidence: "change-123", code: 2},
		{name: "missing drain evidence", args: []string{"verify"}, dsn: "dsn", expectedIdentity: "mysql:prod", code: 2},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			opened = false
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			values := map[string]string{"SQL_DSN": testCase.dsn, expectedDatabaseIdentityEnv: testCase.expectedIdentity, drainEvidenceReferenceEnv: testCase.evidence}
			code := run(testCase.args, func(key string) string { return values[key] }, opener, &stdout, &stderr)
			require.Equal(t, testCase.code, code)
			require.False(t, opened)
			require.Empty(t, stdout.String())
			require.NotEmpty(t, stderr.String())
		})
	}
}

func TestRunVerifyAndFinalizeAreAuditable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	opener := func(string) (*gorm.DB, error) { return db, nil }
	values := map[string]string{
		"SQL_DSN":                   "injected-test-dsn",
		expectedDatabaseIdentityEnv: "sqlite:main",
		drainEvidenceReferenceEnv:   "change-request-123/drain-proof",
	}
	getenv := func(key string) string { return values[key] }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	require.Equal(t, 1, run([]string{"verify"}, getenv, opener, &stdout, &stderr))
	require.Contains(t, stderr.String(), "not finalized")
	require.Contains(t, stderr.String(), `"finalized":false`)
	require.Contains(t, stderr.String(), `"admin_command_ledger_state":"invalid"`)

	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SupplierAdminCommand{}, &model.SupplierInventoryAdjustment{}))
	stdout.Reset()
	stderr.Reset()
	require.Equal(t, 0, run([]string{"finalize"}, getenv, opener, &stdout, &stderr))
	require.Empty(t, stderr.String())
	require.Contains(t, stdout.String(), `"action":"finalize"`)
	require.Contains(t, stdout.String(), `"database_identity":"sqlite:main"`)
	require.Contains(t, stdout.String(), `"drain_evidence_ref":"change-request-123/drain-proof"`)
	require.Contains(t, stdout.String(), `"admin_command_ledger_state":"finalized"`)
	require.Contains(t, stdout.String(), `"finalized":true`)

	stdout.Reset()
	require.Equal(t, 0, run([]string{"finalize"}, getenv, opener, &stdout, &stderr))
	require.Contains(t, stdout.String(), `"finalized":true`)
	require.NoError(t, model.ValidateSupplierAdminCommandLedgerFinalized(db))
}

func TestRunRejectsWrongDatabaseIdentityBeforeFinalization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SupplierAdminCommand{}, &model.SupplierInventoryAdjustment{}))
	opener := func(string) (*gorm.DB, error) { return db, nil }
	values := map[string]string{
		"SQL_DSN":                   "injected-test-dsn",
		expectedDatabaseIdentityEnv: "sqlite:wrong",
		drainEvidenceReferenceEnv:   "change-request-456/drain-proof",
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	require.Equal(t, 1, run([]string{"finalize"}, func(key string) string { return values[key] }, opener, &stdout, &stderr))
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "identity mismatch")
	require.ErrorIs(t, model.ValidateSupplierAdminCommandLedgerFinalized(db), model.ErrSupplierAdminCommandLedgerNotFinalized)
}

func TestRunRejectsInvalidDrainEvidenceBeforeOpeningDatabase(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		evidence string
	}{
		{name: "newline", evidence: "change-123\ndrain-proof"},
		{name: "NUL", evidence: "change-123\x00drain-proof"},
		{name: "overlength", evidence: strings.Repeat("x", maxDrainEvidenceRefBytes+1)},
		{name: "leading whitespace", evidence: " change-123/drain-proof"},
		{name: "trailing whitespace", evidence: "change-123/drain-proof "},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			const dsn = "operator:super-secret@tcp(database.internal:3306)/newapi"
			opened := false
			opener := func(string) (*gorm.DB, error) {
				opened = true
				return nil, errors.New("must not open")
			}
			values := map[string]string{
				"SQL_DSN":                   dsn,
				expectedDatabaseIdentityEnv: "mysql:newapi",
				drainEvidenceReferenceEnv:   testCase.evidence,
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := run([]string{"verify"}, func(key string) string { return values[key] }, opener, &stdout, &stderr)

			require.Equal(t, 2, code)
			require.False(t, opened)
			require.Empty(t, stdout.String())
			require.Contains(t, stderr.String(), drainEvidenceReferenceEnv)
			require.NotContains(t, stderr.String(), dsn)
		})
	}
}

func TestRunDoesNotExposeSQLDSNWhenDatabaseOpenFails(t *testing.T) {
	const dsn = "operator:super-secret@tcp(database.internal:3306)/newapi"
	values := map[string]string{
		"SQL_DSN":                   dsn,
		expectedDatabaseIdentityEnv: "mysql:newapi",
		drainEvidenceReferenceEnv:   "change-123/drain-proof",
	}
	opener := func(receivedDSN string) (*gorm.DB, error) {
		require.Equal(t, dsn, receivedDSN)
		return nil, errors.New("database connection failed for " + receivedDSN)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"verify"}, func(key string) string { return values[key] }, opener, &stdout, &stderr)

	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.NotContains(t, stderr.String(), dsn)
}

func TestSupplierDatabaseIdentityFormatsMySQLAndPostgreSQLWithoutLiveDatabase(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		dialect  string
		expected string
	}{
		{name: "MySQL", dialect: "mysql", expected: "mysql:supplier_prod"},
		{name: "PostgreSQL", dialect: "postgres", expected: "postgres:supplier_prod/finance"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			connection := sql.OpenDB(identityConnector{})
			t.Cleanup(func() { require.NoError(t, connection.Close()) })
			dialector := identityDialector{
				Dialector: sqlite.Dialector{Conn: connection},
				name:      testCase.dialect,
			}
			db, err := gorm.Open(dialector, &gorm.Config{DisableAutomaticPing: true})
			require.NoError(t, err)

			identity, err := model.SupplierDatabaseIdentity(db)

			require.NoError(t, err)
			require.Equal(t, testCase.expected, identity)
		})
	}
}

func TestOpenMaintenanceDatabaseRejectsSQLiteFallbacks(t *testing.T) {
	for _, dsn := range []string{"", "local", "local-anything", "file:test.db", ":memory:"} {
		t.Run(strings.ReplaceAll(dsn, ":", "_"), func(t *testing.T) {
			_, err := openMaintenanceDatabase(dsn)
			require.ErrorContains(t, err, "only an explicit MySQL or PostgreSQL")
		})
	}
}

type identityDialector struct {
	sqlite.Dialector
	name string
}

func (dialector identityDialector) Name() string {
	return dialector.name
}

type identityConnector struct{}

func (identityConnector) Connect(context.Context) (driver.Conn, error) {
	return identityConnection{}, nil
}

func (identityConnector) Driver() driver.Driver {
	return identityDriver{}
}

type identityDriver struct{}

func (identityDriver) Open(string) (driver.Conn, error) {
	return identityConnection{}, nil
}

type identityConnection struct{}

func (identityConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported by identity test connection")
}

func (identityConnection) Close() error {
	return nil
}

func (identityConnection) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported by identity test connection")
}

func (identityConnection) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch strings.TrimSpace(query) {
	case "select sqlite_version()":
		return &identityRows{columns: []string{"sqlite_version"}, values: []driver.Value{"3.40.0"}}, nil
	case "SELECT DATABASE()":
		return &identityRows{columns: []string{"database_name"}, values: []driver.Value{"supplier_prod"}}, nil
	case "SELECT current_database() AS database_name, current_schema() AS schema_name":
		return &identityRows{columns: []string{"database_name", "schema_name"}, values: []driver.Value{"supplier_prod", "finance"}}, nil
	default:
		return nil, errors.New("unexpected identity test query: " + query)
	}
}

type identityRows struct {
	columns []string
	values  []driver.Value
	read    bool
}

func (rows *identityRows) Columns() []string {
	return rows.columns
}

func (*identityRows) Close() error {
	return nil
}

func (rows *identityRows) Next(destination []driver.Value) error {
	if rows.read {
		return io.EOF
	}
	copy(destination, rows.values)
	rows.read = true
	return nil
}
