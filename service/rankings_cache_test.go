package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetRankingsSnapshotCoalescesConcurrentBuilds(t *testing.T) {
	rankingCacheMu.Lock()
	originalCache := rankingCache
	rankingCache = map[string]rankingCacheItem{}
	originalWebsiteCache := websiteRankingCache
	websiteRankingCache = map[string]websiteRankingCacheItem{}
	rankingCacheMu.Unlock()
	originalBuilder := buildRankingsSnapshotFunc
	defer func() {
		rankingCacheMu.Lock()
		rankingCache = originalCache
		websiteRankingCache = originalWebsiteCache
		rankingInflight = map[string]*rankingInflightCall{}
		rankingCacheMu.Unlock()
		buildRankingsSnapshotFunc = originalBuilder
	}()

	var builds int32
	buildRankingsSnapshotFunc = func(config rankingPeriodConfig, now time.Time) (*RankingsResponse, error) {
		atomic.AddInt32(&builds, 1)
		time.Sleep(20 * time.Millisecond)
		return &RankingsResponse{}, nil
	}

	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := GetRankingsSnapshot("week")
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	for err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&builds))
}
