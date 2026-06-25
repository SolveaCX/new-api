package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const emailSequenceTickInterval = 1 * time.Hour

var (
	emailSequenceOnce    sync.Once
	emailSequenceRunning atomic.Bool
)

// dueStep 根据注册时间返回当前"最大已到期"的 step(1-4),0 表示还没到 E1。
// createdAt: 用户注册时间戳;delays: step→延迟天数;nowTs: 当前时间戳。
func dueStep(createdAt int64, delays map[int]int, nowTs int64) int {
	ageSeconds := nowTs - createdAt
	if ageSeconds < 0 {
		return 0 // 注册时间在未来(时钟异常),不发
	}
	ageDays := ageSeconds / (24 * 3600)
	result := 0
	for step := 1; step <= 4; step++ {
		d, ok := delays[step]
		if !ok {
			continue
		}
		if ageDays >= int64(d) && step > result {
			result = step
		}
	}
	return result
}

// StartEmailSequenceTask 启动召回邮件定时任务(每小时)。仅 master 节点运行。
func StartEmailSequenceTask() {
	emailSequenceOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("email sequence task started: tick=%s", emailSequenceTickInterval))
			ticker := time.NewTicker(emailSequenceTickInterval)
			defer ticker.Stop()
			runEmailSequenceOnce()
			for range ticker.C {
				runEmailSequenceOnce()
			}
		})
	})
}

func runEmailSequenceOnce() {
	if !emailSequenceRunning.CompareAndSwap(false, true) {
		return
	}
	defer emailSequenceRunning.Store(false)

	if !emailSequenceGloballyEnabled() {
		return // 开关关 / SMTP 未配置 → 静默
	}

	ctx := context.Background()
	es := operation_setting.GetEmailSequenceSetting()

	// 只扫描可能还在序列内的用户:注册时间在最大延迟天数 + 缓冲内
	maxDelayDays := 0
	for _, d := range es.StepDelayDays {
		if d > maxDelayDays {
			maxDelayDays = d
		}
	}
	cutoff := common.GetTimestamp() - int64(maxDelayDays+2)*24*3600
	users, err := model.GetUsersRegisteredAfter(cutoff, 0)
	if err != nil {
		logger.LogError(ctx, "email sequence: scan users failed: "+err.Error())
		return
	}

	sendLimit := es.BatchLimit
	if sendLimit <= 0 {
		sendLimit = 500
	}
	sentCount := 0
	for _, u := range users {
		if sentCount >= sendLimit {
			break
		}
		step := dueStep(u.CreatedAt, es.StepDelayDays, common.GetTimestamp())
		if step == 0 || !es.IsStepEnabled(step) {
			continue
		}
		// 抑制规则:退订 / 禁用 / 空邮箱或非法 / 内部账号
		if u.EmailOptOut || u.Status != common.UserStatusEnabled || !isValidEmail(u.Email) {
			continue
		}
		if isInternalUser(u, es.InternalEmailDomains) {
			continue
		}
		// 已达成目标判定(E2 看 request_count;E3/E4 看是否已充值)
		hasPaid := false
		if step == model.EmailSeqStepE3 || step == model.EmailSeqStepE4 {
			paid, ok := emailSequencePaidStatus(ctx, u.Id, model.HasSuccessfulTopUp)
			if !ok {
				continue
			}
			hasPaid = paid
		}
		if stepTargetMet(u, step, hasPaid) {
			continue
		}
		// 单用户每日 1 封节流:今天已发过任意 step 则跳过
		sentToday, ok := emailSequenceSentToday(ctx, u.Id, model.HasSentAnyStepToday)
		if !ok {
			continue
		}
		if sentToday {
			continue
		}

		// 渲染 + 发送 + 落记录(幂等:先占记录再发)
		sent, err := sendRecallEmail(u, step, es)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("email sequence send failed user=%d step=%d: %v", u.Id, step, err))
			continue
		}
		if sent {
			sentCount++
		}
	}
	if sentCount > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("email sequence: sent %d emails", sentCount))
	}
}

// sendRecallEmail 渲染并发送一封召回邮件,幂等落记录。
func sendRecallEmail(u *model.User, step int, es *operation_setting.EmailSequenceSetting) (bool, error) {
	lang := normalizeEmailLang(model.GetUserLanguage(u.Id))
	bonusText := buildBonusText(lang, step, es)
	if emailSequenceStepRequiresBonus(step) && bonusText == "" {
		return false, nil
	}

	// 幂等:先尝试占用 (user,step) 名额,占不到说明已发过
	ok, err := model.RecordEmailSequenceSent(u.Id, step)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil // 已发过,跳过
	}

	origin := system_setting.ServerAddress
	data := EmailRenderData{
		SystemName:     common.SystemName,
		SignUpLink:     withUTM(origin+"/sign-up?redirect=/keys", step),
		QuickstartLink: withUTM(origin+"/quickstart", step),
		TopupLink:      withUTM(origin+"/wallet", step),
		BonusText:      bonusText,
		UnsubscribeURL: BuildUnsubscribeLink(origin, u.Id),
	}
	subject, body, err := RenderEmail(lang, step, data)
	if err != nil {
		if cleanupErr := model.DeleteEmailSequenceSent(u.Id, step); cleanupErr != nil {
			return false, fmt.Errorf("%w; release email sequence record failed: %v", err, cleanupErr)
		}
		return false, err
	}
	if err := common.SendEmail(subject, u.Email, body); err != nil {
		return false, err
	}
	return true, nil
}

func emailSequenceStepRequiresBonus(step int) bool {
	return step == model.EmailSeqStepE3 || step == model.EmailSeqStepE4
}

func emailSequencePaidStatus(ctx context.Context, userId int, check func(int) (bool, error)) (bool, bool) {
	hasPaid, err := check(userId)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("email sequence: topup check failed user=%d: %v", userId, err))
		return false, false
	}
	return hasPaid, true
}

func emailSequenceSentToday(ctx context.Context, userId int, check func(int) (bool, error)) (bool, bool) {
	sentToday, err := check(userId)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("email sequence: sent-today check failed user=%d: %v", userId, err))
		return false, false
	}
	return sentToday, true
}

// normalizeEmailLang 把用户语言归一化到支持的 5 语言,不支持回退 en。
func normalizeEmailLang(lang string) string {
	l := lang
	if len(l) >= 2 {
		l = strings.ToLower(l[:2])
	}
	switch l {
	case "en", "zh", "pt", "es", "ja":
		return l
	default:
		return "en"
	}
}

// withUTM 给链接追加 UTM 参数。
func withUTM(link string, step int) string {
	sep := "?"
	if strings.Contains(link, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%sutm_source=lifecycle_email&utm_medium=email&utm_campaign=recall&utm_content=e%d", link, sep, step)
}

// buildBonusText 生成当前阶段 bonus 文案(从配置实时读,不写死)。
func buildBonusText(lang string, step int, es *operation_setting.EmailSequenceSetting) string {
	sb, ok := es.StageBonusFor(step)
	if !ok {
		return "" // E2 等无 bonus 阶段
	}
	return formatBonusText(lang, sb.Amount, sb.Bonus)
}

// formatBonusText 按语言格式化 "充 $X 送 $Y"。
func formatBonusText(lang string, amount int, bonus int64) string {
	switch lang {
	case "zh":
		return fmt.Sprintf("充 $%d 送 $%d", amount, bonus)
	case "pt":
		return fmt.Sprintf("Recarregue $%d e ganhe $%d", amount, bonus)
	case "es":
		return fmt.Sprintf("Recarga $%d y obtén $%d", amount, bonus)
	case "ja":
		return fmt.Sprintf("$%d チャージで $%d プレゼント", amount, bonus)
	default:
		return fmt.Sprintf("Top up $%d, get $%d free", amount, bonus)
	}
}
