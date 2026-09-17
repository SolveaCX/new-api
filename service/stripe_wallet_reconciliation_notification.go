package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

var stripeWalletNotificationSecretPattern = regexp.MustCompile(`(?i)\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9_-]+`)

type stripeWalletNotification struct {
	Kind              string
	Scope             string
	SessionID         string
	EventID           string
	TradeNo           string
	LocalStatus       string
	Detail            string
	UserID            int
	Amount            int64
	PaidAt            int64
	FirstAnomalyAt    int64
	Currency          string
	NotificationCount int
	AutoRepairedAt    int64
	RepairFromStatus  string
	RepairCredit      int64
}

func sendStripeWalletReconciliationNotification(ctx context.Context, notification stripeWalletNotification) error {
	setting := operation_setting.GetMonitorSetting()
	if setting == nil || !setting.DingTalkAlertEnabled {
		return fmt.Errorf("stripe wallet reconciliation notification is disabled")
	}
	if strings.TrimSpace(setting.DingTalkAlertWebhookURL) == "" {
		return fmt.Errorf("stripe wallet reconciliation dingtalk webhook url is empty")
	}
	if strings.TrimSpace(setting.DingTalkAlertSecret) == "" {
		return fmt.Errorf("stripe wallet reconciliation dingtalk signing secret is empty")
	}
	content := buildStripeWalletReconciliationNotification(notification)
	return sendStripeWalletReconciliationDingTalkText(ctx, setting.DingTalkAlertWebhookURL, setting.DingTalkAlertSecret, content)
}

func buildStripeWalletReconciliationNotification(notification stripeWalletNotification) string {
	title := map[string]string{
		"initial":       "Stripe 钱包对账异常",
		"reminder":      "Stripe 钱包对账异常提醒",
		"recovered":     "Stripe 钱包对账已恢复",
		"task_error":    "Stripe 钱包对账任务错误",
		"auto_repaired": "Stripe 钱包漏单已自动补账，请核查",
	}[strings.ToLower(strings.TrimSpace(notification.Kind))]
	if title == "" {
		title = "Stripe 钱包对账通知"
	}

	lines := []string{
		title,
		"类型：" + safeStripeWalletNotificationField(notification.Kind),
		"账户范围：" + safeStripeWalletNotificationField(notification.Scope),
	}
	if notification.SessionID != "" {
		lines = append(lines, "Stripe Session："+safeStripeWalletNotificationField(notification.SessionID))
	}
	if notification.EventID != "" {
		lines = append(lines, "Stripe Event："+safeStripeWalletNotificationField(notification.EventID))
	}
	if notification.TradeNo != "" {
		lines = append(lines, "本地订单："+safeStripeWalletNotificationField(notification.TradeNo))
	}
	if notification.UserID > 0 {
		lines = append(lines, fmt.Sprintf("用户 ID：%d", notification.UserID))
	}
	if notification.LocalStatus != "" {
		lines = append(lines, "本地状态："+safeStripeWalletNotificationField(notification.LocalStatus))
	}
	if notification.Amount != 0 || strings.TrimSpace(notification.Currency) != "" {
		lines = append(lines, fmt.Sprintf("支付金额（最小货币单位）：%d %s", notification.Amount, safeStripeWalletNotificationField(strings.ToUpper(notification.Currency))))
	}
	if notification.PaidAt > 0 {
		lines = append(lines, "Stripe 支付成功事件时间："+time.Unix(notification.PaidAt, 0).UTC().Format(time.RFC3339))
	}
	if notification.FirstAnomalyAt > 0 {
		lines = append(lines, "首次发现异常："+time.Unix(notification.FirstAnomalyAt, 0).UTC().Format(time.RFC3339))
	}
	if notification.NotificationCount > 0 {
		lines = append(lines, fmt.Sprintf("通知次数：%d", notification.NotificationCount))
	}
	if detail := safeStripeWalletNotificationDetail(notification.Detail); detail != "" {
		lines = append(lines, "详情："+detail)
	}
	if notification.Kind == "auto_repaired" {
		lines = append(lines,
			"补账前状态："+safeStripeWalletNotificationField(notification.RepairFromStatus),
			fmt.Sprintf("补入额度（quota，含实际获赠额度）：%d", notification.RepairCredit),
			"自动补账时间："+time.Unix(notification.AutoRepairedAt, 0).UTC().Format(time.RFC3339),
			"处理方式：已自动补入钱包并完成订单，请核查异常原因；不要重复加款。")
	} else if notification.Kind == "recovered" {
		lines = append(lines, "处理方式：本地订单已恢复成功，请核查之前的异常原因；不要重复加款。")
	} else {
		lines = append(lines, "处理方式：未确认自动补账成功，请人工核查；缺单、身份冲突、退款或争议不会自动加款。")
	}
	return strings.Join(lines, "\n")
}

func safeStripeWalletNotificationField(value string) string {
	value = strings.TrimSpace(sanitizeDingTalkAlertText(stripeWalletNotificationSecretPattern.ReplaceAllString(value, "stripe-key-***")))
	return truncateStripeWalletNotificationText(value, 240)
}

func safeStripeWalletNotificationDetail(value string) string {
	value = strings.TrimSpace(sanitizeDingTalkAlertText(stripeWalletNotificationSecretPattern.ReplaceAllString(value, "stripe-key-***")))
	return truncateStripeWalletNotificationText(value, 600)
}

func truncateStripeWalletNotificationText(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return value
}

func sendStripeWalletReconciliationDingTalkText(ctx context.Context, webhookURL string, secret string, content string) error {
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("stripe wallet reconciliation dingtalk signing secret is empty")
	}
	finalURL, err := BuildDingTalkWebhookURL(webhookURL, secret, time.Now())
	if err != nil {
		return fmt.Errorf("failed to build dingtalk webhook url: %s", sanitizeDingTalkAlertText(err.Error()))
	}
	payloadBytes, err := common.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	})
	if err != nil {
		return err
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(finalURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("request reject: %s", sanitizeDingTalkAlertText(err.Error()))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, dingTalkRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finalURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create dingtalk request: %s", sanitizeDingTalkAlertText(err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NewAPI-Stripe-Wallet-Reconciliation/1.0")
	client := GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk request failed: %s", sanitizeDingTalkAlertText(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("dingtalk request failed with status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, dingTalkMaxResponseBodyBytes))
	if err != nil {
		return fmt.Errorf("failed to read dingtalk response: %s", sanitizeDingTalkAlertText(err.Error()))
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return fmt.Errorf("dingtalk request returned empty response")
	}
	var sendResponse dingTalkSendResponse
	if err := common.Unmarshal(body, &sendResponse); err != nil {
		return fmt.Errorf("dingtalk request returned invalid response: %s", sanitizeDingTalkAlertText(err.Error()))
	}
	if sendResponse.ErrCode == nil {
		return fmt.Errorf("dingtalk request returned missing errcode")
	}
	if *sendResponse.ErrCode != 0 {
		return fmt.Errorf("dingtalk request failed: errcode=%d errmsg=%s", *sendResponse.ErrCode, sanitizeDingTalkAlertText(sendResponse.ErrMsg))
	}
	return nil
}
