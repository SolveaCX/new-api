package service

import (
	"image"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestRegistrationCaptchaLandscapeBackground(t *testing.T) {
	background, err := registrationCaptchaLandscape()
	if err != nil {
		t.Fatalf("decode embedded captcha landscape: %v", err)
	}
	if background.Bounds().Dx() < 300 || background.Bounds().Dy() < 220 {
		t.Fatalf("captcha landscape is too small: %v", background.Bounds())
	}
	for _, size := range []image.Point{image.Pt(300, 180), image.Pt(220, 220)} {
		resized := newRegistrationCaptchaBackground(size.X, size.Y)
		if resized.Bounds() != image.Rect(0, 0, size.X, size.Y) {
			t.Fatalf("unexpected captcha background dimensions: %v", resized.Bounds())
		}
	}
	// Each request must receive its own canvas, not mutate the cached source.
	first := newRegistrationCaptchaBackground(300, 180).(*image.NRGBA)
	second := newRegistrationCaptchaBackground(300, 180).(*image.NRGBA)
	original := second.NRGBAAt(0, 0)
	first.Pix[0] ^= 0xff
	if second.NRGBAAt(0, 0) != original {
		t.Fatal("captcha backgrounds must not share writable pixels")
	}
}

func useRegistrationCaptchaMemoryStore(t *testing.T) {
	t.Helper()
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	registrationCaptchaMemoryMu.Lock()
	registrationCaptchaMemory = make(map[string]registrationCaptchaMemoryValue)
	registrationCaptchaMemoryMu.Unlock()
	t.Cleanup(func() {
		registrationCaptchaMemoryMu.Lock()
		registrationCaptchaMemory = make(map[string]registrationCaptchaMemoryValue)
		registrationCaptchaMemoryMu.Unlock()
		common.RedisEnabled = previousRedisEnabled
	})
}

func storedRegistrationCaptchaChallenge(t *testing.T, challengeID string) registrationCaptchaStoredChallenge {
	t.Helper()
	registrationCaptchaMemoryMu.Lock()
	entry, ok := registrationCaptchaMemory[registrationCaptchaChallengePrefix+challengeID]
	registrationCaptchaMemoryMu.Unlock()
	if !ok {
		t.Fatal("registration captcha challenge was not stored")
	}
	var challenge registrationCaptchaStoredChallenge
	if err := common.Unmarshal([]byte(entry.value), &challenge); err != nil {
		t.Fatalf("decode stored registration captcha challenge: %v", err)
	}
	return challenge
}

func TestRegistrationSlideCaptchaOneTimeFlow(t *testing.T) {
	useRegistrationCaptchaMemoryStore(t)

	challenge, err := GenerateRegistrationCaptcha(RegistrationCaptchaTypeSlide)
	if err != nil {
		t.Fatalf("generate slide captcha: %v", err)
	}
	if challenge.ID == "" || challenge.Type != RegistrationCaptchaTypeSlide {
		t.Fatalf("unexpected challenge metadata: %#v", challenge)
	}
	if !strings.HasPrefix(challenge.BackgroundImage, "data:image/") || !strings.HasPrefix(challenge.PieceImage, "data:image/") {
		t.Fatal("slide captcha images must be embedded data URLs")
	}
	if challenge.Width <= 0 || challenge.Height <= 0 || challenge.PieceWidth <= 0 || challenge.PieceHeight <= 0 {
		t.Fatalf("unexpected challenge dimensions: %#v", challenge)
	}

	stored := storedRegistrationCaptchaChallenge(t, challenge.ID)
	token, err := VerifyRegistrationCaptcha(RegistrationCaptchaAnswer{
		ID: challenge.ID,
		X:  &stored.TargetX,
		Y:  &stored.TargetY,
	}, "192.0.2.10")
	if err != nil || token == "" {
		t.Fatalf("verify slide captcha: token=%q err=%v", token, err)
	}
	valid, err := ConsumeRegistrationCaptchaToken(token, "192.0.2.10")
	if err != nil || !valid {
		t.Fatalf("consume slide captcha token: valid=%v err=%v", valid, err)
	}
	valid, err = ConsumeRegistrationCaptchaToken(token, "192.0.2.10")
	if err != nil || valid {
		t.Fatalf("slide captcha token must be one-time: valid=%v err=%v", valid, err)
	}
}

func TestRegistrationRotateCaptchaOneTimeFlow(t *testing.T) {
	useRegistrationCaptchaMemoryStore(t)

	challenge, err := GenerateRegistrationCaptcha(RegistrationCaptchaTypeRotate)
	if err != nil {
		t.Fatalf("generate rotate captcha: %v", err)
	}
	if challenge.ID == "" || challenge.Type != RegistrationCaptchaTypeRotate {
		t.Fatalf("unexpected challenge metadata: %#v", challenge)
	}
	if !strings.HasPrefix(challenge.BackgroundImage, "data:image/") || challenge.PieceImage != "" {
		t.Fatal("rotate captcha must contain one embedded background image")
	}

	stored := storedRegistrationCaptchaChallenge(t, challenge.ID)
	answer := normalizeCaptchaAngle(360 - stored.TargetAngle)
	token, err := VerifyRegistrationCaptcha(RegistrationCaptchaAnswer{
		ID:    challenge.ID,
		Angle: &answer,
	}, "192.0.2.11")
	if err != nil || token == "" {
		t.Fatalf("verify rotate captcha: token=%q err=%v", token, err)
	}
	valid, err := ConsumeRegistrationCaptchaToken(token, "192.0.2.99")
	if err != nil || valid {
		t.Fatalf("captcha token must be bound to the verifier IP: valid=%v err=%v", valid, err)
	}
	valid, err = ConsumeRegistrationCaptchaToken(token, "192.0.2.11")
	if err != nil || valid {
		t.Fatalf("IP-mismatched consumption must still consume the token: valid=%v err=%v", valid, err)
	}
}

func TestRegistrationCaptchaFailedAnswerConsumesChallenge(t *testing.T) {
	useRegistrationCaptchaMemoryStore(t)

	challenge, err := GenerateRegistrationCaptcha(RegistrationCaptchaTypeSlide)
	if err != nil {
		t.Fatalf("generate slide captcha: %v", err)
	}
	stored := storedRegistrationCaptchaChallenge(t, challenge.ID)
	wrongX := stored.TargetX + 20
	if _, err := VerifyRegistrationCaptcha(RegistrationCaptchaAnswer{
		ID: challenge.ID,
		X:  &wrongX,
		Y:  &stored.TargetY,
	}, "192.0.2.12"); err != ErrRegistrationCaptchaInvalid {
		t.Fatalf("expected invalid answer error, got %v", err)
	}
	if _, err := VerifyRegistrationCaptcha(RegistrationCaptchaAnswer{
		ID: challenge.ID,
		X:  &stored.TargetX,
		Y:  &stored.TargetY,
	}, "192.0.2.12"); err != ErrRegistrationCaptchaInvalid {
		t.Fatalf("failed challenge must not be reusable, got %v", err)
	}
}

func TestGenerateRegistrationCaptchaRejectsUnknownType(t *testing.T) {
	useRegistrationCaptchaMemoryStore(t)
	if _, err := GenerateRegistrationCaptcha("unknown"); err != ErrRegistrationCaptchaInvalid {
		t.Fatalf("expected invalid captcha type error, got %v", err)
	}
}
