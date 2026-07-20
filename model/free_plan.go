package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Free 免费套餐（纯套餐模式的新用户兜底档）
//
// 注册（风控通过后）自动发放：$1 等值月池、全模型、1 个月有效、一次性不刷新。
// 全局模型权重表照常作用——$1 池调 Opus(×2.5) 实得约 $0.4 list，天然限量，
// 无需为 Free 单设模型范围。
// ---------------------------------------------------------------------------

// FreePlanTitle 是免费套餐的固定标题，用于幂等查找 seed。运营侧不要重命名。
const FreePlanTitle = "Free"

// freePlanTotalUSD Free 档月池的美元等值额度
const freePlanTotalUSD = 1.0

var freePlanSeedMu sync.Mutex

// EnsureFreePlanSeed 幂等确保 Free 套餐存在（懒加载：首次发放时创建，无需改
// main.go 启动流程）。enabled=false 使其不出现在购买列表，仅系统发放。
func EnsureFreePlanSeed() (*SubscriptionPlan, error) {
	if plan, err := findFreePlan(); err != nil {
		return nil, err
	} else if plan != nil {
		return plan, nil
	}

	// 进程内互斥缩小并发窗口；跨节点并发极端情况下可能出现重复行，
	// findFreePlan 恒取最小 id，多余行无害（enabled=false 不可购买、不被引用）。
	freePlanSeedMu.Lock()
	defer freePlanSeedMu.Unlock()
	if plan, err := findFreePlan(); err != nil {
		return nil, err
	} else if plan != nil {
		return plan, nil
	}

	totalAmount := int64(freePlanTotalUSD * common.QuotaPerUnit)
	plan := &SubscriptionPlan{
		Title:              FreePlanTitle,
		Subtitle:           "New user free tier",
		PriceAmount:        0,
		Currency:           "USD",
		DurationUnit:       "month",
		DurationValue:      1,
		Enabled:            false, // 不进购买列表，仅注册发放
		MaxPurchasePerUser: 1,     // 一人一生一次（发放幂等的硬保证）
		TotalAmount:        totalAmount,
		Window5hAmount:     0, // Free 不启用短窗，月池即唯一闸
		WindowWeekAmount:   0,
		QuotaResetPeriod:   SubscriptionResetNever, // 一次性，不随周期刷新
	}
	plan.NormalizeDefaults()
	if err := DB.Create(plan).Error; err != nil {
		// 并发建过了就复用
		if existing, err2 := findFreePlan(); err2 == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	// Enabled 带 gorm default:true，Create 零值 false 会被默认值覆盖，建后显式关闭
	if err := DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).
		Update("enabled", false).Error; err != nil {
		return nil, err
	}
	plan.Enabled = false
	InvalidateSubscriptionPlanCache(plan.Id)
	return plan, nil
}

func findFreePlan() (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	query := DB.Where("title = ? AND price_amount = 0", FreePlanTitle).
		Order("id asc").Limit(1).Find(&plan)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		return nil, nil
	}
	return &plan, nil
}

// GrantFreePlanToUser 给用户发放 Free 套餐。幂等：重复调用返回 nil 不重复发。
// 调用方约定：必须在注册风控（域名/IP/邮箱）通过之后调用；风险注册不发放。
func GrantFreePlanToUser(userId int) error {
	if userId <= 0 {
		return errors.New("invalid userId")
	}
	plan, err := EnsureFreePlanSeed()
	if err != nil {
		return err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "free")
		return err
	})
	if err != nil {
		// MaxPurchasePerUser=1 挡住的重复发放视为成功（幂等语义）
		if strings.Contains(err.Error(), "购买上限") {
			return nil
		}
		return err
	}
	RecordLog(userId, LogTypeSystem, fmt.Sprintf("新用户获得 %s 免费套餐（$%.0f 等值额度，1 个月有效）", FreePlanTitle, freePlanTotalUSD))
	return nil
}
