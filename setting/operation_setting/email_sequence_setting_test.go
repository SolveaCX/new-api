package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEmailSequenceSetting_Defaults(t *testing.T) {
	s := GetEmailSequenceSetting()
	require.NotNil(t, s)
	// 默认延迟天数 0/3/14/30
	require.Equal(t, 0, s.DelayDays(1))
	require.Equal(t, 3, s.DelayDays(2))
	require.Equal(t, 14, s.DelayDays(3))
	require.Equal(t, 30, s.DelayDays(4))
	require.Greater(t, s.BatchLimit, 0)
	require.NotEmpty(t, s.InternalEmailDomains)
}

func TestStageBonusFor(t *testing.T) {
	s := GetEmailSequenceSetting()
	original := s.StageBonus
	t.Cleanup(func() { s.StageBonus = original })

	s.StageBonus = map[int]StageBonus{
		3: {Amount: 50, Bonus: 30, WindowDays: 7},
		4: {Amount: 100, Bonus: 80, WindowDays: 7},
	}
	b, ok := s.StageBonusFor(3)
	require.True(t, ok)
	require.Equal(t, int64(30), b.Bonus)
	require.Equal(t, 50, b.Amount)

	_, ok = s.StageBonusFor(2)
	require.False(t, ok, "E2 无阶段 bonus")

	// Amount<=0 视为无效
	s.StageBonus = map[int]StageBonus{1: {Amount: 0, Bonus: 10}}
	_, ok = s.StageBonusFor(1)
	require.False(t, ok, "Amount<=0 应视为无效")
}

func TestIsStepEnabled_DefaultTrue(t *testing.T) {
	s := GetEmailSequenceSetting()
	original := s.StepEnabled
	t.Cleanup(func() { s.StepEnabled = original })

	// 未配置时默认 true
	s.StepEnabled = map[int]bool{}
	require.True(t, s.IsStepEnabled(1))
	// 显式关闭
	s.StepEnabled = map[int]bool{2: false}
	require.False(t, s.IsStepEnabled(2))
	require.True(t, s.IsStepEnabled(3))
}
