package merge

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

// DecodePNG loads a PNG file.
func DecodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}
