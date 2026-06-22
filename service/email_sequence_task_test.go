package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

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

func TestWithUTM(t *testing.T) {
	// 无 query 的链接用 ?
	got := withUTM("http://x/quickstart", 2)
	require.Equal(t, "http://x/quickstart?utm_source=lifecycle_email&utm_medium=email&utm_campaign=recall&utm_content=e2", got)
	// 已有 query 的链接用 &
	got = withUTM("http://x/sign-up?redirect=/keys", 1)
	require.Contains(t, got, "/sign-up?redirect=/keys&utm_source=lifecycle_email")
	require.Contains(t, got, "utm_content=e1")
}

func TestNormalizeEmailLang(t *testing.T) {
	require.Equal(t, "en", normalizeEmailLang("en"))
	require.Equal(t, "zh", normalizeEmailLang("zh-CN"))
	require.Equal(t, "zh", normalizeEmailLang("zh-TW"))
	require.Equal(t, "ja", normalizeEmailLang("ja"))
	require.Equal(t, "pt", normalizeEmailLang("pt-BR"))
	require.Equal(t, "es", normalizeEmailLang("ES"))
	// 不支持的语言回退 en
	require.Equal(t, "en", normalizeEmailLang("de"))
	require.Equal(t, "en", normalizeEmailLang("fr"))
	require.Equal(t, "en", normalizeEmailLang(""))
}

func TestFormatBonusText(t *testing.T) {
	require.Equal(t, "Top up $50, get $30 free", formatBonusText("en", 50, 30))
	require.Equal(t, "充 $50 送 $30", formatBonusText("zh", 50, 30))
	require.Contains(t, formatBonusText("ja", 50, 30), "チャージ")
	require.Contains(t, formatBonusText("pt", 50, 30), "Recarregue")
	require.Contains(t, formatBonusText("es", 50, 30), "Recarga")
	// 未知语言回退 en 格式
	require.Equal(t, "Top up $50, get $30 free", formatBonusText("de", 50, 30))
}

func TestBuildBonusText_NoStageReturnsEmpty(t *testing.T) {
	es := operation_setting.GetEmailSequenceSetting()
	original := es.StageBonus
	t.Cleanup(func() { es.StageBonus = original })

	es.StageBonus = map[int]operation_setting.StageBonus{
		3: {Amount: 50, Bonus: 30, WindowDays: 7},
	}
	// E2 无阶段 bonus → 空
	require.Equal(t, "", buildBonusText("en", 2, es))
	// E3 有 → 非空
	require.Contains(t, buildBonusText("en", 3, es), "50")
}
