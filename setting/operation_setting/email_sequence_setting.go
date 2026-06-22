package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// StageBonus 单个召回阶段的充值赠送配置(单档推荐位)。
type StageBonus struct {
	Amount     int   `json:"amount"`      // 推荐充值档位金额(USD)
	Bonus      int64 `json:"bonus"`       // 该档位赠送额度(USD)
	WindowDays int   `json:"window_days"` // 邮件发出后多少天内充值可享此阶段 bonus
}

// EmailSequenceSetting 召回邮件序列配置。
type EmailSequenceSetting struct {
	// StepDelayDays: step(1-4) → 注册后多少天发送(默认 0/3/14/30)
	StepDelayDays map[int]int `json:"step_delay_days"`
	// StepEnabled: step(1-4) → 是否启用该封(默认全 true)
	StepEnabled map[int]bool `json:"step_enabled"`
	// StageBonus: step(3,4 及可选 1) → 阶段充值赠送配置
	StageBonus map[int]StageBonus `json:"stage_bonus"`
	// BatchLimit: 单次任务运行最多处理多少用户(防 SMTP 打爆)
	BatchLimit int `json:"batch_limit"`
	// InternalEmailDomains: 内部账号邮箱域名白名单(命中即不发)
	InternalEmailDomains []string `json:"internal_email_domains"`
}

// stepDelayDefaults 默认延迟天数,作为缺省回退。
var stepDelayDefaults = map[int]int{1: 0, 2: 3, 3: 14, 4: 30}

var emailSequenceSetting = EmailSequenceSetting{
	StepDelayDays: map[int]int{1: 0, 2: 3, 3: 14, 4: 30},
	StepEnabled:   map[int]bool{1: true, 2: true, 3: true, 4: true},
	StageBonus:    map[int]StageBonus{},
	BatchLimit:    500,
	InternalEmailDomains: []string{
		"lockin.com", "voc.ai", "shulex", "solvea", "flatkey.ai", "quantumnous",
	},
}

func init() {
	config.GlobalConfig.Register("email_sequence_setting", &emailSequenceSetting)
}

func GetEmailSequenceSetting() *EmailSequenceSetting {
	return &emailSequenceSetting
}

// StageBonusFor 返回某 step 配置的阶段 bonus(若有且有效)。
func (s *EmailSequenceSetting) StageBonusFor(step int) (StageBonus, bool) {
	b, ok := s.StageBonus[step]
	if !ok || b.Amount <= 0 {
		return StageBonus{}, false
	}
	return b, true
}

// DelayDays 返回某 step 的延迟天数,无配置回退默认。
func (s *EmailSequenceSetting) DelayDays(step int) int {
	if d, ok := s.StepDelayDays[step]; ok {
		return d
	}
	return stepDelayDefaults[step]
}

// IsStepEnabled 某 step 是否启用(默认 true)。
func (s *EmailSequenceSetting) IsStepEnabled(step int) bool {
	if e, ok := s.StepEnabled[step]; ok {
		return e
	}
	return true
}
