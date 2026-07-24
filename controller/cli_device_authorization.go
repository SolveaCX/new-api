package controller

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	cliDeviceAuthorizationTTL      = 10 * 60
	cliDeviceAuthorizationInterval = 5
	defaultCliClientName           = "flatkey-cli"
	cliUserCodeChars               = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

type createCliDeviceAuthorizationRequest struct {
	ClientName    string `json:"client_name"`
	ClientVersion string `json:"client_version"`
	DeviceId      string `json:"device_id"`
}

type pollCliDeviceAuthorizationRequest struct {
	DeviceCode string `json:"device_code"`
}

type approveCliDeviceAuthorizationRequest struct {
	UserCode string `json:"user_code"`
}

func CreateCliDeviceAuthorization(c *gin.Context) {
	request := createCliDeviceAuthorizationRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(request.DeviceId) == "" {
		common.ApiError(c, errors.New("missing device_id"))
		return
	}

	deviceCode, err := common.GenerateRandomCharsKey(64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userCode, err := generateCliUserCode()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	clientName := strings.TrimSpace(request.ClientName)
	if clientName == "" {
		clientName = defaultCliClientName
	}
	auth := model.CliDeviceAuthorization{
		DeviceCodeHash: hashCliDeviceCode(deviceCode),
		UserCodeHash:   hashCliUserCode(userCode),
		Status:         model.CliDeviceAuthorizationStatusPending,
		ClientName:     clientName,
		ClientVersion:  strings.TrimSpace(request.ClientVersion),
		DeviceIdHash:   hashCliDeviceId(request.DeviceId),
		CreatedTime:    now,
		ExpiresAt:      now + cliDeviceAuthorizationTTL,
	}
	if err := model.CreateCliDeviceAuthorization(&auth); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"device_code":               deviceCode,
		"user_code":                 userCode,
		"verification_uri":          buildCliVerificationURL(c, ""),
		"verification_uri_complete": buildCliVerificationURL(c, userCode),
		"expires_in":                cliDeviceAuthorizationTTL,
		"interval":                  cliDeviceAuthorizationInterval,
	})
}

func PollCliDeviceAuthorization(c *gin.Context) {
	request := pollCliDeviceAuthorizationRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	auth, err := model.GetCliDeviceAuthorizationByDeviceCodeHash(hashCliDeviceCode(request.DeviceCode))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, errors.New("invalid device_code"))
			return
		}
		common.ApiError(c, err)
		return
	}

	now := common.GetTimestamp()
	if auth.Status == model.CliDeviceAuthorizationStatusPending && auth.ExpiresAt <= now {
		if err := model.ExpireCliDeviceAuthorization(auth.Id); err != nil {
			common.ApiError(c, err)
			return
		}
		auth.Status = model.CliDeviceAuthorizationStatusExpired
	}

	data := gin.H{"status": auth.Status}
	if auth.Status == model.CliDeviceAuthorizationStatusApproved && auth.TokenId > 0 {
		token, err := model.GetTokenById(auth.TokenId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		data["api_key"] = tokenKeyWithPrefix(token.GetFullKey())
		data["token_id"] = token.Id
		data["user_id"] = token.UserId
		data["expires_at"] = auth.ExpiresAt
		data["consumed_at"] = now
		_ = model.DB.Model(auth).Update("consumed_at", now).Error
	}
	common.ApiSuccess(c, data)
}

func GetCliDeviceAuthorization(c *gin.Context) {
	auth, ok := getCliAuthorizationForUserCode(c, c.Param("user_code"))
	if !ok {
		return
	}
	common.ApiSuccess(c, cliAuthorizationResponse(auth, common.GetTimestamp()))
}

func ApproveCliDeviceAuthorization(c *gin.Context) {
	request := approveCliDeviceAuthorizationRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	auth, ok := getCliAuthorizationForUserCode(c, request.UserCode)
	if !ok {
		return
	}
	now := common.GetTimestamp()
	if auth.Status != model.CliDeviceAuthorizationStatusPending {
		common.ApiSuccess(c, cliAuthorizationResponse(auth, now))
		return
	}
	if auth.ExpiresAt <= now {
		if err := model.ExpireCliDeviceAuthorization(auth.Id); err != nil {
			common.ApiError(c, err)
			return
		}
		auth.Status = model.CliDeviceAuthorizationStatusExpired
		common.ApiSuccess(c, cliAuthorizationResponse(auth, now))
		return
	}

	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		common.SysLog("failed to generate cli token key: " + err.Error())
		return
	}
	token := model.Token{
		Name:             "Flatkey CLI",
		Key:              key,
		CreatedTime:      now,
		AccessedTime:     now,
		ExpiredTime:      -1,
		UnlimitedQuota:   true,
		Source:           model.TokenSourceCLI,
		DeviceIdHash:     auth.DeviceIdHash,
		ClientName:       auth.ClientName,
		ClientVersion:    auth.ClientVersion,
		LastUsedClientAt: now,
	}
	cleanToken, err := buildTokenForInsert(c, token, key)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyInitialTokenDefaults(c, &cleanToken); err != nil {
		common.ApiError(c, err)
		return
	}
	approval, err := model.ApproveCliDeviceAuthorizationWithToken(
		auth.Id,
		c.GetInt("id"),
		cleanToken,
		operation_setting.GetMaxUserTokens(),
		model.InviteRewardTriggerManualTokenCreate,
		now,
	)
	if err != nil {
		if errors.Is(err, model.ErrUserTokenLimitReached) {
			common.ApiErrorI18n(c, i18n.MsgTokenLimitReached, map[string]any{"Max": operation_setting.GetMaxUserTokens()})
			return
		}
		common.ApiError(c, err)
		return
	}
	if approval == nil {
		common.ApiError(c, errors.New("failed to create cli token"))
		return
	}
	common.ApiSuccess(c, cliAuthorizationResponse(&approval.Authorization, now))
}

func DenyCliDeviceAuthorization(c *gin.Context) {
	request := approveCliDeviceAuthorizationRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	auth, ok := getCliAuthorizationForUserCode(c, request.UserCode)
	if !ok {
		return
	}
	now := common.GetTimestamp()
	if auth.Status != model.CliDeviceAuthorizationStatusPending {
		common.ApiSuccess(c, cliAuthorizationResponse(auth, now))
		return
	}
	if auth.ExpiresAt <= now {
		if err := model.ExpireCliDeviceAuthorization(auth.Id); err != nil {
			common.ApiError(c, err)
			return
		}
		auth.Status = model.CliDeviceAuthorizationStatusExpired
		common.ApiSuccess(c, cliAuthorizationResponse(auth, now))
		return
	}
	if err := model.DenyCliDeviceAuthorization(auth.Id, now); err != nil {
		common.ApiError(c, err)
		return
	}
	auth.Status = model.CliDeviceAuthorizationStatusDenied
	auth.ApprovedAt = now
	common.ApiSuccess(c, cliAuthorizationResponse(auth, now))
}

func getCliAuthorizationForUserCode(c *gin.Context, userCode string) (*model.CliDeviceAuthorization, bool) {
	normalized := normalizeCliUserCode(userCode)
	if normalized == "" {
		common.ApiError(c, errors.New("missing user_code"))
		return nil, false
	}
	auth, err := model.GetCliDeviceAuthorizationByUserCodeHash(hashCliUserCode(normalized))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "authorization request not found"})
			return nil, false
		}
		common.ApiError(c, err)
		return nil, false
	}
	return auth, true
}

func cliAuthorizationResponse(auth *model.CliDeviceAuthorization, now int64) gin.H {
	status := auth.Status
	if status == model.CliDeviceAuthorizationStatusPending && auth.ExpiresAt <= now {
		status = model.CliDeviceAuthorizationStatusExpired
	}
	return gin.H{
		"status":         status,
		"client_name":    auth.ClientName,
		"client_version": auth.ClientVersion,
		"expires_at":     auth.ExpiresAt,
		"approved_at":    auth.ApprovedAt,
	}
}

func generateCliUserCode() (string, error) {
	code := make([]byte, 8)
	max := big.NewInt(int64(len(cliUserCodeChars)))
	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		code[i] = cliUserCodeChars[n.Int64()]
	}
	return string(code[:4]) + "-" + string(code[4:]), nil
}

func normalizeCliUserCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

func hashCliUserCode(code string) string {
	return common.GenerateHMAC("cli_user_code:" + normalizeCliUserCode(code))
}

func hashCliDeviceCode(code string) string {
	return common.GenerateHMAC("cli_device_code:" + strings.TrimSpace(code))
}

func hashCliDeviceId(deviceId string) string {
	return common.GenerateHMAC("cli_device_id:" + strings.TrimSpace(deviceId))
}

func tokenKeyWithPrefix(key string) string {
	if strings.HasPrefix(key, "sk-") {
		return key
	}
	return "sk-" + key
}

func buildCliVerificationURL(c *gin.Context, userCode string) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	base := scheme + "://" + c.Request.Host + "/cli/authorize"
	if userCode == "" {
		return base
	}
	return base + "?user_code=" + userCode
}
