package service

import (
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// topUpURL suppresses only the known loopback addresses we already avoid
// surfacing to API callers; it is not a full public-address validator.
func topUpURL() string {
	base := strings.TrimRight(topUpBaseOrigin(), "/")
	if base == "" {
		return ""
	}
	parseBase := base
	if !strings.Contains(parseBase, "://") {
		parseBase = "http://" + parseBase
	}
	host := ""
	if parsed, err := url.Parse(parseBase); err == nil {
		host = parsed.Hostname()
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return ""
	}
	return base + common.ThemeAwarePath("/console/topup")
}

func topUpBaseOrigin() string {
	if consoleOrigin := strings.TrimSpace(os.Getenv("APP_CONSOLE_ORIGIN")); consoleOrigin != "" {
		return consoleOrigin
	}
	return system_setting.ServerAddress
}

func appendWalletTopUpHint(c *gin.Context, msg string) string {
	if topUp := topUpURL(); topUp != "" {
		return msg + " " + common.TranslateMessage(c, "quota.topup_hint", map[string]any{"URL": topUp})
	}
	return msg
}

// walletTopUpHintPreserveOption keeps the top-up URL readable after error
// sanitization (common.MaskSensitiveInfo masks every URL it sees). The URL is
// built from our own configured console origin (APP_CONSOLE_ORIGIN /
// ServerAddress) — an origin the operator deliberately surfaces to users — so
// masking it only breaks the payment hint: the message renders as
// "Add credits at https://***.***.ai/***/*** to keep going." and the customer
// has no path to pay. Domain-name origins are therefore preserved by default;
// IP-literal hosts stay masked (they read as internal infrastructure) unless
// force-allowed via the topup_hint.allowed_hosts allowlist.
func walletTopUpHintPreserveOption() types.NewAPIErrorOptions {
	topUp := topUpURL()
	if topUp == "" {
		return func(*types.NewAPIError) {}
	}
	if system_setting.TopupHintHostAllowed(topUp) || topUpHintHostIsDomainName(topUp) {
		return types.ErrOptionWithPreservedMessageFragments(topUp)
	}
	return func(*types.NewAPIError) {}
}

// topUpHintHostIsDomainName reports whether the URL's host is a plain domain
// name (not empty, not an IP literal). topUpURL already refuses loopback
// hosts, so anything left with a domain name is the operator's public origin.
func topUpHintHostIsDomainName(rawURL string) bool {
	candidate := rawURL
	if !strings.Contains(candidate, "://") {
		candidate = "http://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host != "" && net.ParseIP(host) == nil
}

func buildUserQuotaInsufficientMessage(c *gin.Context, quota int) string {
	base := common.TranslateMessage(c, "quota.user_insufficient", map[string]any{"Quota": logger.FormatQuota(quota)})
	return appendWalletTopUpHint(c, base)
}

func buildPreConsumeQuotaFailedMessage(c *gin.Context, remaining int, required int) string {
	base := common.TranslateMessage(c, "quota.pre_consume_failed", map[string]any{
		"Remaining": logger.FormatQuota(remaining),
		"Required":  logger.FormatQuota(required),
	})
	return appendWalletTopUpHint(c, base)
}
