package service

import (
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// isInternalUser 判断是否内部账号(命中即不发召回邮件)。
// 规则:用户名包含独立 test token;或邮箱域名属于内部白名单(子串匹配,覆盖 shulex/solvea 这类无 TLD 的写法)。
func isInternalUser(u *model.User, internalDomains []string) bool {
	if isInternalTestUsername(u.Username) {
		return true
	}
	email := strings.ToLower(u.Email)
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := email[at+1:]
	for _, d := range internalDomains {
		if d == "" {
			continue
		}
		if strings.Contains(domain, strings.ToLower(d)) {
			return true
		}
	}
	return false
}

func isInternalTestUsername(username string) bool {
	parts := strings.FieldsFunc(strings.ToLower(username), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, part := range parts {
		if part == "test" {
			return true
		}
	}
	return false
}

// stepTargetMet 该 step 的目标是否已达成(达成则跳过)。
// E2: request_count>0(已首调)。E3/E4: 已充值(hasPaid)。E1: 永不达成。
func stepTargetMet(u *model.User, step int, hasPaid bool) bool {
	switch step {
	case model.EmailSeqStepE2:
		return u.RequestCount > 0
	case model.EmailSeqStepE3, model.EmailSeqStepE4:
		return hasPaid
	default:
		return false
	}
}

// isValidEmail 邮箱非空且基本格式合法。
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}

// emailSequenceGloballyEnabled 全局开关 + SMTP 已配置(任一不满足则整序列静默禁用)。
func emailSequenceGloballyEnabled() bool {
	if !common.EmailSequenceEnabled {
		return false
	}
	if common.SMTPServer == "" && common.SMTPAccount == "" {
		return false // SMTP 未配置,静默禁用
	}
	return true
}
