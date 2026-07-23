package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSupplierBatchProtocolCrossDBMatrix(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		t.Run("shared_log_db", func(t *testing.T) {
			db := supplierDailyTestDB(t, t.Name())
			runSupplierBatchProtocolMatrix(t, db, db, false)
		})
		t.Run("separate_log_db", func(t *testing.T) {
			mainDB := supplierDailyTestDB(t, t.Name()+"-main")
			logDB := supplierDailyTestDB(t, t.Name()+"-log")
			runSupplierBatchProtocolMatrix(t, mainDB, logDB, true)
		})
	})

	external := []struct {
		name, dialect, mainEnv, logEnv, mainDatabase, logDatabase string
		open                                                      func(string) gorm.Dialector
	}{
		{name: "mysql", dialect: "mysql", mainEnv: "TEST_MYSQL_DSN", logEnv: "TEST_MYSQL_LOG_DSN", mainDatabase: "supplier_g009_mysql", logDatabase: "supplier_g003_mysql_log", open: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", dialect: "postgres", mainEnv: "TEST_POSTGRES_DSN", logEnv: "TEST_POSTGRES_LOG_DSN", mainDatabase: "supplier_g009_postgres", logDatabase: "supplier_g003_postgres_log", open: func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
	}
	for _, testCase := range external {
		t.Run(testCase.name, func(t *testing.T) {
			mainDSN := strings.TrimSpace(os.Getenv(testCase.mainEnv))
			if mainDSN == "" {
				t.Skipf("set %s to run the isolated %s supplier batch matrix", testCase.mainEnv, testCase.name)
			}
			t.Run("shared_log_db", func(t *testing.T) {
				db, err := gorm.Open(testCase.open(mainDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				requireIsolatedSupplierDatabase(t, db, testCase.dialect, testCase.mainDatabase)
				runSupplierBatchProtocolMatrix(t, db, db, false)
			})
			t.Run("separate_log_db", func(t *testing.T) {
				logDSN := strings.TrimSpace(os.Getenv(testCase.logEnv))
				if logDSN == "" {
					t.Skipf("set %s for the physically separate %s LOG_DB matrix", testCase.logEnv, testCase.name)
				}
				mainDB, err := gorm.Open(testCase.open(mainDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				logDB, err := gorm.Open(testCase.open(logDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				require.NoError(t, err)
				requireIsolatedSupplierDatabase(t, mainDB, testCase.dialect, testCase.mainDatabase)
				requireIsolatedSupplierDatabase(t, logDB, testCase.dialect, testCase.logDatabase)
				runSupplierBatchProtocolMatrix(t, mainDB, logDB, true)
			})
		})
	}
}

func runSupplierBatchProtocolMatrix(t *testing.T, mainDB, logDB *gorm.DB, separateLogDB bool) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, mainDB.AutoMigrate(
		&model.Option{}, &model.SupplierAccountingCoverageGap{}, &model.SupplierUsageDailySummary{},
		&model.SupplierUsageDailyBatchRun{}, &model.SupplierAdminCommand{},
	))
	require.NoError(t, model.MigrateSupplierAdminCommandLedger(mainDB))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	cleanSupplierBatchProtocolMatrix(t, mainDB, logDB, separateLogDB)
	runSupplierDistinctOwnerAcquisitionRace(t, mainDB)
	cleanSupplierBatchProtocolMatrix(t, mainDB, logDB, separateLogDB)
	runSupplierMaxLengthRerunOwnerAcquisition(t, mainDB)
	cleanSupplierBatchProtocolMatrix(t, mainDB, logDB, separateLogDB)

	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	firstDay := beginningOfSupplierDay(now).AddDate(0, 0, -2)
	activateSupplierAccountingForBatch(t, mainDB, firstDay.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: firstDay.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: fmt.Sprintf("matrix-%s-%t", mainDB.Dialector.Name(), separateLogDB), AuditSlot: dto.SupplierBatchAuditSlotCurrent}

	preparationFailureName := "test:matrix_preparation_failure:" + t.Name()
	var preparationFailureOnce sync.Once
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(preparationFailureName, func(tx *gorm.DB) {
		if supplierBatchCallbackState(tx) == types.SupplierBatchCommandStateRunning {
			preparationFailureOnce.Do(func() { tx.AddError(errors.New("forced matrix running-state failure")) })
		}
	}))
	_, err = CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-preparation-failure"}, now)
	require.ErrorContains(t, err, "forced matrix running-state failure")
	require.NoError(t, mainDB.Callback().Update().Remove(preparationFailureName))
	var failedPreparationCommands, failedPreparationRuns int64
	require.NoError(t, mainDB.Model(&model.SupplierAdminCommand{}).Where("idempotency_key = ?", "matrix-preparation-failure").Count(&failedPreparationCommands).Error)
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&failedPreparationRuns).Error)
	require.Zero(t, failedPreparationCommands, "claim must roll back when the running-state store fails")
	require.Zero(t, failedPreparationRuns, "date materialization and lease acquisition must roll back with the claim")

	const concurrentCallers = 24
	preparationReached := make(chan struct{})
	releasePreparation := make(chan struct{})
	visibilityCallbackName := "test:matrix_running_visibility:" + t.Name()
	var visibilityOnce sync.Once
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(visibilityCallbackName, func(tx *gorm.DB) {
		if supplierBatchCallbackState(tx) != types.SupplierBatchCommandStateRunning {
			return
		}
		visibilityOnce.Do(func() {
			close(preparationReached)
			<-releasePreparation
		})
	}))
	results := make(chan supplierBatchAsyncResult, concurrentCallers)
	go func() {
		response, runErr := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-first"}, now)
		results <- supplierBatchAsyncResult{response: response, err: runErr}
	}()
	select {
	case <-preparationReached:
	case <-time.After(5 * time.Second):
		require.Fail(t, "first acquisition did not reach running-state store")
	}
	visibilityResult := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, getErr := getSupplierDailyBatchRequestStatus(ctx, mainDB, logDB, principal, "matrix-first", now)
		visibilityResult <- supplierBatchAsyncResult{response: response, err: getErr}
	}()
	for range concurrentCallers - 1 {
		go func() {
			response, runErr := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-first"}, now)
			results <- supplierBatchAsyncResult{response: response, err: runErr}
		}()
	}
	var earlyVisibility *supplierBatchAsyncResult
	select {
	case visible := <-visibilityResult:
		require.ErrorIs(t, visible.err, ErrSupplierBatchRequestNotFound, "an uncommitted claim must not expose a partial claimed/running state")
		earlyVisibility = &visible
	case <-time.After(50 * time.Millisecond):
		// A blocked read is also valid: it cannot observe the partially prepared transaction.
	}
	close(releasePreparation)
	var completedResponses []dto.SupplierBatchStatusResponse
	for range concurrentCallers {
		result := <-results
		if result.err != nil {
			require.ErrorIs(t, result.err, ErrSupplierBatchBusy)
			continue
		}
		require.NoError(t, result.response.Validate())
		if result.response.Status == dto.SupplierBatchStatusCompleted {
			completedResponses = append(completedResponses, result.response)
		} else {
			require.Equal(t, dto.SupplierBatchStatusRunning, result.response.Status)
		}
	}
	require.NotEmpty(t, completedResponses, "one concurrent caller must own and complete the first acquisition")
	first, err := getSupplierDailyBatchRequestStatus(ctx, mainDB, logDB, principal, "matrix-first", now)
	require.NoError(t, err)
	for _, completed := range completedResponses {
		require.Equal(t, first, completed)
	}
	if earlyVisibility == nil {
		select {
		case visible := <-visibilityResult:
			if visible.err == nil {
				if visible.response.Status == dto.SupplierBatchStatusCompleted {
					require.Equal(t, first, visible.response)
				} else {
					require.Equal(t, dto.SupplierBatchStatusRunning, visible.response.Status)
				}
			} else {
				require.ErrorIs(t, visible.err, ErrSupplierBatchRequestNotFound)
			}
		case <-time.After(5 * time.Second):
			require.Fail(t, "visibility read remained blocked after preparation committed")
		}
	}
	require.NoError(t, mainDB.Callback().Update().Remove(visibilityCallbackName))
	require.Equal(t, firstDay.Format("2006-01-02"), *first.BatchDate)
	firstRun, firstEvidence, err := model.LoadSupplierPublishedDailyBatch(ctx, mainDB, *first.BatchDate)
	require.NoError(t, err)
	require.Equal(t, "incomplete", firstEvidence.PersistedLogSnapshotCompleteness)
	identityDigest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	require.NoError(t, err)
	firstCommand, err := model.GetSupplierBatchSchedulerCommand(ctx, mainDB, identityDigest, "matrix-first")
	require.NoError(t, err)
	storedFirst, err := supplierBatchCommandResponseToDTO(firstCommand.State)
	require.NoError(t, err)
	require.Equal(t, first, storedFirst, "zero-amount publication and its terminal command must commit atomically")
	var firstDateRuns int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Where("batch_date = ?", firstDay.Format("2006-01-02")).Count(&firstDateRuns).Error)
	require.EqualValues(t, 1, firstDateRuns, "concurrent first acquisition must have one durable batch owner")

	second, err := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-second"}, now)
	require.NoError(t, err)
	require.Equal(t, firstDay.AddDate(0, 0, 1).Format("2006-01-02"), *second.BatchDate, "exact-zero selection must advance past an incomplete publication")
	noWork, err := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-no-work"}, now)
	require.NoError(t, err)
	require.Nil(t, noWork.BatchDate)
	require.Equal(t, &dto.SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: false}, noWork.Result)

	firstReplay, err := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-first"}, now)
	require.NoError(t, err)
	require.Equal(t, first, firstReplay, "a lost first response must replay the exact committed terminal DTO")
	seedSupplierSchedulerRunningCommand(t, mainDB, principal, "matrix-legacy-crash-recovery", firstRun.BatchDate, int64(firstRun.Id), firstRun.PublishedFenceToken, now.Add(-time.Minute))
	reconciled, err := getSupplierDailyBatchRequestStatus(ctx, mainDB, logDB, principal, "matrix-legacy-crash-recovery", now)
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, reconciled.Status)

	scanReached := make(chan struct{})
	var scanOnce sync.Once
	cancelScanName := "test:matrix_cancel_scan:" + t.Name()
	cancelScan := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "logs" {
			return
		}
		scanOnce.Do(func() { close(scanReached) })
		<-tx.Statement.Context.Done()
		tx.AddError(tx.Statement.Context.Err())
	}
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(cancelScanName+":query", cancelScan))
	require.NoError(t, logDB.Callback().Row().Before("gorm:row").Register(cancelScanName+":row", cancelScan))
	require.NoError(t, logDB.Callback().Raw().Before("gorm:raw").Register(cancelScanName+":raw", cancelScan))
	cancelCtx, cancel := context.WithCancel(ctx)
	cancelResult := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, runErr := RerunSupplierDailyReport(cancelCtx, mainDB, logDB, 91, firstRun.BatchDate, "matrix-cancelled-read", dto.SupplierDailyReportRerunRequest{
			Reason: "cross-db cancelled read", ExpectedPublishedFenceToken: firstRun.PublishedFenceToken,
		}, now.Add(time.Minute))
		cancelResult <- supplierBatchAsyncResult{response: response, err: runErr}
	}()
	select {
	case <-scanReached:
	case early := <-cancelResult:
		require.Failf(t, "matrix cancellation ended before scanning", "response=%+v err=%v", early.response, early.err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "matrix cancellation did not reach log scan")
	}
	cancel()
	cancelled := <-cancelResult
	require.NoError(t, cancelled.err)
	require.Equal(t, dto.SupplierBatchStatusFailed, cancelled.response.Status)
	require.NoError(t, logDB.Callback().Query().Remove(cancelScanName+":query"))
	require.NoError(t, logDB.Callback().Row().Remove(cancelScanName+":row"))
	require.NoError(t, logDB.Callback().Raw().Remove(cancelScanName+":raw"))

	require.NoError(t, logDB.Migrator().DropTable(&model.Log{}))
	readFailed, err := RerunSupplierDailyReport(ctx, mainDB, logDB, 91, firstRun.BatchDate, "matrix-read-failure", dto.SupplierDailyReportRerunRequest{
		Reason: "cross-db read failure", ExpectedPublishedFenceToken: firstRun.PublishedFenceToken,
	}, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusFailed, readFailed.Status)
	retained, _, err := model.LoadSupplierPublishedDailyBatch(ctx, mainDB, firstRun.BatchDate)
	require.NoError(t, err)
	require.Equal(t, firstRun.PublishedFenceToken, retained.PublishedFenceToken)

	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	valid := supplierDailySnapshot(firstDay, 700_000)
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: firstDay.Add(30 * time.Minute).Unix(), Other: `{}`}).Error)
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: firstDay.Add(time.Hour).Unix(), ChannelId: 4, ModelName: "matrix-rerun", Other: supplierDailyLogOther(t, valid)}).Error)
	if separateLogDB {
		callbackName := "test:force_matrix_publication_failure:" + t.Name()
		require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
			updates, ok := tx.Statement.Dest.(map[string]any)
			if ok {
				if _, publishing := updates["published_fence_token"]; publishing {
					tx.AddError(errors.New("forced matrix main-db publication failure"))
				}
			}
		}))
		_, rerunErr := RerunSupplierDailyReport(ctx, mainDB, logDB, 91, firstRun.BatchDate, "matrix-publication-failure", dto.SupplierDailyReportRerunRequest{
			Reason: "cross-db publication failure", ExpectedPublishedFenceToken: firstRun.PublishedFenceToken,
		}, now.Add(2*time.Minute))
		require.NoError(t, mainDB.Callback().Update().Remove(callbackName))
		require.ErrorContains(t, rerunErr, "forced matrix main-db publication failure")
		retained, _, err = model.LoadSupplierPublishedDailyBatch(ctx, mainDB, firstRun.BatchDate)
		require.NoError(t, err)
		require.Equal(t, firstRun.PublishedFenceToken, retained.PublishedFenceToken)
	}

	maxLengthRerunKey := strings.Repeat("matrix-k", 16)
	require.Len(t, maxLengthRerunKey, 128)
	succeeded, err := RerunSupplierDailyReport(ctx, mainDB, logDB, 91, firstRun.BatchDate, maxLengthRerunKey, dto.SupplierDailyReportRerunRequest{
		Reason: "cross-db replacement", ExpectedPublishedFenceToken: firstRun.PublishedFenceToken,
	}, now.Add(3*time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, succeeded.Status)
	require.Greater(t, succeeded.PublishedFenceToken, firstRun.PublishedFenceToken)
	nextSucceeded, err := RerunSupplierDailyReport(ctx, mainDB, logDB, 91, firstRun.BatchDate, "matrix-success-next", dto.SupplierDailyReportRerunRequest{
		Reason: "cross-db second replacement", ExpectedPublishedFenceToken: succeeded.PublishedFenceToken,
	}, now.Add(4*time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, nextSucceeded.Status)
	require.Greater(t, nextSucceeded.PublishedFenceToken, succeeded.PublishedFenceToken)
	succeededReplay, err := RerunSupplierDailyReport(ctx, mainDB, logDB, 91, firstRun.BatchDate, maxLengthRerunKey, dto.SupplierDailyReportRerunRequest{
		Reason: "cross-db replacement", ExpectedPublishedFenceToken: firstRun.PublishedFenceToken,
	}, now.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, succeeded, succeededReplay, "a later rerun fence must not rewrite an earlier completed rerun terminal response")
	firstAfterReruns, err := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "matrix-first"}, now.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, first, firstAfterReruns, "later Root rerun fences must not rewrite the scheduler's lost-response replay")
	cancelledReplay, err := RerunSupplierDailyReport(ctx, mainDB, logDB, 91, firstRun.BatchDate, "matrix-cancelled-read", dto.SupplierDailyReportRerunRequest{
		Reason: "cross-db cancelled read", ExpectedPublishedFenceToken: firstRun.PublishedFenceToken,
	}, now.Add(6*time.Minute))
	require.NoError(t, err)
	require.Equal(t, cancelled.response, cancelledReplay, "later replacement fences must not rewrite a canceled rerun terminal response")
}

func runSupplierMaxLengthRerunOwnerAcquisition(t *testing.T, mainDB *gorm.DB) {
	t.Helper()
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 19, 0, 0, 0, 0, location)
	now := day.Add(12 * time.Hour)
	idempotencyKey := strings.Repeat("max-key-", 16)
	require.Len(t, idempotencyKey, 128)
	owner := supplierDailyReportRerunLeaseOwner(91, idempotencyKey)
	require.LessOrEqual(t, len(owner), 128)
	require.NotContains(t, owner, idempotencyKey)

	lease, err := model.AcquireSupplierDailyBatch(context.Background(), mainDB, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), owner, now, supplierDailyLeaseDuration, false)
	require.NoError(t, err)
	require.Equal(t, owner, lease.Owner)
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.First(&run, lease.RunId).Error)
	require.Equal(t, owner, run.LeaseOwner)
}

func runSupplierDistinctOwnerAcquisitionRace(t *testing.T, mainDB *gorm.DB) {
	t.Helper()
	const owners = 24
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	now := day.Add(12 * time.Hour)
	start := make(chan struct{})
	type acquisitionResult struct {
		lease model.SupplierDailyBatchLease
		err   error
	}
	results := make(chan acquisitionResult, owners)
	for owner := range owners {
		go func(owner int) {
			<-start
			lease, acquireErr := model.AcquireSupplierDailyBatch(context.Background(), mainDB, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), fmt.Sprintf("matrix-owner-%02d", owner), now, supplierDailyLeaseDuration, false)
			results <- acquisitionResult{lease: lease, err: acquireErr}
		}(owner)
	}
	close(start)
	winners := 0
	busy := 0
	var winningLease model.SupplierDailyBatchLease
	for range owners {
		result := <-results
		if result.err == nil {
			winners++
			winningLease = result.lease
			continue
		}
		require.ErrorIs(t, result.err, model.ErrSupplierDailyBatchBusy)
		busy++
	}
	require.Equal(t, 1, winners, "one distributed owner must win the first acquisition")
	require.Equal(t, owners-1, busy, "every losing distributed owner must receive controlled busy")
	require.NotZero(t, winningLease.FenceToken)
	var runs []model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).Find(&runs).Error)
	require.Len(t, runs, 1)
	require.Equal(t, model.SupplierDailyBatchStatusRunning, runs[0].Status)
	require.Equal(t, winningLease.Owner, runs[0].LeaseOwner)
}

func supplierBatchCallbackState(tx *gorm.DB) string {
	if tx == nil || tx.Statement == nil || tx.Statement.Table != "supplier_admin_commands" {
		return ""
	}
	updates, ok := tx.Statement.Dest.(map[string]any)
	if !ok {
		return ""
	}
	statusJSON, ok := updates["status_json"].(string)
	if !ok {
		return ""
	}
	state, err := types.ParseSupplierBatchCommandStateV1(statusJSON)
	if err != nil {
		return ""
	}
	return state.State
}

func cleanSupplierBatchProtocolMatrix(t *testing.T, mainDB, logDB *gorm.DB, separateLogDB bool) {
	t.Helper()
	for _, value := range []any{&model.SupplierAdminCommand{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}, &model.SupplierAccountingCoverageGap{}, &model.Option{}} {
		require.NoError(t, mainDB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(value).Error)
	}
	require.NoError(t, logDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Log{}).Error)
	if !separateLogDB && mainDB != logDB {
		require.NoError(t, mainDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Log{}).Error)
	}
}
