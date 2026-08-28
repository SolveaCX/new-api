package model

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStripePaymentCardObservationTest(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "stripe-card-observation.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, DB.AutoMigrate(&User{}, &StripeBonusClaim{}, &StripePaymentCardObservation{}, &StripePaymentCardBackfillLease{}))
	t.Cleanup(func() {
		_ = sqlDB.Close()
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})
}

func TestObserveStripePaymentCardIsIdempotentByCharge(t *testing.T) {
	setupStripePaymentCardObservationTest(t)
	require.NoError(t, DB.Create(&User{Id: 101, Username: "card-user-101", Email: "card-101@example.com"}).Error)

	created, err := ObserveStripePaymentCard(&StripePaymentCardObservation{
		UserId:      101,
		ChargeId:    "ch_idempotent",
		Fingerprint: "fp_idempotent",
		Brand:       "visa",
		Last4:       "4242",
	})
	require.NoError(t, err)
	require.True(t, created)

	created, err = ObserveStripePaymentCard(&StripePaymentCardObservation{
		UserId:      101,
		ChargeId:    "ch_idempotent",
		Fingerprint: "fp_idempotent",
		Brand:       "visa",
		Last4:       "4242",
	})
	require.NoError(t, err)
	require.False(t, created)

	var count int64
	require.NoError(t, DB.Model(&StripePaymentCardObservation{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	var observation StripePaymentCardObservation
	require.NoError(t, DB.First(&observation, "charge_id = ?", "ch_idempotent").Error)
	require.Equal(t, StripeCardRewardDecisionFirstExactCard, observation.RewardDecision)
	require.Equal(t, 101, observation.RewardClaimUserId)

	created, err = ObserveStripePaymentCard(&StripePaymentCardObservation{
		UserId:            101,
		TradeNo:           "trade-enriched",
		CheckoutSessionId: "cs_enriched",
		PaymentIntentId:   "pi_enriched",
		ChargeId:          "ch_idempotent",
		Fingerprint:       "fp_idempotent",
	})
	require.NoError(t, err)
	require.False(t, created)
	require.NoError(t, DB.First(&observation, "charge_id = ?", "ch_idempotent").Error)
	require.Equal(t, "trade-enriched", observation.TradeNo)
	require.Equal(t, "cs_enriched", observation.CheckoutSessionId)

	withhold, claimUserId, err := ShouldWithholdRewardForStripeFingerprint(102, "fp_idempotent")
	require.NoError(t, err)
	require.True(t, withhold)
	require.Equal(t, 101, claimUserId)
	withhold, claimUserId, err = ShouldWithholdRewardForStripeFingerprint(101, "fp_idempotent")
	require.NoError(t, err)
	require.False(t, withhold)
	require.Equal(t, 101, claimUserId)
}

func TestGetStripeCardRiskGroupsUsesOnlyExactFingerprintAcrossUsers(t *testing.T) {
	setupStripePaymentCardObservationTest(t)
	for _, user := range []User{
		{Id: 201, Username: "card-user-201", Email: "first@example.com", AffCode: "card201"},
		{Id: 202, Username: "card-user-202", Email: "second@example.com", AffCode: "card202"},
		{Id: 203, Username: "card-user-203", Email: "third@example.com", AffCode: "card203"},
	} {
		require.NoError(t, DB.Create(&user).Error)
	}

	observations := []*StripePaymentCardObservation{
		{UserId: 201, ChargeId: "ch_shared_1", PaymentMethodId: "pm_shared_1", StripeCustomerId: "cus_shared_1", Fingerprint: "fp_shared", Brand: "visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Country: "us"},
		{UserId: 202, ChargeId: "ch_shared_2", PaymentMethodId: "pm_shared_2", StripeCustomerId: "cus_shared_2", Fingerprint: "fp_shared", Brand: "visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Country: "us"},
		// Same display attributes but a different exact fingerprint form a secondary,
		// human-review-only suspected group; they never become an exact enforcement match.
		{UserId: 203, ChargeId: "ch_other", PaymentMethodId: "pm_other", StripeCustomerId: "cus_other", Fingerprint: "fp_other", Brand: "visa", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Country: "us"},
	}
	for i, observation := range observations {
		observation.TradeNo = fmt.Sprintf("trade-%d", i)
		created, err := ObserveStripePaymentCard(observation)
		require.NoError(t, err)
		require.True(t, created)
	}

	groups, err := GetStripeCardRiskGroups(100)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, StripeCardRiskMatchExactFingerprint, groups[0].MatchBasis)
	require.NotEqual(t, "fp_shared", groups[0].FingerprintDigest)
	require.Equal(t, 2, groups[0].UserCount)
	require.Equal(t, 2, groups[0].ChargeCount)
	require.Len(t, groups[0].Users, 2)
	require.Equal(t, StripeCardRewardDecisionDuplicateExactCard, groups[0].Users[0].RewardDecision)
	require.Equal(t, "***@example.com", groups[0].Users[0].Email)

	require.Equal(t, StripeCardRiskMatchSuspectedAttributes, groups[1].MatchBasis)
	require.Equal(t, 2, groups[1].FingerprintCount)
	require.Equal(t, 3, groups[1].UserCount)
	require.Len(t, groups[1].Users, 3)
	withhold, _, err := ShouldWithholdRewardForStripeFingerprint(203, "fp_shared")
	require.NoError(t, err)
	require.True(t, withhold, "only the exact fingerprint remains an enforcement signal")
	withhold, _, err = ShouldWithholdRewardForStripeFingerprint(201, "fp_other")
	require.NoError(t, err)
	require.True(t, withhold, "different exact fingerprints keep independent claims")

	encoded, err := common.Marshal(groups)
	require.NoError(t, err)
	for _, secret := range []string{"fp_shared", "fp_other", "ch_shared_1", "pm_shared_1", "cus_shared_1", "first@example.com"} {
		require.NotContains(t, string(encoded), secret)
	}
}

func TestObserveStripePaymentCardPersistsNonCardAndUnknownSnapshots(t *testing.T) {
	setupStripePaymentCardObservationTest(t)
	created, err := ObserveStripePaymentCard(&StripePaymentCardObservation{
		UserId: 301, ChargeId: "ch_non_card", PaymentMethodType: "us_bank_account",
	})
	require.NoError(t, err)
	require.True(t, created)

	var observation StripePaymentCardObservation
	require.NoError(t, DB.First(&observation, "charge_id = ?", "ch_non_card").Error)
	require.Equal(t, "us_bank_account", observation.PaymentMethodType)
	require.Empty(t, observation.Fingerprint)
	require.Equal(t, StripeCardRewardDecisionNotCard, observation.RewardDecision)

	created, err = ObserveStripePaymentCard(&StripePaymentCardObservation{
		UserId: 302, ChargeId: "ch_card_without_fingerprint", PaymentMethodType: "card",
	})
	require.NoError(t, err)
	require.True(t, created)
	observation = StripePaymentCardObservation{}
	require.NoError(t, DB.First(&observation, "charge_id = ?", "ch_card_without_fingerprint").Error)
	require.Equal(t, StripeCardRewardDecisionUnknown, observation.RewardDecision)
}

func TestObserveStripePaymentCardCompletesAttributionAndClaimInOneTransaction(t *testing.T) {
	setupStripePaymentCardObservationTest(t)

	created, err := ObserveStripePaymentCard(&StripePaymentCardObservation{
		ChargeId: "ch_out_of_order", PaymentMethodType: "card", Fingerprint: "fp_out_of_order",
		Brand: "visa", Last4: "4242", ExpMonth: 12, ExpYear: 2032, Country: "us",
	})
	require.NoError(t, err)
	require.True(t, created)

	var observation StripePaymentCardObservation
	require.NoError(t, DB.First(&observation, "charge_id = ?", "ch_out_of_order").Error)
	require.Zero(t, observation.UserId)
	require.Equal(t, StripeCardRewardDecisionUnknown, observation.RewardDecision)
	var claims int64
	require.NoError(t, DB.Model(&StripeBonusClaim{}).Count(&claims).Error)
	require.Zero(t, claims)

	created, err = ObserveStripePaymentCard(&StripePaymentCardObservation{
		UserId: 401, TradeNo: "trade-out-of-order", CheckoutSessionId: "cs_out_of_order",
		PaymentIntentId: "pi_out_of_order", ChargeId: "ch_out_of_order",
	})
	require.NoError(t, err)
	require.False(t, created)
	require.NoError(t, DB.First(&observation, "charge_id = ?", "ch_out_of_order").Error)
	require.Equal(t, 401, observation.UserId)
	require.Equal(t, "trade-out-of-order", observation.TradeNo)
	require.Equal(t, "cs_out_of_order", observation.CheckoutSessionId)
	require.Equal(t, StripeCardRewardDecisionFirstExactCard, observation.RewardDecision)
	require.Equal(t, 401, observation.RewardClaimUserId)
	require.NoError(t, DB.Model(&StripeBonusClaim{}).Where("card_fingerprint = ? AND user_id = ?", "fp_out_of_order", 401).Count(&claims).Error)
	require.EqualValues(t, 1, claims)
}

func TestStripeCardBackfillLeaseIsConditionalAndOwnerScoped(t *testing.T) {
	setupStripePaymentCardObservationTest(t)

	acquired, err := AcquireStripeCardBackfillLease("stripe_card_backfill", "owner-a", 100, 160)
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = AcquireStripeCardBackfillLease("stripe_card_backfill", "owner-b", 120, 180)
	require.NoError(t, err)
	require.False(t, acquired)
	acquired, err = AcquireStripeCardBackfillLease("stripe_card_backfill", "owner-a", 130, 200)
	require.NoError(t, err)
	require.True(t, acquired, "the current owner can renew its lease")

	require.NoError(t, ReleaseStripeCardBackfillLease("stripe_card_backfill", "owner-b"))
	var lease StripePaymentCardBackfillLease
	require.NoError(t, DB.First(&lease, "lease_key = ?", "stripe_card_backfill").Error)
	require.Equal(t, "owner-a", lease.OwnerToken)
	require.Equal(t, int64(200), lease.LeaseUntil)

	acquired, err = AcquireStripeCardBackfillLease("stripe_card_backfill", "owner-b", 201, 260)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, ReleaseStripeCardBackfillLease("stripe_card_backfill", "owner-b"))
	require.NoError(t, DB.First(&lease, "lease_key = ?", "stripe_card_backfill").Error)
	require.Empty(t, lease.OwnerToken)
	require.Zero(t, lease.LeaseUntil)
}
