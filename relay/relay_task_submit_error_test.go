package relay

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

func TestTaskSubmitStatusErrorScrubsBytePlusNonOKBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body: io.NopCloser(strings.NewReader(
			`{"error":"BytePlus bytepluses.com endpoint ep-test-secret rejected"}`)),
	}

	taskErr := taskSubmitStatusError(constant.TaskPlatform("107"), resp)
	if taskErr == nil {
		t.Fatal("expected task submit status error")
	}
	for _, marker := range []string{"bytepluses.com", "BytePlus", "ep-test-secret"} {
		if strings.Contains(taskErrorText(taskErr), marker) {
			t.Fatalf("non-200 submit error leaked %q in %q", marker, taskErrorText(taskErr))
		}
	}
}

func TestTaskSubmitStatusErrorScrubsBytePlusRawNonOKBodyWithoutBrandMarkers(t *testing.T) {
	const rawSentinel = "upstream-json-shape-sentinel"
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"` + rawSentinel + `","retry_after":30}}`)),
	}

	taskErr := taskSubmitStatusError(constant.TaskPlatform("107"), resp)
	if taskErr == nil {
		t.Fatal("expected task submit status error")
	}
	if taskErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", taskErr.StatusCode, http.StatusTooManyRequests)
	}
	if strings.Contains(taskErrorText(taskErr), rawSentinel) {
		t.Fatalf("non-200 submit error leaked raw upstream body in %q", taskErrorText(taskErr))
	}
	if taskErr.Message != "task failed at upstream provider" {
		t.Fatalf("message = %q, want fixed generic message", taskErr.Message)
	}
}

func TestTaskSubmitStatusErrorPreservesDoubaoBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(`{"error":"doubao upstream details"}`)),
	}

	taskErr := taskSubmitStatusError(constant.TaskPlatform("52"), resp)
	if taskErr == nil {
		t.Fatal("expected task submit status error")
	}
	if !strings.Contains(taskErr.Message, "doubao upstream details") {
		t.Fatalf("doubao submit error was unexpectedly scrubbed: %q", taskErr.Message)
	}
}

func taskErrorText(taskErr *dto.TaskError) string {
	if taskErr == nil {
		return ""
	}
	text := taskErr.Message
	if taskErr.Error != nil {
		text += " " + taskErr.Error.Error()
	}
	return text
}
