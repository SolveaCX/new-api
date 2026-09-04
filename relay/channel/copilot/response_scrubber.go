package copilot

import (
	"bufio"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var copilotResponseHeaders = []string{
	"X-GitHub-Request-Id",
	"X-GitHub-Copilot-Request-Te",
	"X-Copilot-Service-Request-Id",
}

func scrubCopilotResponse(resp *http.Response, stream bool) error {
	if resp == nil {
		return nil
	}
	scrubCopilotResponseHeaders(resp)
	if resp.Body == nil {
		return nil
	}
	body, err := newCopilotResponseBody(resp.Body, stream)
	if err != nil {
		return err
	}
	resp.Body = body
	return nil
}

func scrubCopilotResponseHeaders(resp *http.Response) {
	if resp == nil {
		return
	}
	for _, name := range copilotResponseHeaders {
		for key := range resp.Header {
			if strings.EqualFold(key, name) {
				delete(resp.Header, key)
			}
		}
	}
}

func newCopilotResponseBody(body io.ReadCloser, stream bool) (io.ReadCloser, error) {
	if !stream {
		data, err := io.ReadAll(body)
		if err != nil {
			_ = body.Close()
			return nil, err
		}
		return &copilotResponseBody{source: body, data: scrubCopilotUsageJSON(data)}, nil
	}

	return &copilotResponseBody{
		source: body,
		lines:  bufio.NewReader(body),
	}, nil
}

type copilotResponseBody struct {
	source io.ReadCloser
	lines  *bufio.Reader
	data   []byte
	err    error
}

func (b *copilotResponseBody) Read(p []byte) (int, error) {
	if len(b.data) == 0 && b.err == nil {
		if b.lines == nil {
			b.err = io.EOF
		} else {
			line, err := b.lines.ReadString('\n')
			if len(line) > 0 {
				b.data = scrubCopilotSSELine(line)
			}
			if err != nil {
				b.err = err
			}
		}
	}

	if len(b.data) > 0 {
		n := copy(p, b.data)
		b.data = b.data[n:]
		if len(b.data) > 0 {
			return n, nil
		}
		if b.err != nil {
			err := b.err
			b.err = nil
			return n, err
		}
		return n, nil
	}

	err := b.err
	if err == nil {
		err = io.EOF
	}
	return 0, err
}

func (b *copilotResponseBody) Close() error {
	if b.source == nil {
		return nil
	}
	return b.source.Close()
}

func scrubCopilotSSELine(line string) []byte {
	lineEnd := ""
	body := line
	if strings.HasSuffix(body, "\n") {
		lineEnd = "\n"
		body = strings.TrimSuffix(body, "\n")
		if strings.HasSuffix(body, "\r") {
			lineEnd = "\r\n"
			body = strings.TrimSuffix(body, "\r")
		}
	}
	if !strings.HasPrefix(body, "data:") {
		return []byte(line)
	}

	data := strings.TrimSpace(body[len("data:"):])
	scrubbed := scrubCopilotUsageJSON([]byte(data))
	return []byte("data: " + string(scrubbed) + lineEnd)
}

func scrubCopilotUsageJSON(data []byte) []byte {
	var payload any
	if common.Unmarshal(data, &payload) != nil {
		return data
	}
	if !removeCopilotUsage(&payload) {
		return data
	}
	scrubbed, err := common.Marshal(payload)
	if err != nil {
		return data
	}
	return scrubbed
}

func removeCopilotUsage(value *any) bool {
	switch current := (*value).(type) {
	case map[string]any:
		removed := false
		if _, ok := current["copilot_usage"]; ok {
			delete(current, "copilot_usage")
			removed = true
		}
		for key, child := range current {
			if removeCopilotUsage(&child) {
				current[key] = child
				removed = true
			}
		}
		return removed
	case []any:
		removed := false
		for index := range current {
			if removeCopilotUsage(&current[index]) {
				removed = true
			}
		}
		return removed
	default:
		return false
	}
}
