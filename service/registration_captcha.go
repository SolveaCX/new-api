package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
	xdraw "golang.org/x/image/draw"
)

//go:embed assets/captcha/alpine-lake.png
var registrationCaptchaLandscapeData []byte

var registrationCaptchaLandscape = sync.OnceValues(func() (image.Image, error) {
	return png.Decode(bytes.NewReader(registrationCaptchaLandscapeData))
})

const (
	RegistrationCaptchaTypeSlide  = "slide"
	RegistrationCaptchaTypeRotate = "rotate"

	registrationCaptchaChallengePrefix = "registration-captcha:challenge:"
	registrationCaptchaTokenPrefix     = "registration-captcha:token:"
	registrationCaptchaChallengeTTL    = 5 * time.Minute
	registrationCaptchaTokenTTL        = 5 * time.Minute
	registrationCaptchaRedisTimeout    = 2 * time.Second
	registrationCaptchaMemoryMaxSize   = 4096
)

var (
	ErrRegistrationCaptchaInvalid     = errors.New("registration captcha is invalid")
	ErrRegistrationCaptchaUnavailable = errors.New("registration captcha is unavailable")

	registrationCaptchaMemoryMu sync.Mutex
	registrationCaptchaMemory   = make(map[string]registrationCaptchaMemoryValue)
	registrationCaptchaTake     = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if value then
  redis.call("DEL", KEYS[1])
end
return value
`)
)

type registrationCaptchaMemoryValue struct {
	value     string
	expiresAt time.Time
}

type registrationCaptchaStoredChallenge struct {
	Type        string `json:"type"`
	TargetX     int    `json:"target_x,omitempty"`
	TargetY     int    `json:"target_y,omitempty"`
	TargetAngle int    `json:"target_angle,omitempty"`
}

type RegistrationCaptchaChallenge struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	BackgroundImage string `json:"background_image"`
	PieceImage      string `json:"piece_image,omitempty"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	PieceWidth      int    `json:"piece_width,omitempty"`
	PieceHeight     int    `json:"piece_height,omitempty"`
	StartX          int    `json:"start_x,omitempty"`
	StartY          int    `json:"start_y,omitempty"`
}

type RegistrationCaptchaAnswer struct {
	ID    string `json:"id"`
	X     *int   `json:"x,omitempty"`
	Y     *int   `json:"y,omitempty"`
	Angle *int   `json:"angle,omitempty"`
}

func GenerateRegistrationCaptcha(captchaType string) (*RegistrationCaptchaChallenge, error) {
	switch captchaType {
	case RegistrationCaptchaTypeSlide:
		return generateRegistrationSlideCaptcha()
	case RegistrationCaptchaTypeRotate:
		return generateRegistrationRotateCaptcha()
	default:
		return nil, ErrRegistrationCaptchaInvalid
	}
}

func VerifyRegistrationCaptcha(answer RegistrationCaptchaAnswer, clientIP string) (string, error) {
	if answer.ID == "" {
		return "", ErrRegistrationCaptchaInvalid
	}

	rawChallenge, err := takeRegistrationCaptchaValue(registrationCaptchaChallengePrefix + answer.ID)
	if err != nil {
		return "", err
	}
	if rawChallenge == "" {
		return "", ErrRegistrationCaptchaInvalid
	}

	var challenge registrationCaptchaStoredChallenge
	if err := common.Unmarshal([]byte(rawChallenge), &challenge); err != nil {
		return "", ErrRegistrationCaptchaUnavailable
	}

	valid := false
	switch challenge.Type {
	case RegistrationCaptchaTypeSlide:
		valid = answer.X != nil && answer.Y != nil && slide.Validate(*answer.X, *answer.Y, challenge.TargetX, challenge.TargetY, 6)
	case RegistrationCaptchaTypeRotate:
		valid = answer.Angle != nil && rotate.Validate(normalizeCaptchaAngle(*answer.Angle), challenge.TargetAngle, 8)
	}
	if !valid {
		return "", ErrRegistrationCaptchaInvalid
	}

	token, err := randomRegistrationCaptchaCredential(32)
	if err != nil {
		return "", ErrRegistrationCaptchaUnavailable
	}
	if err := storeRegistrationCaptchaValue(registrationCaptchaTokenPrefix+token, registrationCaptchaIPDigest(clientIP), registrationCaptchaTokenTTL); err != nil {
		return "", err
	}
	return token, nil
}

func ConsumeRegistrationCaptchaToken(token string, clientIP string) (bool, error) {
	if token == "" {
		return false, nil
	}
	value, err := takeRegistrationCaptchaValue(registrationCaptchaTokenPrefix + token)
	if err != nil {
		return false, err
	}
	return value != "" && value == registrationCaptchaIPDigest(clientIP), nil
}

func generateRegistrationSlideCaptcha() (*RegistrationCaptchaChallenge, error) {
	const (
		width     = 300
		height    = 180
		pieceSize = 52
	)

	builder := slide.NewBuilder(
		slide.WithImageSize(option.Size{Width: width, Height: height}),
		slide.WithRangeGraphSize(option.RangeVal{Min: pieceSize, Max: pieceSize}),
		slide.WithRangeGraphAnglePos([]option.RangeVal{{Min: 0, Max: 0}}),
	)
	builder.SetResources(
		slide.WithBackgrounds([]image.Image{newRegistrationCaptchaBackground(width, height)}),
		slide.WithGraphImages([]*slide.GraphImage{newRegistrationCaptchaSlideGraph(pieceSize)}),
	)
	captchaData, err := builder.Make().Generate()
	if err != nil {
		return nil, fmt.Errorf("generate registration slide captcha: %w", err)
	}
	block := captchaData.GetData()
	background, err := captchaData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("encode registration slide background: %w", err)
	}
	piece, err := captchaData.GetTileImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("encode registration slide piece: %w", err)
	}

	challengeID, err := storeRegistrationCaptchaChallenge(registrationCaptchaStoredChallenge{
		Type:    RegistrationCaptchaTypeSlide,
		TargetX: block.X,
		TargetY: block.Y,
	})
	if err != nil {
		return nil, err
	}
	return &RegistrationCaptchaChallenge{
		ID:              challengeID,
		Type:            RegistrationCaptchaTypeSlide,
		BackgroundImage: background,
		PieceImage:      piece,
		Width:           width,
		Height:          height,
		PieceWidth:      block.Width,
		PieceHeight:     block.Height,
		StartX:          block.DX,
		StartY:          block.DY,
	}, nil
}

func generateRegistrationRotateCaptcha() (*RegistrationCaptchaChallenge, error) {
	const size = 220

	builder := rotate.NewBuilder(
		rotate.WithImageSquareSize(size),
		rotate.WithRangeAnglePos([]option.RangeVal{{Min: 35, Max: 325}}),
	)
	builder.SetResources(rotate.WithImages([]image.Image{newRegistrationCaptchaBackground(size, size)}))
	captchaData, err := builder.Make().Generate()
	if err != nil {
		return nil, fmt.Errorf("generate registration rotate captcha: %w", err)
	}
	block := captchaData.GetData()
	background, err := captchaData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("encode registration rotate background: %w", err)
	}

	challengeID, err := storeRegistrationCaptchaChallenge(registrationCaptchaStoredChallenge{
		Type:        RegistrationCaptchaTypeRotate,
		TargetAngle: block.Angle,
	})
	if err != nil {
		return nil, err
	}
	return &RegistrationCaptchaChallenge{
		ID:              challengeID,
		Type:            RegistrationCaptchaTypeRotate,
		BackgroundImage: background,
		Width:           size,
		Height:          size,
	}, nil
}

func storeRegistrationCaptchaChallenge(challenge registrationCaptchaStoredChallenge) (string, error) {
	challengeID, err := randomRegistrationCaptchaCredential(24)
	if err != nil {
		return "", ErrRegistrationCaptchaUnavailable
	}
	data, err := common.Marshal(challenge)
	if err != nil {
		return "", ErrRegistrationCaptchaUnavailable
	}
	if err := storeRegistrationCaptchaValue(registrationCaptchaChallengePrefix+challengeID, string(data), registrationCaptchaChallengeTTL); err != nil {
		return "", err
	}
	return challengeID, nil
}

func storeRegistrationCaptchaValue(key, value string, ttl time.Duration) error {
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), registrationCaptchaRedisTimeout)
		defer cancel()
		if err := common.RDB.Set(ctx, key, value, ttl).Err(); err != nil {
			common.SysError("failed to store registration captcha in Redis: " + err.Error())
			return ErrRegistrationCaptchaUnavailable
		}
		return nil
	}

	registrationCaptchaMemoryMu.Lock()
	defer registrationCaptchaMemoryMu.Unlock()
	pruneRegistrationCaptchaMemoryLocked(time.Now())
	if len(registrationCaptchaMemory) >= registrationCaptchaMemoryMaxSize {
		return ErrRegistrationCaptchaUnavailable
	}
	registrationCaptchaMemory[key] = registrationCaptchaMemoryValue{value: value, expiresAt: time.Now().Add(ttl)}
	return nil
}

func takeRegistrationCaptchaValue(key string) (string, error) {
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), registrationCaptchaRedisTimeout)
		defer cancel()
		value, err := registrationCaptchaTake.Run(ctx, common.RDB, []string{key}).Text()
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		if err != nil {
			common.SysError("failed to consume registration captcha from Redis: " + err.Error())
			return "", ErrRegistrationCaptchaUnavailable
		}
		return value, nil
	}

	registrationCaptchaMemoryMu.Lock()
	defer registrationCaptchaMemoryMu.Unlock()
	now := time.Now()
	entry, ok := registrationCaptchaMemory[key]
	delete(registrationCaptchaMemory, key)
	if !ok || !entry.expiresAt.After(now) {
		return "", nil
	}
	return entry.value, nil
}

func pruneRegistrationCaptchaMemoryLocked(now time.Time) {
	for key, entry := range registrationCaptchaMemory {
		if !entry.expiresAt.After(now) {
			delete(registrationCaptchaMemory, key)
		}
	}
}

func registrationCaptchaIPDigest(clientIP string) string {
	digest := sha256.Sum256([]byte(clientIP))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func randomRegistrationCaptchaCredential(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func normalizeCaptchaAngle(value int) int {
	value %= 360
	if value < 0 {
		value += 360
	}
	return value
}

func newRegistrationCaptchaBackground(width, height int) image.Image {
	if background, err := registrationCaptchaLandscape(); err == nil {
		// Center-crop without distortion for both the slide and rotation layouts.
		crop := background.Bounds()
		if crop.Dx()*height > crop.Dy()*width {
			cropWidth := crop.Dy() * width / height
			crop.Min.X += (crop.Dx() - cropWidth) / 2
			crop.Max.X = crop.Min.X + cropWidth
		} else {
			cropHeight := crop.Dx() * height / width
			crop.Min.Y += (crop.Dy() - cropHeight) / 2
			crop.Max.Y = crop.Min.Y + cropHeight
		}
		canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
		xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), background, crop, draw.Src, nil)
		return canvas
	}
	// Keep a local fallback so an asset decoding failure cannot break signup.
	palettes := [][4]color.NRGBA{
		{{R: 231, G: 240, B: 255, A: 255}, {R: 129, G: 160, B: 214, A: 255}, {R: 65, G: 87, B: 140, A: 255}, {R: 248, G: 198, B: 105, A: 255}},
		{{R: 237, G: 247, B: 239, A: 255}, {R: 132, G: 184, B: 159, A: 255}, {R: 48, G: 103, B: 84, A: 255}, {R: 242, G: 170, B: 120, A: 255}},
		{{R: 248, G: 237, B: 244, A: 255}, {R: 190, G: 145, B: 181, A: 255}, {R: 103, G: 69, B: 114, A: 255}, {R: 246, G: 195, B: 93, A: 255}},
	}
	selector := make([]byte, 1)
	_, _ = rand.Read(selector)
	palette := palettes[int(selector[0])%len(palettes)]
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		ratio := float64(y) / float64(height)
		for x := 0; x < width; x++ {
			canvas.SetNRGBA(x, y, blendCaptchaColor(palette[0], palette[1], ratio))
		}
	}

	fillCaptchaCircle(canvas, width*3/4, height/4, maxCaptchaInt(12, height/13), palette[3])
	drawCaptchaTriangle(canvas, image.Pt(0, height), image.Pt(width/2, height/3), image.Pt(width, height), palette[2])
	drawCaptchaTriangle(canvas, image.Pt(width/3, height), image.Pt(width*2/3, height/2), image.Pt(width, height), palette[1])

	houseWidth := maxCaptchaInt(28, width/7)
	houseHeight := maxCaptchaInt(24, height/5)
	houseX := width / 6
	houseY := height - houseHeight - maxCaptchaInt(8, height/14)
	draw.Draw(canvas, image.Rect(houseX, houseY, houseX+houseWidth, houseY+houseHeight), &image.Uniform{C: color.NRGBA{R: 247, G: 244, B: 235, A: 255}}, image.Point{}, draw.Src)
	drawCaptchaTriangle(canvas, image.Pt(houseX-5, houseY), image.Pt(houseX+houseWidth/2, houseY-houseHeight/2), image.Pt(houseX+houseWidth+5, houseY), palette[3])
	doorWidth := maxCaptchaInt(6, houseWidth/5)
	draw.Draw(canvas, image.Rect(houseX+houseWidth/2-doorWidth/2, houseY+houseHeight/2, houseX+houseWidth/2+doorWidth/2, houseY+houseHeight), &image.Uniform{C: palette[2]}, image.Point{}, draw.Src)
	return canvas
}

func newRegistrationCaptchaSlideGraph(size int) *slide.GraphImage {
	overlay := image.NewNRGBA(image.Rect(0, 0, size, size))
	shadow := image.NewNRGBA(image.Rect(0, 0, size, size))
	mask := image.NewAlpha(image.Rect(0, 0, size, size))
	inside := func(x, y int) bool {
		margin := size / 6
		center := size / 2
		radius := size / 7
		body := x >= margin && x < size-margin && y >= margin && y < size-margin
		topKnob := captchaPointInCircle(x, y, center, margin, radius)
		rightKnob := captchaPointInCircle(x, y, size-margin, center, radius)
		leftNotch := captchaPointInCircle(x, y, margin, center, radius-1)
		return (body || topKnob || rightKnob) && !leftNotch
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !inside(x, y) {
				continue
			}
			mask.SetAlpha(x, y, color.Alpha{A: 255})
			shadow.SetNRGBA(x, y, color.NRGBA{R: 24, G: 32, B: 55, A: 115})
			border := !inside(x-1, y) || !inside(x+1, y) || !inside(x, y-1) || !inside(x, y+1)
			if border {
				overlay.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 235})
			}
		}
	}
	return &slide.GraphImage{OverlayImage: overlay, ShadowImage: shadow, MaskImage: mask}
}

func blendCaptchaColor(start, end color.NRGBA, ratio float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(start.R)*(1-ratio) + float64(end.R)*ratio),
		G: uint8(float64(start.G)*(1-ratio) + float64(end.G)*ratio),
		B: uint8(float64(start.B)*(1-ratio) + float64(end.B)*ratio),
		A: 255,
	}
}

func fillCaptchaCircle(canvas *image.NRGBA, centerX, centerY, radius int, fill color.NRGBA) {
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if captchaPointInCircle(x, y, centerX, centerY, radius) && image.Pt(x, y).In(canvas.Bounds()) {
				canvas.SetNRGBA(x, y, fill)
			}
		}
	}
}

func captchaPointInCircle(x, y, centerX, centerY, radius int) bool {
	dx := x - centerX
	dy := y - centerY
	return dx*dx+dy*dy <= radius*radius
}

func drawCaptchaTriangle(canvas *image.NRGBA, a, b, c image.Point, fill color.NRGBA) {
	minX := minCaptchaInt(a.X, minCaptchaInt(b.X, c.X))
	maxX := maxCaptchaInt(a.X, maxCaptchaInt(b.X, c.X))
	minY := minCaptchaInt(a.Y, minCaptchaInt(b.Y, c.Y))
	maxY := maxCaptchaInt(a.Y, maxCaptchaInt(b.Y, c.Y))
	area := captchaTriangleEdge(a, b, c)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			point := image.Pt(x, y)
			w1 := captchaTriangleEdge(point, b, c)
			w2 := captchaTriangleEdge(a, point, c)
			w3 := captchaTriangleEdge(a, b, point)
			if image.Pt(x, y).In(canvas.Bounds()) && ((area >= 0 && w1 >= 0 && w2 >= 0 && w3 >= 0) || (area < 0 && w1 <= 0 && w2 <= 0 && w3 <= 0)) {
				canvas.SetNRGBA(x, y, fill)
			}
		}
	}
}

func captchaTriangleEdge(a, b, c image.Point) int {
	return (c.X-a.X)*(b.Y-a.Y) - (c.Y-a.Y)*(b.X-a.X)
}

func minCaptchaInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxCaptchaInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
