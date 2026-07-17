package model

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRecallAttributionCandidateDiscoveryIsBoundedAndAdvancesCursor(t *testing.T) {
	db := setupRecallAttributionDiscoveryTestDB(t)
	const (
		limit          = 2
		maxScannedRows = 8 * limit
		orderCount     = 64
		nowUnix        = int64(1_700_000_300)
	)

	progress := make([]RecallEvent, 0, maxScannedRows)
	for i := 1; i <= orderCount; i++ {
		candidate := createRecallAttributionDiscoveryOrder(t, i, nowUnix+int64(i))
		if i <= maxScannedRows {
			progress = append(progress, RecallEvent{
				EventType: recallAttributionProgressTerminal, Source: recallAttributionProgressSource,
				SourceEventId: recallAttributionSourceEventID(candidate), EventData: `{"attempt":1}`, CreatedAt: nowUnix,
			})
		}
	}
	require.NoError(t, DB.Create(&progress).Error)

	stats := &recallAttributionQueryStats{}
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:recall_attribution_bounded_discovery", stats.observe))
	t.Cleanup(func() {
		db.Callback().Query().Remove("test:recall_attribution_bounded_discovery")
	})

	first, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, limit)
	require.NoError(t, err)
	require.Empty(t, first, "one batch must stop after its bounded scan instead of walking all history")
	require.LessOrEqual(t, stats.discoveryRows, int64(maxScannedRows))
	require.LessOrEqual(t, stats.maxVariables, 32, "candidate discovery must not expand user IDs into an unbounded IN list")

	stats.reset()
	second, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, limit)
	require.NoError(t, err)
	require.Len(t, second, limit)
	require.Equal(t, "trade_discovery_17", second[0].TradeNo)
	require.LessOrEqual(t, stats.discoveryRows, int64(maxScannedRows))
	require.LessOrEqual(t, stats.maxVariables, 32)
}

func TestRecallAttributionCandidateCursorWrapsToRevisitDueRetry(t *testing.T) {
	setupRecallAttributionDiscoveryTestDB(t)
	const nowUnix = int64(1_700_000_300)
	firstOrder := createRecallAttributionDiscoveryOrder(t, 1, nowUnix+1)
	secondOrder := createRecallAttributionDiscoveryOrder(t, 2, nowUnix+2)

	first, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, 1)
	require.NoError(t, err)
	require.Equal(t, []RecallAttributionCandidate{firstOrder}, first)
	firstLease, acquired, err := LeaseRecallAttributionCandidateWithContext(context.Background(), firstOrder, nowUnix, nowUnix+900)
	require.NoError(t, err)
	require.True(t, acquired)
	updated, err := RetryRecallAttributionCandidateWithContext(context.Background(), firstOrder, firstLease, nowUnix+60, "test_retry")
	require.NoError(t, err)
	require.True(t, updated)

	second, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, 1)
	require.NoError(t, err)
	require.Equal(t, []RecallAttributionCandidate{secondOrder}, second)
	secondLease, acquired, err := LeaseRecallAttributionCandidateWithContext(context.Background(), secondOrder, nowUnix, nowUnix+900)
	require.NoError(t, err)
	require.True(t, acquired)
	updated, err = CompleteRecallAttributionCandidateWithContext(context.Background(), secondOrder, secondLease, nowUnix, "test_terminal")
	require.NoError(t, err)
	require.True(t, updated)

	none, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, 1)
	require.NoError(t, err)
	require.Empty(t, none)
	cursorEvent := RecallEvent{}
	require.NoError(t, DB.Where("source = ? AND source_event_id = ? AND event_type = ?", recallAttributionProgressSource, "cursor:v1", "reconciliation_cursor").First(&cursorEvent).Error)
	require.NotContains(t, cursorEvent.EventData, firstOrder.TradeNo)
	require.NotContains(t, cursorEvent.EventData, firstOrder.CheckoutSessionId)

	due, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix+120, 1)
	require.NoError(t, err)
	require.Equal(t, []RecallAttributionCandidate{firstOrder}, due, "cursor wrap must eventually revisit retry work after it becomes due")
}

func TestRecallAttributionCandidateDiscoveryHandlesLargeLimitWithoutOverflow(t *testing.T) {
	setupRecallAttributionDiscoveryTestDB(t)
	const nowUnix = int64(1_700_000_300)
	want := createRecallAttributionDiscoveryOrder(t, 1, nowUnix+1)

	got, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, int(^uint(0)>>1))
	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.Equal(t, want, got[0])
}

func TestRecallAttributionCandidateDiscoveryDoesNotDuplicateCandidatesAfterCursorWrap(t *testing.T) {
	setupRecallAttributionDiscoveryTestDB(t)
	const nowUnix = int64(1_700_000_300)
	want := createRecallAttributionDiscoveryOrder(t, 1, nowUnix+1)

	got, err := ListRecallAttributionCandidatesWithContext(context.Background(), nowUnix, 2)
	require.NoError(t, err)
	require.Equal(t, []RecallAttributionCandidate{want}, got)
}

func TestRecallAttributionOrderPageSQLSupportsAllDatabaseDialects(t *testing.T) {
	silentLogger := logger.Default.LogMode(logger.Silent)
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: silentLogger})
	require.NoError(t, err)
	sqlDB, err := sqliteDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mysqlDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, Logger: silentLogger,
	})
	require.NoError(t, err)
	postgresDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, Logger: silentLogger,
	})
	require.NoError(t, err)

	dialects := map[string]*gorm.DB{
		"sqlite":     sqliteDB.Session(&gorm.Session{DryRun: true}),
		"mysql":      mysqlDB,
		"postgresql": postgresDB,
	}
	for name, dialectDB := range dialects {
		for _, phase := range []string{recallAttributionPhaseSubscription, recallAttributionPhaseTopUp} {
			t.Run(name+"/"+phase, func(t *testing.T) {
				generatedSQL := strings.ToLower(dialectDB.ToSQL(func(tx *gorm.DB) *gorm.DB {
					rows := make([]recallAttributionOrderRow, 0)
					return recallAttributionOrderPageQueryWithContext(context.Background(), tx, recallAttributionCursor{Phase: phase}, 10).Find(&rows)
				}))
				require.Contains(t, generatedSQL, "select min(recall_recipients.created_at)")
				require.Contains(t, generatedSQL, "recall_recipients.user_id = recall_orders.user_id")
				require.Contains(t, generatedSQL, "recall_orders.create_time")
				if phase == recallAttributionPhaseTopUp {
					require.Contains(t, generatedSQL, "left join subscription_orders")
				}
			})
		}
	}
}

func TestRecallAttributionCursorCASDoesNotOverwriteConcurrentAdvance(t *testing.T) {
	setupRecallAttributionDiscoveryTestDB(t)
	const nowUnix = int64(1_700_000_300)
	_, expectedData, err := loadRecallAttributionCursorWithContext(context.Background(), nowUnix)
	require.NoError(t, err)
	advanced := recallAttributionCursor{Phase: recallAttributionPhaseTopUp, OrderCreatedAt: nowUnix + 20, OrderId: 20}
	require.NoError(t, storeRecallAttributionCursorWithContext(context.Background(), expectedData, advanced, nowUnix+1))

	stale := recallAttributionCursor{Phase: recallAttributionPhaseSubscription, OrderCreatedAt: nowUnix + 10, OrderId: 10}
	require.NoError(t, storeRecallAttributionCursorWithContext(context.Background(), expectedData, stale, nowUnix+2))
	stored, _, err := loadRecallAttributionCursorWithContext(context.Background(), nowUnix+3)
	require.NoError(t, err)
	require.Equal(t, advanced, stored)
}

type recallAttributionQueryStats struct {
	discoveryRows int64
	maxVariables  int
}

func (s *recallAttributionQueryStats) observe(tx *gorm.DB) {
	sql := strings.ToLower(tx.Statement.SQL.String())
	if !strings.Contains(sql, "recall_recipients") && !strings.Contains(sql, "subscription_orders") && !strings.Contains(sql, "top_ups") {
		return
	}
	s.discoveryRows += tx.RowsAffected
	if len(tx.Statement.Vars) > s.maxVariables {
		s.maxVariables = len(tx.Statement.Vars)
	}
}

func (s *recallAttributionQueryStats) reset() {
	s.discoveryRows = 0
	s.maxVariables = 0
}

func setupRecallAttributionDiscoveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, _ := setupRecallRepositoryTestDB(t)
	require.NoError(t, DB.AutoMigrate(&TopUp{}, &SubscriptionOrder{}))
	return db
}

func createRecallAttributionDiscoveryOrder(t *testing.T, index int, createdAt int64) RecallAttributionCandidate {
	t.Helper()
	userID := 10_000 + index
	tradeNo := fmt.Sprintf("trade_discovery_%d", index)
	sessionID := fmt.Sprintf("cs_discovery_%d", index)
	enrolledAt := createdAt - 100
	require.NoError(t, DB.Create(&RecallRecipient{
		CampaignId: int64(index), UserId: userID, EligibilitySnapshot: `{}`,
		EmailSnapshot: fmt.Sprintf("user-%d@example.com", index), LanguageSnapshot: "en",
		State: RecallRecipientCodeReady, CreatedAt: enrolledAt,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: userID, TradeNo: tradeNo, GatewayTradeNo: sessionID,
		PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess,
		CreateTime: createdAt, CompleteTime: createdAt + 1,
	}).Error)
	return RecallAttributionCandidate{
		TradeNo: tradeNo, UserId: userID, CheckoutSessionId: sessionID,
		OrderCreatedAt: createdAt, EnrolledAt: enrolledAt,
	}
}
