package service

import (
	"os"
	"strings"
	"time"
)

const (
	defaultVideoResultSignedURLTTL = 15 * time.Minute
	maxVideoResultSignedURLTTL     = time.Hour
	defaultVideoResultRetention    = 24 * time.Hour
	defaultVideoResultFetchTimeout = 30 * time.Minute
	maxVideoResultFetchTimeout     = 30 * time.Minute
	defaultVideoResultMaxBytes     = int64(500 << 20)
)

type VideoResultStorageConfig struct {
	Bucket              string
	ServiceAccountEmail string
	SignedURLTTL        time.Duration
	Retention           time.Duration
	FetchTimeout        time.Duration
	MaxBytes            int64
}

var videoResultNow = time.Now

func CurrentVideoResultStorageConfig() VideoResultStorageConfig {
	ttl := time.Duration(getEnvInt("VIDEO_RESULT_SIGNED_URL_TTL_SECONDS", int(defaultVideoResultSignedURLTTL.Seconds()))) * time.Second
	if ttl <= 0 {
		ttl = defaultVideoResultSignedURLTTL
	} else if ttl > maxVideoResultSignedURLTTL {
		ttl = maxVideoResultSignedURLTTL
	}

	retention := time.Duration(getEnvInt("VIDEO_RESULT_RETENTION_SECONDS", int(defaultVideoResultRetention.Seconds()))) * time.Second
	if retention <= 0 {
		retention = defaultVideoResultRetention
	}

	fetchTimeout := time.Duration(getEnvInt("VIDEO_RESULT_FETCH_TIMEOUT_SECONDS", int(defaultVideoResultFetchTimeout.Seconds()))) * time.Second
	if fetchTimeout <= 0 || fetchTimeout > maxVideoResultFetchTimeout {
		fetchTimeout = defaultVideoResultFetchTimeout
	}

	maxBytes := getEnvInt64("VIDEO_RESULT_MAX_BYTES", defaultVideoResultMaxBytes)
	if maxBytes <= 0 || maxBytes > defaultVideoResultMaxBytes {
		maxBytes = defaultVideoResultMaxBytes
	}

	return VideoResultStorageConfig{
		Bucket:              strings.TrimSpace(os.Getenv("VIDEO_RESULT_STORAGE_BUCKET")),
		ServiceAccountEmail: strings.TrimSpace(os.Getenv("VIDEO_RESULT_SERVICE_ACCOUNT_EMAIL")),
		SignedURLTTL:        ttl,
		Retention:           retention,
		FetchTimeout:        fetchTimeout,
		MaxBytes:            maxBytes,
	}
}
