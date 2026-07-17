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

func TestPerfMetricAvailabilityFiveMinuteCountersAccumulate(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PerfMetricAvailability{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, UpsertPerfMetricAvailability(&PerfMetricAvailability{
		ModelName:     "gpt-5",
		Group:         "default",
		BucketTs:      300,
		EligibleCount: 2,
		SuccessCount:  1,
	}))
	require.NoError(t, UpsertPerfMetricAvailability(&PerfMetricAvailability{
		ModelName:     "gpt-5",
		Group:         "default",
		BucketTs:      300,
		EligibleCount: 3,
		SuccessCount:  3,
	}))

	summaries, err := GetPerfMetricAvailabilitySummaryAll(300, 599, []string{"default"})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.EqualValues(t, 5, summaries[0].AvailabilityEligibleCount)
	require.EqualValues(t, 4, summaries[0].AvailabilitySuccessCount)
}
