package middleware

import (
	"bytes"
	"sync"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const sessionStoreMaxResponseCaptureBytes = 10 * 1024 * 1024

type sessionStoreResponseWriter struct {
	gin.ResponseWriter
	mu        sync.Mutex
	buffer    bytes.Buffer
	truncated bool
}

func (w *sessionStoreResponseWriter) Write(data []byte) (int, error) {
	written, err := w.ResponseWriter.Write(data)
	w.capture(data[:written])
	return written, err
}

func (w *sessionStoreResponseWriter) WriteString(data string) (int, error) {
	written, err := w.ResponseWriter.WriteString(data)
	w.capture([]byte(data[:written]))
	return written, err
}

func (w *sessionStoreResponseWriter) capture(data []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := sessionStoreMaxResponseCaptureBytes - w.buffer.Len()
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(data) > remaining {
		_, _ = w.buffer.Write(data[:remaining])
		w.truncated = true
		return
	}
	_, _ = w.buffer.Write(data)
}

func (w *sessionStoreResponseWriter) snapshot() ([]byte, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buffer.Bytes()...), w.truncated
}

// SessionStoreCapture captures the client-visible request and the complete
// streamed/non-streamed response. Uploading starts only after the handler has
// finished and happens on the session-store worker pool.
func SessionStoreCapture() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil || !service.SessionStoreCaptureEnabledForRequest(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		service.BeginSessionStoreCapture(c)
		writer := &sessionStoreResponseWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		defer func() {
			body, truncated := writer.snapshot()
			service.FinishSessionStoreCapture(c, body, truncated)
		}()
		c.Next()
	}
}
