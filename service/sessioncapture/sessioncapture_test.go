package sessioncapture

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

func TestNewWireMessageMatchesConsumerContract(t *testing.T) {
	transcript := []byte("{\"session_id\":\"session-1\"}\n")
	startedAt := time.Date(2026, 9, 18, 1, 2, 3, 0, time.FixedZone("test", 8*60*60))
	endedAt := startedAt.Add(5 * time.Second)
	message := newWireMessage(Meta{
		SessionID: "session-1",
		TenantID:  "flatkey",
		UserID:    "42",
		Model:     "claude-sonnet-4-5",
		Status:    "ok",
		StartedAt: startedAt,
		EndedAt:   endedAt,
		TokensIn:  10,
		TokensOut: 20,
		CostUSD:   0.25,
	}, transcript, "flatkey/20260917/session-1.jsonl.zst", startedAt)

	sum := sha256.Sum256(transcript)
	require.Equal(t, "session-1", message.SessionID)
	require.Equal(t, "flatkey", message.TenantID)
	require.Equal(t, "2026-09-17T17:02:03Z", message.StartedAt)
	require.Equal(t, "2026-09-17T17:02:08Z", message.EndedAt)
	require.Equal(t, len(transcript), message.RawBytes)
	require.Equal(t, hex.EncodeToString(sum[:]), message.SHA256)
	require.Equal(t, "flatkey/20260917/session-1.jsonl.zst", message.GCSKey)
}

func TestCaptureCopiesTranscriptBeforeAsyncDelivery(t *testing.T) {
	originalEnabled := enabled
	originalAsync := asyncRunner
	originalDeliver := deliverRunner
	enabled = true
	var pending func()
	asyncRunner = func(fn func()) { pending = fn }
	var delivered []byte
	deliverRunner = func(_ Meta, transcript []byte) {
		delivered = append([]byte(nil), transcript...)
	}
	t.Cleanup(func() {
		enabled = originalEnabled
		asyncRunner = originalAsync
		deliverRunner = originalDeliver
	})

	transcript := []byte("original")
	Capture(Meta{SessionID: "session-1", TenantID: "flatkey"}, transcript)
	require.NotNil(t, pending)
	copy(transcript, "modified")
	pending()
	require.Equal(t, "original", string(delivered))
}

func TestCaptureRejectsUnsafeObjectSegments(t *testing.T) {
	originalEnabled := enabled
	originalAsync := asyncRunner
	enabled = true
	called := false
	asyncRunner = func(func()) { called = true }
	t.Cleanup(func() {
		enabled = originalEnabled
		asyncRunner = originalAsync
	})

	Capture(Meta{SessionID: "../session", TenantID: "flatkey"}, []byte("data"))
	Capture(Meta{SessionID: "session", TenantID: "tenant/path"}, []byte("data"))
	require.False(t, called)
}

func TestIsPreconditionFailed(t *testing.T) {
	require.True(t, isPreconditionFailed(&googleapi.Error{Code: 412}))
	require.False(t, isPreconditionFailed(&googleapi.Error{Code: 500}))
}
