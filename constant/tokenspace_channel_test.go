package constant

import "testing"

func TestTokenSpaceChannelRegistration(t *testing.T) {
	if ChannelTypeTokenSpace != 114 {
		t.Fatalf("ChannelTypeTokenSpace = %d, want 114", ChannelTypeTokenSpace)
	}
	if ChannelTypeDummy != 115 {
		t.Fatalf("ChannelTypeDummy = %d, want 115", ChannelTypeDummy)
	}
	if got := ChannelTypeNames[ChannelTypeTokenSpace]; got != "TokenSpace" {
		t.Fatalf("TokenSpace channel name = %q, want TokenSpace", got)
	}
	if len(ChannelBaseURLs) <= ChannelTypeTokenSpace {
		t.Fatalf("ChannelBaseURLs missing index %d", ChannelTypeTokenSpace)
	}
	if got := ChannelBaseURLs[ChannelTypeTokenSpace]; got != "https://api.tokenspace.net.cn" {
		t.Fatalf("TokenSpace base URL = %q, want https://api.tokenspace.net.cn", got)
	}
}

func TestTokenSpaceSplitPreservesLegacyChannelTypeValues(t *testing.T) {
	if ChannelTypeDoubaoVideo != 54 {
		t.Fatalf("ChannelTypeDoubaoVideo changed to %d", ChannelTypeDoubaoVideo)
	}
	if ChannelTypeGrokSubscription != 113 {
		t.Fatalf("ChannelTypeGrokSubscription changed to %d", ChannelTypeGrokSubscription)
	}
}
