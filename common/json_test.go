package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonRawMessageToString(t *testing.T) {
	tests := []struct {
		name string
		data json.RawMessage
		want string
	}{
		{
			name: "object",
			data: json.RawMessage(`{"city":"Paris","days":0,"strict":false}`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "string",
			data: json.RawMessage(`"{\"city\":\"Paris\",\"days\":0,\"strict\":false}"`),
			want: `{"city":"Paris","days":0,"strict":false}`,
		},
		{
			name: "null",
			data: json.RawMessage(`null`),
			want: "",
		},
		{
			name: "empty",
			data: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, JsonRawMessageToString(tt.data))
		})
	}
}

func TestUnmarshalJsonObjectStrict(t *testing.T) {
	allowed := map[string]struct{}{"name": {}, "metadata": {}}

	t.Run("valid nested values", func(t *testing.T) {
		fields, err := UnmarshalJsonObjectStrict(`{"name":"supplier","metadata":{"regions":["us","eu"],"enabled":true}}`, allowed)
		require.NoError(t, err)
		require.JSONEq(t, `"supplier"`, string(fields["name"]))
		require.JSONEq(t, `{"regions":["us","eu"],"enabled":true}`, string(fields["metadata"]))
	})

	t.Run("duplicate allowed field", func(t *testing.T) {
		_, err := UnmarshalJsonObjectStrict(`{"name":"first","name":"second"}`, allowed)
		require.EqualError(t, err, `duplicate field "name"`)
	})

	t.Run("duplicate unknown field takes precedence", func(t *testing.T) {
		_, err := UnmarshalJsonObjectStrict(`{"name":"supplier","extra":1,"extra":2}`, allowed)
		require.EqualError(t, err, `duplicate field "extra"`)
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := UnmarshalJsonObjectStrict(`{"name":"supplier","extra":1}`, allowed)
		require.EqualError(t, err, `unknown field "extra"`)
	})

	t.Run("non-object document", func(t *testing.T) {
		_, err := UnmarshalJsonObjectStrict(`["supplier"]`, allowed)
		require.EqualError(t, err, "document must be an object")
	})

	t.Run("trailing document", func(t *testing.T) {
		_, err := UnmarshalJsonObjectStrict(`{"name":"supplier"} {"name":"second"}`, allowed)
		require.EqualError(t, err, "unexpected trailing token {")
	})
}
