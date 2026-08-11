package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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
	} else {
		// Final serialized-output redaction is a fail-safe for structs and other
		// JSON-marshalable values not covered by the typed recursive cases below.
		data = []byte(e.redactString(string(data)))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, err = e.sink.Write(append(data, '\n'))
	return err
}

func (e *evidence) redactValue(value any) any {
	return e.redactReflect(reflect.ValueOf(value), make(map[reflectVisit]struct{}))
}

type reflectVisit struct {
	typ reflect.Type
	ptr uintptr
}

func (e *evidence) redactReflect(value reflect.Value, seen map[reflectVisit]struct{}) any {
	if !value.IsValid() {
		return nil
	}
	if value.CanInterface() {
		switch typed := value.Interface().(type) {
		case error:
			return e.redactString(typed.Error())
		case fmt.Stringer:
			return e.redactString(typed.String())
		}
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return e.redactReflect(value.Elem(), seen)
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		visit := reflectVisit{typ: value.Type(), ptr: value.Pointer()}
		if _, exists := seen[visit]; exists {
			return "<cycle>"
		}
		seen[visit] = struct{}{}
		defer delete(seen, visit)
		return e.redactReflect(value.Elem(), seen)
	case reflect.String:
		return e.redactString(value.String())
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint()
	case reflect.Float32, reflect.Float64:
		return value.Float()
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			bytes := make([]byte, value.Len())
			for i := range bytes {
				bytes[i] = byte(value.Index(i).Uint())
			}
			return e.redactString(string(bytes))
		}
		items := make([]any, value.Len())
		for i := range items {
			items[i] = e.redactReflect(value.Index(i), seen)
		}
		return items
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		items := make(map[string]any, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			key := e.redactString(fmt.Sprint(iterator.Key().Interface()))
			items[key] = e.redactReflect(iterator.Value(), seen)
		}
		return items
	case reflect.Struct:
		items := make(map[string]any)
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			fieldType := typ.Field(i)
			field := value.Field(i)
			if !fieldType.IsExported() || !field.CanInterface() {
				continue
			}
			name := fieldType.Name
			if tag := fieldType.Tag.Get("json"); tag != "" {
				parts := strings.Split(tag, ",")
				if parts[0] == "-" {
					continue
				}
				if parts[0] != "" {
					name = parts[0]
				}
			}
			items[name] = e.redactReflect(field, seen)
		}
		return items
	default:
		if value.CanInterface() {
			return value.Interface()
		}
		return fmt.Sprint(value)
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
