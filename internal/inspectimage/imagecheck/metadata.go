package imagecheck

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

type Metadata struct {
	Width    int  `json:"width,omitempty"`
	Height   int  `json:"height,omitempty"`
	Animated bool `json:"animated"`
}

func ReadMetadata(data []byte, mime string) (Metadata, string) {
	var meta Metadata
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return meta, "Image dimensions could not be read."
	}
	meta.Width = cfg.Width
	meta.Height = cfg.Height
	if mime == "image/gif" {
		meta.Animated = DetectAnimatedGIF(data)
	}
	return meta, ""
}

func DetectAnimatedGIF(data []byte) bool {
	count := 0
	for i := 0; i < len(data); i++ {
		if data[i] == 0x2c {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}
