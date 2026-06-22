package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 召回邮件 step 常量
const (
	EmailSeqStepE1 = 1 // 注册当天欢迎
	EmailSeqStepE2 = 2 // D3 提醒首调
	EmailSeqStepE3 = 3 // D14 加码 bonus
	EmailSeqStepE4 = 4 // D30 最高档 bonus
)

// 发送状态
const (
	EmailSeqStatusSent    = "sent"
	EmailSeqStatusSkipped = "skipped"
	EmailSeqStatusFailed  = "failed"
)

// UserEmailSequence 召回邮件发送去重表。
// (UserId, Step) 唯一索引是幂等的硬保证:调度器重启/重复运行绝不重复发送同一 (user, step)。
type UserEmailSequence struct {
	Id     int    `json:"id" gorm:"primaryKey"`
	UserId int    `json:"user_id" gorm:"uniqueIndex:idx_user_email_seq_user_step;index"`
	Step   int    `json:"step" gorm:"uniqueIndex:idx_user_email_seq_user_step"`
	Status string `json:"status" gorm:"type:varchar(16)"`
	SentAt int64  `json:"sent_at" gorm:"bigint;index"`
}

// RecordEmailSequenceSent 幂等记录一次发送。
// 返回 (true, nil) 表示本次成功占用 (user,step) 名额、应发送;
// 返回 (false, nil) 表示已存在记录(唯一索引冲突)、绝不重发。
func RecordEmailSequenceSent(userId, step int) (bool, error) {
	rec := &UserEmailSequence{
		UserId: userId,
		Step:   step,
		Status: EmailSeqStatusSent,
		SentAt: common.GetTimestamp(),
	}
	res := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(rec)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// GetSentSteps 返回某用户已发送过的所有 step。
func GetSentSteps(userId int) ([]int, error) {
	var steps []int
	err := DB.Model(&UserEmailSequence{}).
		Where("user_id = ? AND status = ?", userId, EmailSeqStatusSent).
		Pluck("step", &steps).Error
	return steps, err
}

// HasSentStepWithinWindow 判断某用户的某 step 是否在 windowSeconds 内发送过。
// 返回 (是否在窗口内, 发送时间戳, error)。用于阶段 bonus 的有效期判定。
func HasSentStepWithinWindow(userId, step int, windowSeconds int64) (bool, int64, error) {
	var rec UserEmailSequence
	err := DB.Where("user_id = ? AND step = ? AND status = ?", userId, step, EmailSeqStatusSent).
		First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	within := common.GetTimestamp()-rec.SentAt <= windowSeconds
	return within, rec.SentAt, nil
}

// HasSentAnyStepToday 判断用户今天是否已收到过本序列任意邮件(单用户每日 1 封节流)。
func HasSentAnyStepToday(userId int) (bool, error) {
	now := common.GetTimestamp()
	startOfDay := now - (now % (24 * 3600))
	var count int64
	err := DB.Model(&UserEmailSequence{}).
		Where("user_id = ? AND status = ? AND sent_at >= ?", userId, EmailSeqStatusSent, startOfDay).
		Count(&count).Error
	return count > 0, err
}

// SetUserEmailOptOut 标记用户退订召回邮件序列(永久退出)。
func SetUserEmailOptOut(userId int) error {
	return DB.Model(&User{}).Where("id = ?", userId).
		Update("email_opt_out", true).Error
}

// GetUsersRegisteredAfter 返回注册时间在 cutoff 之后的用户(召回邮件扫描用),最多 limit 个,按注册时间升序。
func GetUsersRegisteredAfter(cutoff int64, limit int) ([]*User, error) {
	var users []*User
	err := DB.Where("created_at >= ?", cutoff).
		Order("created_at asc").Limit(limit).Find(&users).Error
	return users, err
}
