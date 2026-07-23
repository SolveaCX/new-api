package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type legacySupplierAdminCommand struct {
	Id             int    `gorm:"primaryKey"`
	ActorId        int    `gorm:"not null;default:0;index:idx_supplier_admin_command_actor_scope_key,priority:1"`
	Scope          string `gorm:"type:varchar(64);not null;uniqueIndex:ux_supplier_admin_command_scope_key,priority:1;index:idx_supplier_admin_command_actor_scope_key,priority:2"`
	IdempotencyKey string `gorm:"type:varchar(128);not null;uniqueIndex:ux_supplier_admin_command_scope_key,priority:2;index:idx_supplier_admin_command_actor_scope_key,priority:3"`
	PayloadVersion int    `gorm:"not null;default:1"`
	PayloadDigest  string `gorm:"type:varchar(64);not null"`
	ResourceType   string `gorm:"type:varchar(32);not null"`
	ResourceId     int    `gorm:"not null;default:0"`
	ClaimToken     string `gorm:"type:varchar(32);not null"`
	CreatedAt      int64  `gorm:"autoCreateTime"`
}

func (legacySupplierAdminCommand) TableName() string {
	return "supplier_admin_commands"
}

func TestSupplierAdminCreateSupplierIdempotentReplayConflictAndConcurrency(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-supplier")

	const callers = 24
	results := make([]*UpstreamSupplier, callers)
	replayed := make([]bool, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], replayed[i], errs[i] = CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "  durable supplier  ", Remark: " durable remark "}, "supplier-command-1")
		}(i)
	}
	wg.Wait()
	createdCount := 0
	for i := range errs {
		require.NoError(t, errs[i])
		require.NotNil(t, results[i])
		require.Equal(t, results[0].Id, results[i].Id)
		if !replayed[i] {
			createdCount++
		}
	}
	require.Equal(t, 1, createdCount)
	require.Equal(t, "durable supplier", results[0].Name)
	require.Equal(t, "durable remark", results[0].Remark)

	var supplierCount int64
	require.NoError(t, db.Model(&UpstreamSupplier{}).Where("name = ?", "durable supplier").Count(&supplierCount).Error)
	require.Equal(t, int64(1), supplierCount)
	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("scope = ? AND idempotency_key = ?", SupplierAdminCommandScopeCreateSupplier, "supplier-command-1").Count(&commandCount).Error)
	require.Equal(t, int64(1), commandCount)

	_, _, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "different supplier"}, "supplier-command-1")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	_, _, err = CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "missing key"}, "")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired)
}

func TestSupplierAdminCreateContractIdempotentUsesScopeAndPayload(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-contract")
	supplier, replayed, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "contract command supplier"}, "shared-key")
	require.NoError(t, err)
	require.False(t, replayed)

	input := &SupplierContract{SupplierId: supplier.Id, Name: " contract ", ContractNo: " contract-001 ", Remark: " remark ", RpmLimit: 10, TpmLimit: 20, MaxConcurrency: 3}
	contract, replayed, err := CreateSupplierContractIdempotent(input, "shared-key")
	require.NoError(t, err)
	require.False(t, replayed, "the same key is independent across command scopes")
	require.Equal(t, "contract", contract.Name)
	require.Equal(t, "contract-001", contract.ContractNo)

	replayedContract, replayed, err := CreateSupplierContractIdempotent(&SupplierContract{SupplierId: supplier.Id, Name: "contract", ContractNo: "contract-001", Remark: "remark", RpmLimit: 10, TpmLimit: 20, MaxConcurrency: 3}, "shared-key")
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, contract.Id, replayedContract.Id)

	_, _, err = CreateSupplierContractIdempotent(&SupplierContract{SupplierId: supplier.Id, Name: "changed", ContractNo: "contract-001"}, "shared-key")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	var count int64
	require.NoError(t, db.Model(&SupplierContract{}).Where("supplier_id = ?", supplier.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSupplierAdminCreateRateReplayReturnsOriginalWithoutReactivatingIt(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-rate")
	contract := createSupplierContractFixture(t, db, "rate command supplier", "rate-command-contract")

	first, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 1, " first ", "rate-command-1")
	require.NoError(t, err)
	require.False(t, replayed)
	second, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 700_000, 1, "second", "rate-command-2")
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, first.Id, second.Id)

	replayedFirst, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 1, "first", "rate-command-1")
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, first.Id, replayedFirst.Id)
	var persisted SupplierContract
	require.NoError(t, db.First(&persisted, contract.Id).Error)
	require.NotNil(t, persisted.CurrentRateVersionId)
	require.Equal(t, second.Id, *persisted.CurrentRateVersionId, "replaying an old command must not roll back the current rate")

	_, _, err = CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 600_000, 1, "different", "rate-command-1")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	var count int64
	require.NoError(t, db.Model(&SupplierContractRateVersion{}).Where("contract_id = ?", contract.Id).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestSupplierAdminCommandAndDomainCreateRollbackTogether(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-crash")
	payload := createUpstreamSupplierCommandPayload{Name: "crash-safe supplier", Remark: ""}
	digest, err := supplierAdminPayloadDigest(payload)
	require.NoError(t, err)
	injected := errors.New("injected crash before command completion")
	err = db.Transaction(func(tx *gorm.DB) error {
		claim, err := claimSupplierAdminCommand(tx, 0, SupplierAdminCommandScopeCreateSupplier, "crash-command-1", digest, supplierAdminCommandResourceSupplier)
		if err != nil {
			return err
		}
		require.True(t, claim.Claimed)
		if err := tx.Create(&UpstreamSupplier{Name: payload.Name}).Error; err != nil {
			return err
		}
		return injected
	})
	require.ErrorIs(t, err, injected)

	var count int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&UpstreamSupplier{}).Where("name = ?", payload.Name).Count(&count).Error)
	require.Zero(t, count)

	supplier, replayed, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: payload.Name}, "crash-command-1")
	require.NoError(t, err)
	require.False(t, replayed)
	require.Positive(t, supplier.Id)
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSupplierAdminCommandLedgerIsAppendOnlyAndIndexed(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-ledger")
	_, _, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "ledger supplier"}, "ledger-command-1")
	require.NoError(t, err)
	var command SupplierAdminCommand
	require.NoError(t, db.First(&command).Error)
	require.Positive(t, command.ResourceId)
	require.Len(t, command.IdempotencyKeyDigest, 32)
	require.Equal(t, supplierAdminIdempotencyKeyDigest(command.IdempotencyKey), command.IdempotencyKeyDigest)
	require.ErrorIs(t, db.Model(&command).Update("payload_digest", "different").Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Delete(&command).Error, ErrSupplierAppendOnly)
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key_digest"}, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))
	require.Equal(t, []string{"resource_type", "resource_id"}, supplierSQLiteIndexColumns(t, db, "idx_supplier_admin_command_resource"))
}

func TestSupplierAdminCommandDigestIndexFitsMySQL57Utf8mb4Limit(t *testing.T) {
	parsed, err := schema.Parse(&SupplierAdminCommand{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	_, exists := parsed.ParseIndexes()[supplierAdminCommandActorScopeDigestIndex]
	require.False(t, exists, "AutoMigrate must not create the digest index before the bridge backfill")
	require.Equal(t, 32, parsed.LookUpField("IdempotencyKeyDigest").Size)

	// MySQL 5.7's conservative InnoDB limit is 767 bytes: BIGINT actor (8)
	// + utf8mb4 VARCHAR(64) scope (256) + binary SHA-256 digest (32).
	require.LessOrEqual(t, 8+64*4+sha256.Size, 767)
}

func TestGetSupplierAdminCommandResultIsExactAndActorScoped(t *testing.T) {
	setupSupplierTestDB(t, "supplier-admin-command-result")
	created, replayed, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "lookup supplier"}, "lookup-key", 7)
	require.NoError(t, err)
	require.False(t, replayed)

	result, err := GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, " lookup-key ")
	require.NoError(t, err)
	require.Equal(t, SupplierAdminCommandScopeCreateSupplier, result.Scope)
	require.Equal(t, "lookup-key", result.IdempotencyKey)
	require.Equal(t, supplierAdminCommandResourceSupplier, result.ResourceType)
	require.Equal(t, created.Id, result.ResourceId)
	require.Positive(t, result.CreatedAt)

	_, err = GetSupplierAdminCommandResult(8, SupplierAdminCommandScopeCreateSupplier, "lookup-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "another administrator must not discover the command")
	_, err = GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateContract, "lookup-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "a coincident key in another scope is not the same command")
	_, err = GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, "missing-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetSupplierAdminCommandResult(7, "supplier.invalid", "lookup-key")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired)
	_, err = GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, "")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired)
}

func TestSupplierAdminCommandSameKeyIsActorLocalAndCannotLeakPayload(t *testing.T) {
	setupSupplierTestDB(t, "supplier-admin-command-actor-conflict")
	actorSeven, _, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "actor seven supplier"}, "shared-actor-key", 7)
	require.NoError(t, err)
	actorEight, replayed, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "actor eight supplier"}, "shared-actor-key", 8)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, actorSeven.Id, actorEight.Id)

	resultSeven, err := GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, "shared-actor-key")
	require.NoError(t, err)
	resultEight, err := GetSupplierAdminCommandResult(8, SupplierAdminCommandScopeCreateSupplier, "shared-actor-key")
	require.NoError(t, err)
	require.Equal(t, actorSeven.Id, resultSeven.ResourceId)
	require.Equal(t, actorEight.Id, resultEight.ResourceId)
}

func TestMigrateSupplierAdminCommandLedgerRepairsLegacyIndexWithoutDataLoss(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy-command-ledger.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacySupplierAdminCommand{}))

	legacyRows := []legacySupplierAdminCommand{
		{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "legacy-key", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 1), ResourceType: "activation", ResourceId: 3, ClaimToken: fmt.Sprintf("%032x", 1)},
		{ActorId: 8, Scope: "activation.transition", IdempotencyKey: "other-key", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 2), ResourceType: "activation", ResourceId: 4, ClaimToken: fmt.Sprintf("%032x", 2)},
		{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "second-actor-seven-key", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 3), ResourceType: "activation", ResourceId: 5, ClaimToken: fmt.Sprintf("%032x", 3)},
	}
	require.NoError(t, db.Create(&legacyRows).Error)
	require.Equal(t, []string{"scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))

	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db), "bridge must be rerunnable")
	require.Equal(t, []string{"scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key_digest"}, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))
	status, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
	require.NoError(t, err)
	require.False(t, status.Finalized)
	require.True(t, status.HasDigestColumn)
	require.True(t, status.HasResultColumn)
	require.True(t, status.HasActorDigestIndex)
	require.True(t, status.LegacyScopeKeyIndex)
	require.True(t, status.LegacyActorScopeKeyIndex)
	require.Zero(t, status.InvalidDigestRows)
	require.ErrorIs(t, ValidateSupplierAdminCommandLedgerFinalized(db), ErrSupplierAdminCommandLedgerNotFinalized)

	var migrated []SupplierAdminCommand
	require.NoError(t, db.Order("id ASC").Find(&migrated).Error)
	require.Len(t, migrated, len(legacyRows))
	for i := range migrated {
		require.Equal(t, legacyRows[i].Id, migrated[i].Id)
		require.Equal(t, legacyRows[i].IdempotencyKey, migrated[i].IdempotencyKey)
		require.Equal(t, supplierAdminIdempotencyKeyDigest(legacyRows[i].IdempotencyKey), migrated[i].IdempotencyKeyDigest)
	}
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db), "finalization must be rerunnable")
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
	require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))

	payload := map[string]any{"next_state_version": 5}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, 8, "activation.transition", "legacy-key", payload, "activation")
		if err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, 5, payload)
	}))
	var actorLocalCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("scope = ? AND idempotency_key = ?", "activation.transition", "legacy-key").Count(&actorLocalCount).Error)
	require.Equal(t, int64(2), actorLocalCount)
}

func TestSupplierAdminCommandLedgerCleanBridgeIsFinalizedAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "clean-command-ledger.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierAdminCommand{}))
	require.Empty(t, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))

	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key_digest"}, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))
}

func TestSupplierAdminCommandLedgerBridgeRejectsDuplicateActorScopeDigest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "duplicate-command-ledger.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierAdminCommand{}))
	rows := []SupplierAdminCommand{
		{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "same-key", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 1), ResourceType: "activation", ResourceId: 1, ClaimToken: fmt.Sprintf("%032x", 1)},
		{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "same-key", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 2), ResourceType: "activation", ResourceId: 2, ClaimToken: fmt.Sprintf("%032x", 2)},
	}
	require.NoError(t, db.Create(&rows).Error)

	err = MigrateSupplierAdminCommandLedger(db)
	require.Error(t, err)
	require.Empty(t, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))
}

func TestSupplierAdminCommandLedgerStatusDetectsCorruptDigestAndIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "corrupt-command-ledger.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierAdminCommand{}))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	row := SupplierAdminCommand{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "key", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 1), ResourceType: "activation", ResourceId: 1, ClaimToken: fmt.Sprintf("%032x", 1)}
	require.NoError(t, db.Create(&row).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&SupplierAdminCommand{}).Where("id = ?", row.Id).UpdateColumn("idempotency_key_digest", []byte("wrong")).Error)

	status, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
	require.NoError(t, err)
	require.Equal(t, int64(1), status.InvalidDigestRows)
	require.ErrorIs(t, ValidateSupplierAdminCommandLedgerFinalized(db), ErrSupplierAdminCommandLedgerNotFinalized)

	require.NoError(t, db.Migrator().DropIndex(&SupplierAdminCommand{}, supplierAdminCommandActorScopeDigestIndex))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+supplierAdminCommandActorScopeDigestIndex+" ON supplier_admin_commands (scope, actor_id, idempotency_key_digest)").Error)
	status, err = GetSupplierAdminCommandLedgerMigrationStatus(db)
	require.NoError(t, err)
	require.False(t, status.HasActorDigestIndex)
}

func TestSupplierAdminCommandLedgerStatusRejectsUnsafeSQLiteIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "unsafe-command-ledger.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierAdminCommand{}))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, db.Migrator().DropIndex(&SupplierAdminCommand{}, supplierAdminCommandActorScopeDigestIndex))

	t.Run("partial", func(t *testing.T) {
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+supplierAdminCommandActorScopeDigestIndex+" ON supplier_admin_commands (actor_id, scope, idempotency_key_digest) WHERE actor_id >= 0").Error)
		status, statusErr := GetSupplierAdminCommandLedgerMigrationStatus(db)
		require.NoError(t, statusErr)
		require.False(t, status.HasActorDigestIndex)
		require.ErrorIs(t, ValidateSupplierAdminCommandLedgerFinalized(db), ErrSupplierAdminCommandLedgerNotFinalized)
		require.NoError(t, db.Migrator().DropIndex(&SupplierAdminCommand{}, supplierAdminCommandActorScopeDigestIndex))
	})

	t.Run("expression", func(t *testing.T) {
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+supplierAdminCommandActorScopeDigestIndex+" ON supplier_admin_commands (actor_id, scope, hex(idempotency_key_digest))").Error)
		status, statusErr := GetSupplierAdminCommandLedgerMigrationStatus(db)
		require.NoError(t, statusErr)
		require.False(t, status.HasActorDigestIndex)
		require.ErrorIs(t, ValidateSupplierAdminCommandLedgerFinalized(db), ErrSupplierAdminCommandLedgerNotFinalized)
	})
}

func TestSupplierAdminCommandLedgerMigrationCrossDB(t *testing.T) {
	testCases := []struct {
		name             string
		dsnEnv           string
		expectedDatabase string
		open             func(string) gorm.Dialector
	}{
		{name: "sqlite", open: func(_ string) gorm.Dialector { return sqlite.Open(filepath.Join(t.TempDir(), "cross-db-ledger.db")) }},
		{name: "mysql", dsnEnv: "TEST_MYSQL_DSN", expectedDatabase: "supplier_g009_mysql", open: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", dsnEnv: "TEST_POSTGRES_DSN", expectedDatabase: "supplier_g009_postgres", open: func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
			if testCase.dsnEnv != "" && dsn == "" {
				t.Skipf("set %s to run the isolated %s ledger migration test", testCase.dsnEnv, testCase.name)
			}
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			if testCase.expectedDatabase != "" {
				query := "SELECT DATABASE()"
				if testCase.name == "postgres" {
					query = "SELECT current_database()"
				}
				var current string
				require.NoError(t, db.Raw(query).Scan(&current).Error)
				require.Equal(t, testCase.expectedDatabase, current, "integration test refuses to mutate a non-isolated database")
			}
			require.NoError(t, db.Migrator().DropTable(&SupplierAdminCommand{}))
			require.NoError(t, db.AutoMigrate(&legacySupplierAdminCommand{}))
			rows := []legacySupplierAdminCommand{
				{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "key-a", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 1), ResourceType: "activation", ResourceId: 1, ClaimToken: fmt.Sprintf("%032x", 1)},
				{ActorId: 7, Scope: "activation.transition", IdempotencyKey: "key-b", PayloadVersion: 1, PayloadDigest: fmt.Sprintf("%064x", 2), ResourceType: "activation", ResourceId: 2, ClaimToken: fmt.Sprintf("%032x", 2)},
			}
			require.NoError(t, db.Create(&rows).Error)

			require.NoError(t, MigrateSupplierAdminCommandLedger(db))
			bridge, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
			require.NoError(t, err)
			require.True(t, bridge.LegacyScopeKeyIndex)
			require.True(t, bridge.LegacyActorScopeKeyIndex)
			require.True(t, bridge.HasActorDigestIndex)
			require.False(t, bridge.Finalized)
			require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
			require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
			require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))

			require.NoError(t, db.Migrator().DropIndex(&SupplierAdminCommand{}, supplierAdminCommandActorScopeDigestIndex))
			switch testCase.name {
			case "sqlite":
				require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+supplierAdminCommandActorScopeDigestIndex+" ON supplier_admin_commands (actor_id, scope, idempotency_key_digest) WHERE actor_id >= 0").Error)
			case "mysql":
				require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+supplierAdminCommandActorScopeDigestIndex+" ON supplier_admin_commands (actor_id, scope(8), idempotency_key_digest)").Error)
			case "postgres":
				require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+supplierAdminCommandActorScopeDigestIndex+" ON supplier_admin_commands (actor_id, scope, idempotency_key_digest) WHERE actor_id >= 0").Error)
			}
			unsafeStatus, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
			require.NoError(t, err)
			require.False(t, unsafeStatus.HasActorDigestIndex)
			require.ErrorIs(t, MigrateSupplierAdminCommandLedger(db), ErrSupplierAdminCommandIncomplete, "bridge must not silently replace an unsafe same-name index")
			require.NoError(t, db.Migrator().DropIndex(&SupplierAdminCommand{}, supplierAdminCommandActorScopeDigestIndex))
			require.NoError(t, MigrateSupplierAdminCommandLedger(db))
			require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))

			payload := map[string]any{"state_version": 3}
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				claim, err := ClaimSupplierAdminCommandTx(tx, 8, "activation.transition", "key-a", payload, "activation")
				if err != nil {
					return err
				}
				return CompleteSupplierAdminCommandTx(tx, claim, 3, payload)
			}))
			var count int64
			require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("scope = ? AND idempotency_key = ?", "activation.transition", "key-a").Count(&count).Error)
			require.Equal(t, int64(2), count)
		})
	}
}

func TestSupplierAdminCommandTxClaimOneWinnerReplayConflictAndResult(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-tx-foundation")

	type commandResult struct {
		StateVersion int  `json:"state_version"`
		Enabled      bool `json:"enabled"`
	}
	const callers = 24
	claims := make([]*SupplierAdminCommandClaim, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs[index] = db.Transaction(func(tx *gorm.DB) error {
				claim, err := ClaimSupplierAdminCommandTx(tx, 17, "mutation_gate.transition", "gate-key", map[string]any{"enabled": true}, "mutation_gate")
				if err != nil {
					return err
				}
				claims[index] = claim
				if claim.Replayed {
					var replay commandResult
					return claim.DecodeResult(&replay)
				}
				return CompleteSupplierAdminCommandTx(tx, claim, 1, commandResult{StateVersion: 1, Enabled: true})
			})
		}(i)
	}
	wg.Wait()

	winners := 0
	replays := 0
	for i := range errs {
		require.NoError(t, errs[i])
		require.NotNil(t, claims[i])
		if claims[i].Claimed {
			winners++
		} else {
			replays++
		}
	}
	require.Equal(t, 1, winners)
	require.Equal(t, callers-1, replays)

	var replayed *SupplierAdminCommandClaim
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		replayed, err = ClaimSupplierAdminCommandTx(tx, 17, "mutation_gate.transition", "gate-key", map[string]any{"enabled": true}, "mutation_gate")
		return err
	}))
	require.True(t, replayed.Replayed)
	var decoded commandResult
	require.NoError(t, replayed.DecodeResult(&decoded))
	require.Equal(t, commandResult{StateVersion: 1, Enabled: true}, decoded)

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := ClaimSupplierAdminCommandTx(tx, 17, "mutation_gate.transition", "gate-key", map[string]any{"enabled": false}, "mutation_gate")
		return err
	})
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)

	mixedClaims := make([]*SupplierAdminCommandClaim, callers)
	mixedErrors := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			enabled := index%2 == 0
			mixedErrors[index] = db.Transaction(func(tx *gorm.DB) error {
				claim, err := ClaimSupplierAdminCommandTx(tx, 17, "mutation_gate.transition", "mixed-gate-key", map[string]any{"enabled": enabled}, "mutation_gate")
				if err != nil {
					return err
				}
				mixedClaims[index] = claim
				if claim.Replayed {
					var replay commandResult
					return claim.DecodeResult(&replay)
				}
				return CompleteSupplierAdminCommandTx(tx, claim, 2, commandResult{StateVersion: 2, Enabled: enabled})
			})
		}(i)
	}
	wg.Wait()

	mixedWinners := 0
	mixedReplays := 0
	mixedConflicts := 0
	for i := range mixedErrors {
		if errors.Is(mixedErrors[i], ErrSupplierAdminIdempotencyConflict) {
			mixedConflicts++
			continue
		}
		require.NoError(t, mixedErrors[i])
		require.NotNil(t, mixedClaims[i])
		if mixedClaims[i].Claimed {
			mixedWinners++
		} else {
			mixedReplays++
		}
	}
	require.Equal(t, 1, mixedWinners)
	require.Equal(t, callers/2-1, mixedReplays)
	require.Equal(t, callers/2, mixedConflicts)
}

func TestGetSupplierAdminCommandResultCoversInventoryAndExclusionLedgers(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-append-ledgers")
	contract := createSupplierContractFixture(t, db, "append result supplier", "append-result-contract")
	adjustment := &SupplierInventoryAdjustment{
		ContractId: contract.Id, DeltaMicroUsd: 200_000_000, Type: SupplierInventoryAdjustmentTypeReplenishment,
		IdempotencyKey: "inventory-result-key", CreatedBy: 7,
	}
	require.NoError(t, db.Create(adjustment).Error)
	rule, err := CreateSupplierStatisticsExclusionRule(99, SupplierStatisticsActionExclude, 7, "company account", "exclusion-result-key")
	require.NoError(t, err)

	inventoryScope := SupplierInventoryCommandScope(contract.Id)
	inventoryResult, err := GetSupplierAdminCommandResult(7, inventoryScope, "inventory-result-key")
	require.NoError(t, err)
	require.Equal(t, inventoryScope, inventoryResult.Scope)
	require.Equal(t, supplierAdminCommandResourceInventory, inventoryResult.ResourceType)
	require.Equal(t, adjustment.Id, inventoryResult.ResourceId)

	exclusionResult, err := GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateExclusion, "exclusion-result-key")
	require.NoError(t, err)
	require.Equal(t, supplierAdminCommandResourceExclusion, exclusionResult.ResourceType)
	require.Equal(t, rule.Id, exclusionResult.ResourceId)

	_, err = GetSupplierAdminCommandResult(7, SupplierInventoryCommandScope(contract.Id+1), "inventory-result-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetSupplierAdminCommandResult(8, inventoryScope, "inventory-result-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetSupplierAdminCommandResult(8, SupplierAdminCommandScopeCreateExclusion, "exclusion-result-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	for _, malformed := range []string{SupplierAdminCommandScopeCreateInventory, SupplierAdminCommandScopeCreateInventory + "/0", SupplierAdminCommandScopeCreateInventory + "/01", SupplierAdminCommandScopeCreateInventory + "/not-a-number"} {
		_, err = GetSupplierAdminCommandResult(7, malformed, "inventory-result-key")
		require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired, malformed)
	}
}

func TestGetSupplierAdminCommandResultCoversCommandLedgerScopes(t *testing.T) {
	setupSupplierTestDB(t, "supplier-admin-command-result-ledger-scopes")
	supplier, _, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "all scopes supplier"}, "all-scopes-supplier", 7)
	require.NoError(t, err)
	contract, _, err := CreateSupplierContractIdempotentForActor(&SupplierContract{SupplierId: supplier.Id, Name: "all scopes contract", ContractNo: "all-scopes-contract"}, "all-scopes-contract", 7)
	require.NoError(t, err)
	rate, _, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 7, "initial rate", "all-scopes-rate")
	require.NoError(t, err)

	tests := []struct {
		scope        string
		key          string
		resourceType string
		resourceId   int
	}{
		{SupplierAdminCommandScopeCreateSupplier, "all-scopes-supplier", supplierAdminCommandResourceSupplier, supplier.Id},
		{SupplierAdminCommandScopeCreateContract, "all-scopes-contract", supplierAdminCommandResourceContract, contract.Id},
		{SupplierAdminCommandScopeCreateRate, "all-scopes-rate", supplierAdminCommandResourceRate, rate.Id},
	}
	for _, test := range tests {
		result, err := GetSupplierAdminCommandResult(7, test.scope, test.key)
		require.NoError(t, err, test.scope)
		require.Equal(t, test.resourceType, result.ResourceType)
		require.Equal(t, test.resourceId, result.ResourceId)
	}
}
