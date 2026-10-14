package report

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

func parseImage(data []byte) (*Image, error) {
	if len(data) == 0 {
		return nil, errors.New("image file is empty")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image must be a valid PNG or JPEG: %w", err)
	}
	var mime string
	switch format {
	case "png":
		mime = "image/png"
	case "jpeg":
		mime = "image/jpeg"
	default:
		return nil, errors.New("image must be PNG or JPEG")
	}
	// Check before decoding pixels, and divide to avoid overflow for large dimensions.
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxImagePixels/config.Height {
		return nil, fmt.Errorf("image dimensions must be positive and at most %d pixels", maxImagePixels)
	}
	if format == "png" {
		if err := checkPNGChunks(data); err != nil {
			return nil, err
		}
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("invalid %s image: %w", format, err)
	}
	return &Image{Data: data, MIME: mime, Width: config.Width, Height: config.Height}, nil
}

func checkPNGChunks(data []byte) error {
	// Go decodes only the first APNG frame; browsers would animate the original bytes.
	for offset := 8; ; {
		if len(data)-offset < 12 {
			return errors.New("invalid png image: truncated chunk")
		}
		size := binary.BigEndian.Uint32(data[offset : offset+4])
		if uint64(size) > uint64(len(data)-offset-12) {
			return errors.New("invalid png image: truncated chunk data")
		}
		switch string(data[offset+4 : offset+8]) {
		case "acTL":
			return errors.New("animated PNG images are unsupported")
		case "IEND":
			return nil
		}
		offset += int(size) + 12
	}
}
