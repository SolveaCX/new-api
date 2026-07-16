package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPerfMetricAvailabilityCountersAccumulate(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PerfMetric{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	base := &PerfMetric{
		ModelName:                 "gpt-5",
		Group:                     "default",
		BucketTs:                  100,
		RequestCount:              2,
		SuccessCount:              1,
		AvailabilityEligibleCount: 2,
		AvailabilitySuccessCount:  1,
	}
	require.NoError(t, UpsertPerfMetric(base))
	require.NoError(t, UpsertPerfMetric(&PerfMetric{
		ModelName:                 "gpt-5",
		Group:                     "default",
		BucketTs:                  100,
		RequestCount:              3,
		SuccessCount:              3,
		AvailabilityEligibleCount: 1,
		AvailabilitySuccessCount:  1,
	}))

	var got PerfMetric
	require.NoError(t, DB.First(&got).Error)
	require.EqualValues(t, 3, got.AvailabilityEligibleCount)
	require.EqualValues(t, 2, got.AvailabilitySuccessCount)
}
