package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestBatchUpdateTokensAttemptsEveryCacheDeleteOnFailure(t *testing.T) {
	truncateTables(t)
	first := &Token{UserId: 1, Key: "batch-cache-one", Name: "one", ExpiredTime: -1}
	second := &Token{UserId: 1, Key: "batch-cache-two", Name: "two", ExpiredTime: -1}
	require.NoError(t, DB.Create(first).Error)
	require.NoError(t, DB.Create(second).Error)

	originalRedisEnabled := common.RedisEnabled
	originalDelete := deleteTokenCacheForBatch
	common.RedisEnabled = true
	var attempted []string
	deleteTokenCacheForBatch = func(key string) error {
		attempted = append(attempted, key)
		if key == first.Key {
			return errors.New("injected cache failure")
		}
		return nil
	}
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		deleteTokenCacheForBatch = originalDelete
	})

	enabled := true
	limits := "gpt-5"
	count, err := BatchUpdateTokens(BatchUpdateTokensParams{
		Ids:                []int{first.Id, second.Id},
		UserId:             1,
		ModelLimitsEnabled: &enabled,
		ModelLimits:        &limits,
	})

	require.Equal(t, 2, count)
	require.ErrorIs(t, err, ErrTokenBatchCacheInvalidation)
	require.ElementsMatch(t, []string{first.Key, second.Key}, attempted)

	var updated []Token
	require.NoError(t, DB.Where("id IN ?", []int{first.Id, second.Id}).Find(&updated).Error)
	require.Len(t, updated, 2)
	for _, token := range updated {
		require.True(t, token.ModelLimitsEnabled)
		require.Equal(t, limits, token.ModelLimits)
	}
}
