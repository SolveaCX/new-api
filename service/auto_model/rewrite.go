package auto_model

import (
	"fmt"
	"strings"

	"github.com/tidwall/sjson"
)

// RewriteModel changes only the top-level model field in the original JSON.
// Unknown and explicitly-zero request fields remain untouched.
func RewriteModel(raw []byte, realModel string) ([]byte, error) {
	if strings.TrimSpace(realModel) == "" {
		return nil, fmt.Errorf("real model is required")
	}
	rewritten, err := sjson.SetBytes(raw, "model", realModel)
	if err != nil {
		return nil, fmt.Errorf("rewrite request model: %w", err)
	}
	return rewritten, nil
}
