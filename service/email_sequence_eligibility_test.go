package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestIsInternalUser(t *testing.T) {
	domains := []string{"lockin.com", "voc.ai", "shulex", "solvea", "flatkey.ai", "quantumnous"}
	// 内部邮箱域名
	require.True(t, isInternalUser(&model.User{Email: "a@flatkey.ai", Group: "plg"}, domains))
	require.True(t, isInternalUser(&model.User{Email: "a@voc.ai", Group: "plg"}, domains))
	require.True(t, isInternalUser(&model.User{Email: "a@mail.shulex.com", Group: "plg"}, domains), "子串匹配 shulex")
	// 用户名含 test
	require.True(t, isInternalUser(&model.User{Email: "x@gmail.com", Username: "test_user", Group: "plg"}, domains))
	require.True(t, isInternalUser(&model.User{Email: "x@gmail.com", Username: "QA_TEST", Group: "plg"}, domains), "大小写不敏感")
	// 正常外部用户(plg 是正常组,不排除)
	require.False(t, isInternalUser(&model.User{Email: "real@gmail.com", Username: "alice", Group: "plg"}, domains))
	require.False(t, isInternalUser(&model.User{Email: "bob@company.com", Username: "bob", Group: "default"}, domains))
}

func TestStepTargetMet(t *testing.T) {
	// E1: 永不达成
	require.False(t, stepTargetMet(&model.User{RequestCount: 100}, model.EmailSeqStepE1, true))
	// E2: request_count>0 即达成
	require.True(t, stepTargetMet(&model.User{RequestCount: 5}, model.EmailSeqStepE2, false))
	require.False(t, stepTargetMet(&model.User{RequestCount: 0}, model.EmailSeqStepE2, false))
	// E3/E4: 已充值即达成(hasPaid 参数)
	require.True(t, stepTargetMet(&model.User{}, model.EmailSeqStepE3, true))
	require.True(t, stepTargetMet(&model.User{}, model.EmailSeqStepE4, true))
	require.False(t, stepTargetMet(&model.User{}, model.EmailSeqStepE3, false))
	require.False(t, stepTargetMet(&model.User{}, model.EmailSeqStepE4, false))
}

func TestIsValidEmail(t *testing.T) {
	require.True(t, isValidEmail("a@b.com"))
	require.True(t, isValidEmail("user.name@sub.example.co"))
	require.False(t, isValidEmail(""))
	require.False(t, isValidEmail("notanemail"))
	require.False(t, isValidEmail("@nodomain.com"))
	require.False(t, isValidEmail("noat@"))
}
