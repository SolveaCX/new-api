package elevenlabs

// Billable ElevenLabs models exposed through the gateway. These names double as the
// abilities registered on the ElevenLabs channel so Distribute can select it, and as
// the ModelRatio keys that price each capability (see the package doc).
const (
	ModelMultilingualV2 = "eleven_multilingual_v2" // Text-to-Speech (billed per input character)
	ModelSoundV1        = "eleven_sound_v1"        // Sound effects (billed per requested second)
	ModelMusicV1        = "eleven_music_v1"        // Music generation (billed per requested second)
)

var ModelList = []string{
	ModelMultilingualV2,
	ModelSoundV1,
	ModelMusicV1,
}
