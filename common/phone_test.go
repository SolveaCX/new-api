package common

import "testing"

func TestNormalizePhoneNumberRequiresCountryCodeAndReturnsE164(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "spaces and punctuation", input: "+86 138-0013-8000", want: "+8613800138000"},
		{name: "missing country code", input: "13800138000", wantErr: true},
		{name: "too short", input: "+12345", wantErr: true},
		{name: "too long", input: "+1234567890123456", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePhoneNumber(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizePhoneNumber(%q) expected an error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePhoneNumber(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizePhoneNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
