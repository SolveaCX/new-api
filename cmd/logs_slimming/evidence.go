package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type evidenceSink interface {
	Write([]byte) (int, error)
}

type evidence struct {
	mu      sync.Mutex
	sink    evidenceSink
	secrets []string
}

func newEvidence(sink evidenceSink, secrets []string) *evidence {
	filtered := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if secret != "" {
			filtered = append(filtered, secret)
		}
	}
	return &evidence{sink: sink, secrets: filtered}
}

func (e *evidence) emit(event string, fields map[string]any) error {
	record := make(map[string]any, len(fields)+2)
	record["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	record["event"] = event
	for key, value := range fields {
		record[key] = e.redactValue(value)
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(record)
	data := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	if err != nil {
		data = []byte(fmt.Sprintf(`{"timestamp":%q,"event":"evidence_marshal_failed","error":%q}`, time.Now().UTC().Format(time.RFC3339Nano), e.redactString(err.Error())))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, err = e.sink.Write(append(data, '\n'))
	return err
}

func (e *evidence) redactValue(value any) any {
	switch typed := value.(type) {
	case error:
		return e.redactString(typed.Error())
	case string:
		return e.redactString(typed)
	default:
		return typed
	}
}

func (e *evidence) redactString(value string) string {
	for _, secret := range e.secrets {
		value = strings.ReplaceAll(value, secret, "<redacted>")
	}
	return value
}

type memoryEvidence struct {
	mu sync.Mutex
	b  strings.Builder
}

func (m *memoryEvidence) Write(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.b.Write(data)
}

func (m *memoryEvidence) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.b.String()
}

var _ io.Writer = (*memoryEvidence)(nil)
