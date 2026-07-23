package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func UnmarshalJsonStr(data string, v any) error {
	return json.Unmarshal(StringToByteSlice(data), v)
}

func DecodeJson(reader io.Reader, v any) error {
	return json.NewDecoder(reader).Decode(v)
}

// UnmarshalJsonObjectStrict decodes a JSON object while rejecting duplicate
// top-level fields, fields outside the allowlist, non-object documents, and
// trailing JSON values. Keeping the token scan in this package ensures callers
// do not bypass the project's JSON wrapper contract for strict object parsing.
func UnmarshalJsonObjectStrict(data string, allowedFields map[string]struct{}) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(data))
	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("document must be an object")
	}

	seen := make(map[string]struct{}, len(allowedFields))
	unknownField := ""
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		field, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("object field name must be a string")
		}
		if _, duplicate := seen[field]; duplicate {
			return nil, fmt.Errorf("duplicate field %q", field)
		}
		seen[field] = struct{}{}
		if _, allowed := allowedFields[field]; !allowed && unknownField == "" {
			unknownField = field
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
	}

	closing, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := closing.(json.Delim); !ok || delimiter != '}' {
		return nil, errors.New("unterminated object")
	}
	if trailing, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected trailing token %v", trailing)
	}
	if unknownField != "" {
		return nil, fmt.Errorf("unknown field %q", unknownField)
	}

	var fields map[string]json.RawMessage
	if err := UnmarshalJsonStr(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// MarshalNoHTMLEscape marshals like Marshal but keeps '&', '<', '>' literal.
// Use this when the JSON is sent to an upstream HTTP API (not embedded in
// HTML) and the upstream parser doesn't normalize & back to '&' — e.g.
// when forwarding raw URLs through to providers whose URL fetcher consumes
// the JSON string byte-for-byte.
func MarshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode always appends a trailing newline; strip it.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func GetJsonType(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "unknown"
	}
	firstChar := trimmed[0]
	switch firstChar {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// JsonRawMessageToString returns JSON strings as their decoded value and other JSON values as raw text.
func JsonRawMessageToString(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] != '"' {
		return string(trimmed)
	}
	var value string
	if err := Unmarshal(trimmed, &value); err != nil {
		return string(trimmed)
	}
	return value
}
