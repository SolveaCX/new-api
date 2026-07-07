package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const logRequestSampleRedactionVersion = "v1"
const redactedValue = "[redacted]"
const droppedValue = "[dropped]"
const truncatedSuffix = "...[truncated]"
const maxDepthValue = "[max_depth_exceeded]"

const logRequestSampleAsyncQueueSize = 1024
const logRequestSampleAsyncWorkers = 2
const logRequestSampleInsertTimeout = 2 * time.Second

type LogRequestSample struct {
	Id               int     `json:"id"`
	LogId            int     `json:"log_id" gorm:"uniqueIndex;not null"`
	RequestId        string  `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	UserId           int     `json:"user_id" gorm:"index"`
	CreatedAt        int64   `json:"created_at" gorm:"bigint;index"`
	ModelName        string  `json:"model_name" gorm:"index;default:''"`
	TokenId          int     `json:"token_id" gorm:"default:0;index"`
	UserGroup        string  `json:"user_group" gorm:"index;default:''"`
	RequestPath      string  `json:"request_path" gorm:"type:varchar(255);default:''"`
	RequestParams    string  `json:"request_params" gorm:"type:text"`
	RequestBodySize  int64   `json:"request_body_size" gorm:"default:0"`
	SampleRate       float64 `json:"sample_rate" gorm:"default:0"`
	RedactionVersion string  `json:"redaction_version" gorm:"type:varchar(32);default:''"`
}

var logRequestSampleRandomFloat = func() float64 {
	return rand.Float64()
}

var logRequestSampleAsyncQueue = make(chan func(), logRequestSampleAsyncQueueSize)
var logRequestSampleAsyncOnce sync.Once
var logRequestSampleAsyncRunner = enqueueLogRequestSampleAsync

var credentialLikePattern = regexp.MustCompile(`(?i)\b(sk-[a-z0-9_-]{12,}|AIza[0-9A-Za-z_-]{20,}|AKIA[0-9A-Z]{16}|bearer\s+[a-z0-9._~+/=-]{12,}|eyJ[a-z0-9_-]+\.[a-z0-9_-]+\.[a-z0-9_-]+|(?:api[-_]?key|access[-_]?token|refresh[-_]?token|id[-_]?token|secret|password|authorization|bearer)\s*[:=]\s*["']?[^"'\s,}]+)`)
var urlLikePattern = regexp.MustCompile(`https?://[^\s"'<>{}\\]+`)

func enqueueLogRequestSampleAsync(fn func()) {
	logRequestSampleAsyncOnce.Do(startLogRequestSampleAsyncWorkers)
	select {
	case logRequestSampleAsyncQueue <- fn:
	default:
		common.SysError("log request sample async queue full; dropping sample")
	}
}

func startLogRequestSampleAsyncWorkers() {
	for i := 0; i < logRequestSampleAsyncWorkers; i++ {
		gopool.Go(func() {
			for fn := range logRequestSampleAsyncQueue {
				func() {
					defer func() {
						if r := recover(); r != nil {
							common.SysError(fmt.Sprintf("log request sample async worker panic: %v", r))
						}
					}()
					fn()
				}()
			}
		})
	}
}

func maybeRecordLogRequestSample(c *gin.Context, userId int, params RecordConsumeLogParams, log *Log) {
	snapshot := operation_setting.GetLogRequestSamplingRuntimeSnapshot()
	if !snapshot.Enabled {
		return
	}
	if c == nil || c.Request == nil || log == nil || log.Id <= 0 {
		return
	}
	if !common.GetContextKeyBool(c, constant.ContextKeyRequestSamplingEligible) {
		return
	}
	if params.Other != nil {
		if violationFee, ok := params.Other["violation_fee"].(bool); ok && violationFee {
			return
		}
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if userGroup == "" || !snapshot.Groups[userGroup] {
		return
	}
	requestPath := c.Request.URL.Path
	if !isLogRequestSampleEligiblePath(snapshot, requestPath) {
		return
	}
	if isGeminiRequestSamplePath(requestPath) && !isGeminiTextRequestSamplePath(requestPath) {
		return
	}
	if snapshot.SampleRate <= 0 || logRequestSampleRandomFloat() >= snapshot.SampleRate {
		return
	}
	if contentType := c.GetHeader("Content-Type"); contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		logger.LogError(c, "failed to get request body storage for sampling: "+err.Error())
		return
	}
	if storage.Size() <= 0 || storage.Size() > snapshot.MaxBodyBytes {
		_, _ = storage.Seek(0, io.SeekStart)
		return
	}
	body, err := storage.Bytes()
	if err != nil {
		logger.LogError(c, "failed to read request body for sampling: "+err.Error())
		_, _ = storage.Seek(0, io.SeekStart)
		return
	}
	_, _ = storage.Seek(0, io.SeekStart)

	paramsJSON, err := sanitizeLogRequestSampleBody(body, snapshot)
	if err != nil {
		return
	}
	sample := LogRequestSample{
		LogId:            log.Id,
		RequestId:        log.RequestId,
		UserId:           userId,
		CreatedAt:        log.CreatedAt,
		ModelName:        params.ModelName,
		TokenId:          params.TokenId,
		UserGroup:        userGroup,
		RequestPath:      requestPath,
		RequestParams:    paramsJSON,
		RequestBodySize:  storage.Size(),
		SampleRate:       snapshot.SampleRate,
		RedactionVersion: logRequestSampleRedactionVersion,
	}
	logRequestSampleAsyncRunner(func() {
		ctx, cancel := context.WithTimeout(context.Background(), logRequestSampleInsertTimeout)
		defer cancel()
		if err := insertLogRequestSample(ctx, sample); err != nil {
			common.SysError("failed to insert log request sample: " + err.Error())
		}
	})
}

func isLogRequestSampleEligiblePath(snapshot operation_setting.LogRequestSamplingSnapshot, path string) bool {
	if _, ok := snapshot.EligibleExactPaths[path]; ok {
		return true
	}
	for _, prefix := range snapshot.EligiblePathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func insertLogRequestSample(ctx context.Context, sample LogRequestSample) error {
	if LOG_DB == nil {
		return nil
	}
	err := LOG_DB.WithContext(ctx).Create(&sample).Error
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
		return nil
	}
	return err
}

func sanitizeLogRequestSampleBody(body []byte, snapshot operation_setting.LogRequestSamplingSnapshot) (string, error) {
	var value interface{}
	if err := common.Unmarshal(body, &value); err != nil {
		return "", err
	}
	sanitized, err := sanitizeLogRequestSampleValue(value, "", 0, snapshot)
	if err != nil {
		return "", err
	}
	out, err := common.Marshal(sanitized)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func sanitizeLogRequestSampleValue(value interface{}, fieldName string, depth int, snapshot operation_setting.LogRequestSamplingSnapshot) (interface{}, error) {
	if depth > snapshot.MaxJSONDepth {
		return maxDepthValue, nil
	}
	if isSensitiveSampleField(fieldName) {
		return redactedValue, nil
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			cleaned, err := sanitizeLogRequestSampleValue(child, key, depth+1, snapshot)
			if err != nil {
				return nil, err
			}
			out[key] = cleaned
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, child := range typed {
			cleaned, err := sanitizeLogRequestSampleValue(child, fieldName, depth+1, snapshot)
			if err != nil {
				return nil, err
			}
			out = append(out, cleaned)
		}
		return out, nil
	case string:
		return sanitizeLogRequestSampleString(fieldName, typed, snapshot), nil
	default:
		return value, nil
	}
}

func sanitizeLogRequestSampleString(fieldName string, value string, snapshot operation_setting.LogRequestSamplingSnapshot) string {
	if snapshot.DropBinaryPayloads && shouldDropLogRequestSampleString(fieldName, value) {
		return droppedValue
	}
	if isURLSampleField(fieldName) && urlLikePattern.MatchString(value) {
		return redactedValue
	}
	value = urlLikePattern.ReplaceAllString(value, redactedValue)
	value = credentialLikePattern.ReplaceAllString(value, redactedValue)
	if len(value) > snapshot.MaxStringBytes {
		return value[:snapshot.MaxStringBytes] + truncatedSuffix
	}
	return value
}

func isSensitiveSampleField(fieldName string) bool {
	name := strings.ToLower(strings.TrimSpace(fieldName))
	if name == "" {
		return false
	}
	name = strings.NewReplacer("-", "_", ".", "_").Replace(name)
	compactName := strings.ReplaceAll(name, "_", "")
	switch name {
	case "api_key", "apikey", "x_api_key", "authorization", "proxy_authorization", "auth", "bearer", "password", "secret", "client_secret", "access_token", "refresh_token", "id_token", "token", "key", "private_key", "cookie", "set_cookie", "session", "session_id", "credential", "credentials":
		return true
	}
	switch compactName {
	case "apikey", "xapikey", "proxyauthorization", "clientsecret", "accesstoken", "refreshtoken", "idtoken", "privatekey", "setcookie", "sessionid", "credential", "credentials":
		return true
	}
	if strings.HasSuffix(compactName, "apikey") ||
		strings.HasSuffix(compactName, "accesskey") ||
		strings.HasSuffix(compactName, "privatekey") ||
		strings.HasSuffix(compactName, "token") ||
		strings.HasSuffix(compactName, "secret") ||
		strings.HasSuffix(compactName, "authorization") ||
		strings.HasSuffix(compactName, "cookie") ||
		strings.HasSuffix(compactName, "session") ||
		strings.HasSuffix(compactName, "credential") ||
		strings.HasSuffix(compactName, "credentials") {
		return true
	}
	return strings.HasSuffix(name, "_api_key") ||
		strings.HasSuffix(name, "_key") ||
		strings.HasSuffix(name, "_token") ||
		strings.HasSuffix(name, "_secret") ||
		strings.HasSuffix(name, "_authorization") ||
		strings.HasSuffix(name, "_access_token") ||
		strings.HasSuffix(name, "_refresh_token") ||
		strings.HasSuffix(name, "_id_token") ||
		strings.HasSuffix(name, "_cookie") ||
		strings.HasSuffix(name, "_session") ||
		strings.HasSuffix(name, "_credential") ||
		strings.HasSuffix(name, "_credentials")
}

func isURLSampleField(fieldName string) bool {
	name := strings.ToLower(strings.TrimSpace(fieldName))
	return name == "url" ||
		name == "uri" ||
		name == "href" ||
		strings.HasSuffix(name, "_url") ||
		strings.HasSuffix(name, "_uri")
}

func shouldDropLogRequestSampleString(fieldName string, value string) bool {
	lowerField := strings.ToLower(fieldName)
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(lowerValue, "data:") {
		return true
	}
	if strings.Contains(lowerField, "image") ||
		strings.Contains(lowerField, "audio") ||
		strings.Contains(lowerField, "video") ||
		strings.Contains(lowerField, "file") ||
		strings.Contains(lowerField, "binary") {
		return true
	}
	if len(value) >= 512 && looksLikeBase64Blob(value) {
		return true
	}
	return false
}

func looksLikeBase64Blob(value string) bool {
	var valid int
	for _, r := range value {
		if unicode.IsSpace(r) {
			continue
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' {
			valid++
			continue
		}
		return false
	}
	return valid >= 512
}

func CleanupOldLogRequestSamples(targetTimestamp int64, limit int) (int64, error) {
	return deleteOldLogRequestSamples(context.Background(), targetTimestamp, limit, 0)
}

func CleanupOldLogRequestSamplesWithContext(ctx context.Context, targetTimestamp int64, limit int, maxBatches int) (int64, error) {
	return deleteOldLogRequestSamples(ctx, targetTimestamp, limit, maxBatches)
}

func CleanupOldLogRequestSamplesWithBatchLimit(targetTimestamp int64, limit int, maxBatches int) (int64, error) {
	return deleteOldLogRequestSamples(context.Background(), targetTimestamp, limit, maxBatches)
}

func deleteOldLogRequestSamples(ctx context.Context, targetTimestamp int64, limit int, maxBatches int) (int64, error) {
	if LOG_DB == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	db := LOG_DB.WithContext(ctx)
	var total int64
	for batch := 0; maxBatches <= 0 || batch < maxBatches; batch++ {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		var ids []int
		if err := db.Model(&LogRequestSample{}).Where("created_at < ?", targetTimestamp).Order("id").Limit(limit).Pluck("id", &ids).Error; err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		result := db.Where("id IN ?", ids).Delete(&LogRequestSample{})
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
		if len(ids) < limit || result.RowsAffected < int64(limit) {
			return total, nil
		}
	}
	return total, nil
}

func isGeminiTextRequestSamplePath(path string) bool {
	actionIndex := strings.LastIndex(path, ":")
	if actionIndex < 0 || actionIndex == len(path)-1 {
		return false
	}
	action := path[actionIndex+1:]
	return action == "generateContent" || action == "streamGenerateContent"
}

func isGeminiRequestSamplePath(path string) bool {
	return strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/")
}

func deleteLogRequestSamplesByLogIDs(tx *gorm.DB, ids []int) error {
	if len(ids) == 0 || LOG_DB == nil {
		return nil
	}
	if tx == nil {
		tx = LOG_DB
	}
	if err := tx.Where("log_id IN ?", ids).Delete(&LogRequestSample{}).Error; err != nil {
		return fmt.Errorf("delete log request samples: %w", err)
	}
	return nil
}
