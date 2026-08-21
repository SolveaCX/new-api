package model

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/go-redis/redis/v8"
)

var ErrRegistrationRiskTokenBlocked = errors.New("token creation blocked by registration risk")

var registrationRiskBenefitClaimScript = redis.NewScript(`
local key = KEYS[1]
local window = tonumber(ARGV[1])
local max = tonumber(ARGV[2])
local member = ARGV[3]
local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1])
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
if redis.call('ZSCORE', key, member) then
  redis.call('EXPIRE', key, window)
  return 1
end
if redis.call('ZCARD', key) >= max then
  redis.call('EXPIRE', key, window)
  return 0
end
redis.call('ZADD', key, now, member)
redis.call('EXPIRE', key, window)
return 1
`)

const (
	RegistrationRiskChallenge   = 2
	RegistrationRiskBenefits    = 3
	RegistrationRiskTokens      = 5
	registrationRiskRedisPrefix = "new-api:registration-risk:"
)

// RegistrationRiskDecision is calculated before insertion and persisted on the
// user row. Raw device identifiers and emails are never stored.
type RegistrationRiskDecision struct {
	DeviceIDHash        string
	RegistrationIPHash  string
	EmailIdentityHash   string
	DeviceRegistrations int
	IPRegistrations     int
	EmailRegistrations  int
	Level               int
	Challenge           bool
	BlockBenefits       bool
	BlockTokens         bool
}

func hashRegistrationSignal(kind, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return common.GenerateHMAC(kind + ":" + value)
}

func normalizeRegistrationRiskIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if addr, err := netip.ParseAddr(ip); err == nil {
		return addr.Unmap().String()
	}
	return ip
}

func HashRegistrationDeviceID(deviceID string) string {
	return hashRegistrationSignal("device", deviceID)
}

func HashRegistrationEmail(email string) string {
	return hashRegistrationSignal("email", strings.ToLower(strings.TrimSpace(email)))
}

func HashRegistrationIP(ip string) string {
	return hashRegistrationSignal("ip", normalizeRegistrationRiskIP(ip))
}

func registrationRiskKey(kind, hash string) string {
	return registrationRiskRedisPrefix + kind + ":" + hash
}

func registrationRiskCountFromRedis(ctx context.Context, kind, hash string, cutoff int64) int {
	if hash == "" || !common.RedisEnabled || common.RDB == nil {
		return 0
	}
	count, err := common.RDB.ZCount(ctx, registrationRiskKey(kind, hash), fmt.Sprintf("%d", cutoff), "+inf").Result()
	if err != nil {
		common.SysError("count registration risk signals in redis failed: " + err.Error())
		return 0
	}
	return int(count)
}

func registrationRiskCountFromDB(kind, hash string, cutoff int64) int {
	if hash == "" || DB == nil {
		return 0
	}
	query := DB.Model(&User{}).Where("created_at >= ?", cutoff)
	switch kind {
	case "device":
		query = query.Where("device_id_hash = ?", hash)
	case "ip":
		query = query.Where("registration_ip_hash = ?", hash)
	case "email":
		query = query.Where("email_identity_hash = ?", hash)
	default:
		return 0
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		common.SysError("count registration risk signals in database failed: " + err.Error())
		return 0
	}
	return int(count)
}

func registrationRiskCount(kind, hash string, cutoff int64) int {
	dbCount := registrationRiskCountFromDB(kind, hash, cutoff)
	redisCount := registrationRiskCountFromRedis(context.Background(), kind, hash, cutoff)
	if redisCount > dbCount {
		return redisCount
	}
	return dbCount
}

// AssessRegistrationRisk counts recent accounts linked to the same device,
// IP, or normalized email. Registration remains possible; callers decide which
// benefits or API capabilities should be withheld.
func AssessRegistrationRisk(deviceID, email, registrationIP string) RegistrationRiskDecision {
	settings := system_setting.GetRegistrationSecuritySettings()
	windowHours := settings.DeviceWindowHours
	if windowHours < 1 {
		windowHours = 24
	}
	cutoff := time.Now().Unix() - int64(windowHours*3600)
	deviceHash := HashRegistrationDeviceID(deviceID)
	ipHash := HashRegistrationIP(registrationIP)
	emailHash := HashRegistrationEmail(email)
	deviceCount := registrationRiskCount("device", deviceHash, cutoff)
	ipCount := registrationRiskCount("ip", ipHash, cutoff)
	emailCount := registrationRiskCount("email", emailHash, cutoff)
	level := maxRegistrationRisk(deviceCount, ipCount, emailCount) + 1
	decision := RegistrationRiskDecision{
		DeviceIDHash: deviceHash, RegistrationIPHash: ipHash, EmailIdentityHash: emailHash,
		DeviceRegistrations: deviceCount + 1, IPRegistrations: ipCount + 1,
		EmailRegistrations: emailCount + 1, Level: level,
	}
	decision.Challenge = level >= settings.DeviceChallengeThreshold
	decision.BlockBenefits = level >= settings.DeviceBenefitBlockThreshold
	decision.BlockTokens = level >= settings.DeviceTokenBlockThreshold
	return decision
}

func maxRegistrationRisk(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

// RecordRegistrationRiskSignal is safe across application nodes. The user row
// is the durable source; Redis sorted sets improve concurrent velocity checks.
func RecordRegistrationRiskSignal(userID int, decision RegistrationRiskDecision) {
	if userID <= 0 || !common.RedisEnabled || common.RDB == nil {
		return
	}
	now := time.Now().Unix()
	member := fmt.Sprintf("%d", userID)
	ctx := context.Background()
	settings := system_setting.GetRegistrationSecuritySettings()
	for kind, hash := range map[string]string{
		"device": decision.DeviceIDHash,
		"ip":     decision.RegistrationIPHash,
		"email":  decision.EmailIdentityHash,
	} {
		if hash == "" {
			continue
		}
		key := registrationRiskKey(kind, hash)
		if err := common.RDB.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: member}).Err(); err != nil {
			common.SysError("record registration risk signal in redis failed: " + err.Error())
			continue
		}
		_ = common.RDB.Expire(ctx, key, time.Duration(settings.DeviceWindowHours+1)*time.Hour).Err()
	}
}

func claimRegistrationRiskBenefitSlot(ctx context.Context, kind, hash string, userID, max int) (bool, error) {
	if hash == "" || userID <= 0 || max <= 0 || !common.RedisEnabled || common.RDB == nil {
		return true, nil
	}
	settings := system_setting.GetRegistrationSecuritySettings()
	result, err := registrationRiskBenefitClaimScript.Run(ctx, common.RDB, []string{registrationRiskKey(kind, hash)},
		int64(settings.DeviceWindowHours*3600), max, fmt.Sprintf("%d", userID)).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func releaseRegistrationRiskBenefitSlot(ctx context.Context, kind, hash string, userID int) {
	if hash == "" || userID <= 0 || common.RDB == nil {
		return
	}
	_ = common.RDB.ZRem(ctx, registrationRiskKey(kind, hash), fmt.Sprintf("%d", userID)).Err()
}

func IsUserRegistrationRiskBenefitsBlocked(user *User) bool {
	return user != nil && user.Role < common.RoleAdminUser && user.RegistrationRiskLevel >= RegistrationRiskBenefits
}

func IsUserRegistrationRiskTokenBlocked(user *User) bool {
	return user != nil && user.Role < common.RoleAdminUser && user.RegistrationRiskLevel >= RegistrationRiskTokens
}
