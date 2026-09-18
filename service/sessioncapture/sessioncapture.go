// Package sessioncapture publishes finished Claude sessions to the Flatkey
// session pipeline (GCS staging plus Pub/Sub). Capture is best-effort and never
// returns an error to the user request path.
package sessioncapture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/QuantumNous/new-api/common"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/api/googleapi"
)

const captureTimeout = 5 * time.Second

// Meta is the per-session metadata recorded by the NAS consumer.
type Meta struct {
	SessionID string
	TenantID  string
	UserID    string
	Model     string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
	TokensIn  int64
	TokensOut int64
	CostUSD   float64
}

type wireMsg struct {
	SessionID string  `json:"session_id"`
	TenantID  string  `json:"tenant_id"`
	UserID    string  `json:"user_id,omitempty"`
	Model     string  `json:"model,omitempty"`
	Status    string  `json:"status,omitempty"`
	StartedAt string  `json:"started_at"`
	EndedAt   string  `json:"ended_at,omitempty"`
	TokensIn  int64   `json:"tokens_in,omitempty"`
	TokensOut int64   `json:"tokens_out,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	RawBytes  int     `json:"raw_bytes"`
	SHA256    string  `json:"sha256"`
	GCSKey    string  `json:"gcs_key"`
}

var (
	enabled = strings.EqualFold(strings.TrimSpace(os.Getenv("SESSION_CAPTURE_ENABLED")), "true")
	project = strings.TrimSpace(os.Getenv("GCP_PROJECT"))
	topicID = envOr("PUBSUB_TOPIC", "flatkey-sessions")
	bucket  = envOr("GCS_STAGING", "flatkey-session-staging-vocai-prod")

	setupMu sync.Mutex
	gcsCli  *storage.Client
	psCli   *pubsub.Client
	psTopic *pubsub.Topic
	encoder *zstd.Encoder

	asyncRunner   = func(fn func()) { go fn() }
	deliverRunner = deliver
)

func envOr(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func setup(ctx context.Context) error {
	setupMu.Lock()
	defer setupMu.Unlock()
	if gcsCli != nil && psTopic != nil && encoder != nil {
		return nil
	}
	if project == "" {
		return errors.New("GCP_PROJECT is empty")
	}

	newGCS, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	newPS, err := pubsub.NewClient(ctx, project)
	if err != nil {
		_ = newGCS.Close()
		return err
	}
	newEncoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		_ = newGCS.Close()
		_ = newPS.Close()
		return err
	}

	gcsCli = newGCS
	psCli = newPS
	psTopic = newPS.Topic(topicID)
	encoder = newEncoder
	return nil
}

// Capture fires and forgets a finished session. It copies the transcript
// before returning so the caller may release its response buffer immediately.
func Capture(meta Meta, transcript []byte) {
	if !enabled || !safeObjectSegment(meta.SessionID) || !safeObjectSegment(meta.TenantID) || len(transcript) == 0 {
		return
	}
	transcript = append([]byte(nil), transcript...)
	asyncRunner(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				common.SysError(fmt.Sprintf("session capture recovered: %v", recovered))
			}
		}()
		deliverRunner(meta, transcript)
	})
}

func deliver(meta Meta, transcript []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), captureTimeout)
	defer cancel()
	if err := setup(ctx); err != nil {
		common.SysError("session capture init failed: " + err.Error())
		return
	}

	startedAt := meta.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	key := fmt.Sprintf("%s/%s/%s.jsonl.zst", meta.TenantID, startedAt.UTC().Format("20060102"), meta.SessionID)
	compressed := encoder.EncodeAll(transcript, nil)
	if err := writeObject(ctx, key, compressed); err != nil {
		common.SysError("session capture GCS write failed: " + err.Error())
		return
	}

	payload, err := common.Marshal(newWireMessage(meta, transcript, key, startedAt))
	if err != nil {
		common.SysError("session capture metadata marshal failed: " + err.Error())
		return
	}
	result := psTopic.Publish(ctx, &pubsub.Message{Data: payload})
	if _, err := result.Get(ctx); err != nil {
		common.SysError("session capture Pub/Sub publish failed: " + err.Error())
	}
}

func writeObject(ctx context.Context, key string, compressed []byte) error {
	object := gcsCli.Bucket(bucket).Object(key).If(storage.Conditions{DoesNotExist: true})
	writer := object.NewWriter(ctx)
	writer.ContentType = "application/zstd"
	if _, err := writer.Write(compressed); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil && !isPreconditionFailed(err) {
		return err
	}
	return nil
}

func isPreconditionFailed(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 412
}

func newWireMessage(meta Meta, transcript []byte, key string, startedAt time.Time) wireMsg {
	sum := sha256.Sum256(transcript)
	return wireMsg{
		SessionID: meta.SessionID,
		TenantID:  meta.TenantID,
		UserID:    meta.UserID,
		Model:     meta.Model,
		Status:    meta.Status,
		StartedAt: startedAt.UTC().Format(time.RFC3339),
		EndedAt:   fmtTime(meta.EndedAt),
		TokensIn:  meta.TokensIn,
		TokensOut: meta.TokensOut,
		CostUSD:   meta.CostUSD,
		RawBytes:  len(transcript),
		SHA256:    hex.EncodeToString(sum[:]),
		GCSKey:    key,
	}
}

func safeObjectSegment(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func fmtTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// Enabled reports the process-level feature flag state.
func Enabled() bool { return enabled }
