package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestVirtualCharacterRealPersonProviderStaticContract(t *testing.T) {
	p := virtualCharacterRealPersonProvider{}
	require.False(t, p.RequiresCallback())
	require.Equal(t, int64(30*60), p.VerificationTTLSeconds())
}

func TestVirtualCharacterRealPersonAssetStatus(t *testing.T) {
	cases := map[string]struct {
		status string
		want   string
		ok     bool
	}{
		"creating": {"creating", model.BytePlusAssetStatusProcessing, true},
		"pending":  {"pending", model.BytePlusAssetStatusProcessing, true},
		"active":   {"active", model.BytePlusAssetStatusActive, true},
		"failed":   {"failed", model.BytePlusAssetStatusFailed, true},
		"blocked":  {"blocked", model.BytePlusAssetStatusFailed, true},
		"offline":  {"offline", model.BytePlusAssetStatusFailed, true},
		"unknown":  {"weird", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := virtualCharacterRealPersonAssetStatus(tc.status)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
