package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSessionStoreResponseWriterCapturesStreamingWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writer := &sessionStoreResponseWriter{ResponseWriter: c.Writer}
	c.Writer = writer

	_, err := c.Writer.WriteString("data: first\n\n")
	require.NoError(t, err)
	_, err = c.Writer.Write([]byte("data: second\n\n"))
	require.NoError(t, err)
	c.Writer.Flush()

	body, truncated := writer.snapshot()
	require.False(t, truncated)
	require.Equal(t, "data: first\n\ndata: second\n\n", string(body))
	require.Equal(t, string(body), recorder.Body.String())
}
