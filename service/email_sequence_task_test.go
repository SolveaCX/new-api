package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDueStep(t *testing.T) {
	delays := map[int]int{1: 0, 2: 3, 3: 14, 4: 30}
	day := int64(24 * 3600)
	const nowTs = int64(1_700_000_000)

	// 注册 0.5 天 → 应发 E1(D0)
	require.Equal(t, 1, dueStep(nowTs-day/2, delays, nowTs))
	// 注册 5 天 → 最大已到期是 E2(D3)
	require.Equal(t, 2, dueStep(nowTs-5*day, delays, nowTs))
	// 注册 20 天 → E3(D14)
	require.Equal(t, 3, dueStep(nowTs-20*day, delays, nowTs))
	// 注册 40 天 → E4(D30)
	require.Equal(t, 4, dueStep(nowTs-40*day, delays, nowTs))
	// 恰好 D3 边界 → E2
	require.Equal(t, 2, dueStep(nowTs-3*day, delays, nowTs))
}

func TestDueStep_FutureRegistration(t *testing.T) {
	delays := map[int]int{1: 0, 2: 3, 3: 14, 4: 30}
	const nowTs = int64(1_700_000_000)
	// 注册时间在未来(时钟异常)→ 0,不发
	require.Equal(t, 0, dueStep(nowTs+1000, delays, nowTs))
}

func TestDueStep_PartialDelays(t *testing.T) {
	// 只配了 step1/2(管理员关了 E3/E4 的延迟项,理论边界)
	delays := map[int]int{1: 0, 2: 3}
	day := int64(24 * 3600)
	const nowTs = int64(1_700_000_000)
	// 注册 40 天,但只有 1/2 配了 → 最大 E2
	require.Equal(t, 2, dueStep(nowTs-40*day, delays, nowTs))
}
