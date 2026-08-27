package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

func init() {
	model.PaymentSuccessHook = NotifyDingTalkPaymentSuccess
}

func NotifyDingTalkPaymentSuccess(topUp *model.TopUp) {
	if topUp == nil {
		return
	}
	setting := operation_setting.GetMonitorSetting()
	if setting == nil || !setting.DingTalkAlertEnabled || strings.TrimSpace(setting.DingTalkAlertWebhookURL) == "" {
		return
	}
	snapshot := *topUp
	webhookURL := setting.DingTalkAlertWebhookURL
	secret := setting.DingTalkAlertSecret
	content := BuildDingTalkPaymentSuccessContent(&snapshot)
	gopool.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				common.SysError(fmt.Sprintf("panic while sending dingtalk payment success notification trade_no=%s user_id=%d panic=%v", snapshot.TradeNo, snapshot.UserId, r))
			}
		}()
		if err := SendDingTalkText(webhookURL, secret, content); err != nil {
			common.SysError(fmt.Sprintf("failed to send dingtalk payment success notification trade_no=%s user_id=%d error=%q", snapshot.TradeNo, snapshot.UserId, err.Error()))
		}
	})
}

func BuildDingTalkPaymentSuccessContent(topUp *model.TopUp) string {
	user, inviter := loadPaymentSuccessUsers(topUp)
	lines := []string{
		"【Flatkey 付款成功】",
		fmt.Sprintf("用户来源：%s", sanitizeDingTalkAlertText(paymentSuccessUserSource(user))),
		fmt.Sprintf("金额：%s", sanitizeDingTalkAlertText(paymentSuccessAmount(topUp))),
		fmt.Sprintf("国家：%s", sanitizeDingTalkAlertText(paymentSuccessCountry(user))),
		fmt.Sprintf("是否被邀：%s", paymentSuccessInvitedLabel(user)),
	}
	if inviterLine := paymentSuccessInviterLine(inviter); inviterLine != "" {
		lines = append(lines, "邀请人："+sanitizeDingTalkAlertText(inviterLine))
	}
	lines = append(lines, fmt.Sprintf("付款方式：%s", sanitizeDingTalkAlertText(paymentMethodDisplay(topUp))))
	return strings.Join(lines, "\n")
}

func loadPaymentSuccessUsers(topUp *model.TopUp) (*model.User, *model.User) {
	if topUp == nil || topUp.UserId <= 0 || model.DB == nil {
		return nil, nil
	}
	user := &model.User{}
	if err := model.DB.Omit("password").First(user, "id = ?", topUp.UserId).Error; err != nil {
		return nil, nil
	}
	if user.InviterId <= 0 {
		return user, nil
	}
	inviter := &model.User{}
	if err := model.DB.Omit("password").First(inviter, "id = ?", user.InviterId).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog(fmt.Sprintf("failed to load payment success inviter user_id=%d inviter_id=%d error=%q", user.Id, user.InviterId, err.Error()))
		}
		return user, nil
	}
	return user, inviter
}

func paymentSuccessUserSource(user *model.User) string {
	if user == nil {
		return "unknown"
	}
	if source := paymentSuccessAdsSource(user.AdsAttribution); source != "" {
		return source
	}
	providers := make([]string, 0, 6)
	if strings.TrimSpace(user.GoogleId) != "" {
		providers = append(providers, "Google")
	}
	if strings.TrimSpace(user.GitHubId) != "" {
		providers = append(providers, "GitHub")
	}
	if strings.TrimSpace(user.DiscordId) != "" {
		providers = append(providers, "Discord")
	}
	if strings.TrimSpace(user.LinuxDOId) != "" {
		providers = append(providers, "LinuxDO")
	}
	if strings.TrimSpace(user.WeChatId) != "" {
		providers = append(providers, "WeChat")
	}
	if strings.TrimSpace(user.TelegramId) != "" {
		providers = append(providers, "Telegram")
	}
	if len(providers) > 0 {
		return strings.Join(providers, " + ")
	}
	if strings.TrimSpace(user.OidcId) != "" {
		return "OIDC"
	}
	if user.InviterId > 0 {
		return "Invite"
	}
	return "Email / Direct"
}

func paymentSuccessAdsSource(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var attrs map[string]any
	if err := common.Unmarshal([]byte(raw), &attrs); err != nil {
		return ""
	}
	source := paymentSuccessStringAttr(attrs, "utm_source")
	medium := paymentSuccessStringAttr(attrs, "utm_medium")
	campaign := paymentSuccessStringAttr(attrs, "utm_campaign")
	landing := paymentSuccessStringAttr(attrs, "first_landing_path")
	if landing == "" {
		landing = paymentSuccessStringAttr(attrs, "landing_path")
	}
	parts := make([]string, 0, 4)
	if source != "" {
		parts = append(parts, source)
	}
	if medium != "" {
		parts = append(parts, medium)
	}
	if campaign != "" {
		parts = append(parts, campaign)
	}
	if landing != "" {
		parts = append(parts, landing)
	}
	return strings.Join(parts, " / ")
}

func paymentSuccessStringAttr(attrs map[string]any, key string) string {
	value, ok := attrs[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func paymentSuccessAmount(topUp *model.TopUp) string {
	if topUp == nil {
		return "-"
	}
	currency := strings.ToUpper(strings.TrimSpace(topUp.PaymentCurrency))
	if currency == "" {
		currency = "USD"
	}
	if topUp.Money > 0 {
		return fmt.Sprintf("%.2f %s", topUp.Money, currency)
	}
	if topUp.PaymentAmountMinor > 0 {
		return fmt.Sprintf("%d %s", topUp.PaymentAmountMinor, currency)
	}
	if topUp.Amount > 0 {
		return fmt.Sprintf("%d", topUp.Amount)
	}
	return "-"
}

func paymentSuccessCountry(user *model.User) string {
	if user == nil {
		return "-"
	}
	if country := strings.ToUpper(strings.TrimSpace(user.PayCountry)); country != "" {
		return country
	}
	if country := strings.ToUpper(strings.TrimSpace(user.RegistrationCountry)); country != "" {
		return country
	}
	if country := strings.ToUpper(strings.TrimSpace(model.ResolveIPCountry(user.RegistrationIP))); country != "" {
		return country
	}
	if country := strings.ToUpper(strings.TrimSpace(model.ResolveIPCountry(user.LastLoginIp))); country != "" {
		return country
	}
	return "-"
}

func paymentSuccessInvitedLabel(user *model.User) string {
	if user != nil && user.InviterId > 0 {
		return "是"
	}
	return "否"
}

func paymentSuccessInviterLine(inviter *model.User) string {
	if inviter == nil || inviter.Id <= 0 {
		return ""
	}
	name := strings.TrimSpace(inviter.DisplayName)
	if name == "" {
		name = strings.TrimSpace(inviter.Username)
	}
	if name == "" {
		name = strings.TrimSpace(inviter.Email)
	}
	if name == "" {
		return fmt.Sprintf("#%d", inviter.Id)
	}
	return fmt.Sprintf("%s (#%d)", name, inviter.Id)
}

func paymentMethodDisplay(topUp *model.TopUp) string {
	if topUp == nil {
		return "-"
	}
	method := strings.TrimSpace(topUp.PaymentMethod)
	if method == "" {
		method = strings.TrimSpace(topUp.PaymentProvider)
	}
	switch strings.ToLower(method) {
	case "pix":
		return "PIX"
	case "alipay":
		return "Alipay"
	case "wechat", "wxpay":
		return "WeChat Pay"
	case model.PaymentMethodStripe:
		return "Stripe"
	case model.PaymentMethodCreem:
		return "Creem"
	case model.PaymentMethodWaffo:
		return "Waffo"
	case model.PaymentMethodWaffoPancake:
		return "Waffo Pancake"
	case model.PaymentMethodPaddle:
		return "Paddle"
	case model.PaymentMethodBalance:
		return "Balance"
	default:
		return method
	}
}
