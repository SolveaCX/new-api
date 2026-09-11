package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

// PLGCatalogWatchOutcome describes what one watcher cycle did.
type PLGCatalogWatchOutcome string

const (
	PLGCatalogWatchBaselineCreated              PLGCatalogWatchOutcome = "baseline_created"
	PLGCatalogWatchUnchanged                    PLGCatalogWatchOutcome = "unchanged"
	PLGCatalogWatchNotified                     PLGCatalogWatchOutcome = "notified"
	PLGCatalogWatchChangedNotificationDisabled  PLGCatalogWatchOutcome = "changed_notification_disabled"
	PLGCatalogWatchChangedNotificationFailed    PLGCatalogWatchOutcome = "changed_notification_failed"
	plgCatalogWatchGroup                                               = modelAccessPLGGroup
	plgCatalogChangeContentMaxNamesPerDirection                        = 20
)

type PLGCatalogWatchResult struct {
	Outcome PLGCatalogWatchOutcome
	Added   []string
	Removed []string
	Total   int
}

type PLGCatalogChange struct {
	Added      []string
	Removed    []string
	TotalCount int
	Now        time.Time
}

var (
	plgCatalogWatchTaskOnce  sync.Once
	plgCatalogChangeTimezone = time.FixedZone("UTC+8", 8*3600)
)

// FingerprintModelNames hashes the sorted, de-duplicated names so two nodes
// observing the same catalog agree on the fingerprint regardless of order.
func FingerprintModelNames(modelNames []string) string {
	names := normalizedStrings(modelNames)
	hash := sha256.Sum256([]byte(strings.Join(names, "\n")))
	return hex.EncodeToString(hash[:])
}

// RunPLGModelCatalogWatchOnce resolves the plg catalog, compares it with the
// stored snapshot, claims any change with a compare-and-swap update, and
// notifies DingTalk when this node won the claim. It is safe to run on every
// master node concurrently: only the node whose UPDATE hits the previous
// fingerprint sends the message.
func RunPLGModelCatalogWatchOnce(now time.Time) (PLGCatalogWatchResult, error) {
	current, err := ResolvePLGCatalogModelNames()
	if err != nil {
		return PLGCatalogWatchResult{}, fmt.Errorf("resolve plg catalog: %w", err)
	}
	fingerprint := FingerprintModelNames(current)
	result := PLGCatalogWatchResult{Total: len(current)}

	previous, err := model.GetPLGModelCatalogSnapshot(plgCatalogWatchGroup)
	if err != nil {
		return PLGCatalogWatchResult{}, fmt.Errorf("load plg catalog snapshot: %w", err)
	}
	if previous == nil {
		created, err := model.CreatePLGModelCatalogSnapshotIfMissing(plgCatalogWatchGroup, fingerprint, current, now.Unix())
		if err != nil {
			return PLGCatalogWatchResult{}, fmt.Errorf("create plg catalog baseline: %w", err)
		}
		if created {
			result.Outcome = PLGCatalogWatchBaselineCreated
			common.SysLog(fmt.Sprintf("plg catalog watch: baseline recorded with %d models", len(current)))
			return result, nil
		}
		// Another node wrote the baseline between our read and insert; the
		// next cycle compares against it normally.
		result.Outcome = PLGCatalogWatchUnchanged
		return result, nil
	}
	if previous.Fingerprint == fingerprint {
		result.Outcome = PLGCatalogWatchUnchanged
		return result, nil
	}

	claimed, err := model.ClaimPLGModelCatalogSnapshotChange(plgCatalogWatchGroup, previous.Fingerprint, fingerprint, current, now.Unix())
	if err != nil {
		return PLGCatalogWatchResult{}, fmt.Errorf("claim plg catalog change: %w", err)
	}
	if !claimed {
		result.Outcome = PLGCatalogWatchUnchanged
		return result, nil
	}

	result.Added, result.Removed = DiffModelNames(previous.ModelNameList(), current)
	common.SysLog(fmt.Sprintf("plg catalog watch: change detected added=%d removed=%d total=%d added_models=%s removed_models=%s",
		len(result.Added), len(result.Removed), len(current),
		strings.Join(result.Added, ","), strings.Join(result.Removed, ",")))

	setting := operation_setting.GetPLGCatalogNotifySetting()
	if setting == nil || !setting.DingTalkAlertEnabled || strings.TrimSpace(setting.DingTalkAlertWebhookURL) == "" {
		result.Outcome = PLGCatalogWatchChangedNotificationDisabled
		return result, nil
	}
	content := BuildPLGModelCatalogChangeContent(PLGCatalogChange{
		Added:      result.Added,
		Removed:    result.Removed,
		TotalCount: len(current),
		Now:        now,
	})
	if err := SendDingTalkText(setting.DingTalkAlertWebhookURL, setting.DingTalkAlertSecret, content); err != nil {
		// The snapshot already advanced, so this change is not resent; the
		// system log above keeps the diff recoverable.
		common.SysError("plg catalog watch: failed to send dingtalk notification: " + err.Error())
		result.Outcome = PLGCatalogWatchChangedNotificationFailed
		return result, nil
	}
	result.Outcome = PLGCatalogWatchNotified
	return result, nil
}

// BuildPLGModelCatalogChangeContent renders the plain-text DingTalk message.
// The leading "PLG" keyword lets robots configured with keyword security
// accept it.
func BuildPLGModelCatalogChangeContent(change PLGCatalogChange) string {
	var builder strings.Builder
	builder.WriteString("PLG 可用模型变更\n")
	builder.WriteString("时间：")
	builder.WriteString(change.Now.In(plgCatalogChangeTimezone).Format("2006-01-02 15:04:05"))
	builder.WriteString(" (UTC+8)")
	if len(change.Added) > 0 {
		builder.WriteString("\n新增 ")
		builder.WriteString(fmt.Sprintf("%d 个：", len(change.Added)))
		builder.WriteString(formatPLGCatalogNames(change.Added))
	}
	if len(change.Removed) > 0 {
		builder.WriteString("\n移除 ")
		builder.WriteString(fmt.Sprintf("%d 个：", len(change.Removed)))
		builder.WriteString(formatPLGCatalogNames(change.Removed))
	}
	builder.WriteString(fmt.Sprintf("\n当前共 %d 个模型", change.TotalCount))
	return sanitizeDingTalkAlertText(builder.String())
}

func formatPLGCatalogNames(names []string) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	if len(sorted) <= plgCatalogChangeContentMaxNamesPerDirection {
		return strings.Join(sorted, ", ")
	}
	shown := sorted[:plgCatalogChangeContentMaxNamesPerDirection]
	return fmt.Sprintf("%s（其余 %d 个已省略）", strings.Join(shown, ", "), len(sorted)-len(shown))
}

func plgCatalogWatchInterval() time.Duration {
	minutes := operation_setting.GetPLGCatalogNotifySetting().EffectiveCheckIntervalMinutes()
	return time.Duration(minutes * float64(time.Minute))
}

// StartPLGModelCatalogWatchTask polls the plg catalog on master nodes. The
// interval is re-read every cycle so operators can tune it without a restart.
// It keeps polling while notifications are disabled so the stored baseline
// stays current and enabling the switch later does not replay old changes.
func StartPLGModelCatalogWatchTask() {
	if !common.IsMasterNode {
		return
	}
	plgCatalogWatchTaskOnce.Do(func() {
		gopool.Go(func() {
			for {
				time.Sleep(plgCatalogWatchInterval())
				if _, err := RunPLGModelCatalogWatchOnce(time.Now()); err != nil {
					common.SysError("plg catalog watch: " + err.Error())
				}
			}
		})
	})
}
