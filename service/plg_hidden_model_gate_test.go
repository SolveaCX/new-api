package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHiddenModelBlockedForIdentityBlocksPLGOnHiddenModel(t *testing.T) {
	withServiceHiddenPricingModels(t, "hidden-exact, secret-*")

	require.True(t, HiddenModelBlockedForIdentity("plg", "hidden-exact"))
	require.True(t, HiddenModelBlockedForIdentity("plg", "secret-video"))
	require.True(t, HiddenModelBlockedForIdentity("plg", "HIDDEN-EXACT"))
}

func TestHiddenModelBlockedForIdentityAllowsPLGOnVisibleModel(t *testing.T) {
	withServiceHiddenPricingModels(t, "hidden-exact, secret-*")

	require.False(t, HiddenModelBlockedForIdentity("plg", "visible-model"))
	require.False(t, HiddenModelBlockedForIdentity("plg", "hidden-exact-2"))
	require.False(t, HiddenModelBlockedForIdentity("plg", ""))
}

func TestHiddenModelBlockedForIdentityTreatsEmptyGroupAsPLG(t *testing.T) {
	withServiceHiddenPricingModels(t, "hidden-exact")

	require.True(t, HiddenModelBlockedForIdentity("", "hidden-exact"))
	require.True(t, HiddenModelBlockedForIdentity("  plg  ", "hidden-exact"))
}

func TestHiddenModelBlockedForIdentityNeverBlocksEnterprise(t *testing.T) {
	withServiceHiddenPricingModels(t, "hidden-exact, secret-*, *")

	require.False(t, HiddenModelBlockedForIdentity("Enterprise", "hidden-exact"))
	require.False(t, HiddenModelBlockedForIdentity("default", "secret-video"))
	require.False(t, HiddenModelBlockedForIdentity("vip", "anything"))
}

func TestHiddenModelBlockedForIdentityEmptyListBlocksNothing(t *testing.T) {
	withServiceHiddenPricingModels(t, "")

	require.False(t, HiddenModelBlockedForIdentity("plg", "hidden-exact"))
	require.False(t, HiddenModelBlockedForIdentity("", "hidden-exact"))
}
