package commands

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
)

// encodeBridgeScreenshot decodes a PNG screenshot, downscales it so the longest
// side is at most maxSide, and re-encodes it as JPEG. It returns the JPEG
// bytes and the final width and height. No dependency outside the standard
// library is used: downscaling is a box filter over the RGBA pixels.
func encodeBridgeScreenshot(pngBytes []byte, maxSide, quality int) ([]byte, int, int, error) {
	if len(pngBytes) == 0 {
		return nil, 0, 0, errors.New("screenshot is empty")
	}
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, 0, 0, err
	}
	scaled := downscaleImage(src, maxSide)
	var out bytes.Buffer
	if quality <= 0 || quality > 100 {
		quality = jpeg.DefaultQuality
	}
	if err := jpeg.Encode(&out, scaled, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	bounds := scaled.Bounds()
	return out.Bytes(), bounds.Dx(), bounds.Dy(), nil
}

// downscaleImage returns src as an RGBA image whose longest side is at most
// maxSide. Images that already fit are converted without resampling.
func downscaleImage(src image.Image, maxSide int) *image.RGBA {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), src, bounds.Min, draw.Src)
	longest := width
	if height > longest {
		longest = height
	}
	if maxSide <= 0 || longest <= maxSide || width == 0 || height == 0 {
		return rgba
	}
	scale := float64(maxSide) / float64(longest)
	dstWidth := int(float64(width)*scale + 0.5)
	dstHeight := int(float64(height)*scale + 0.5)
	if dstWidth < 1 {
		dstWidth = 1
	}
	if dstHeight < 1 {
		dstHeight = 1
	}
	if width >= height {
		dstWidth = maxSide
	} else {
		dstHeight = maxSide
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	for y := 0; y < dstHeight; y++ {
		y0 := y * height / dstHeight
		y1 := (y + 1) * height / dstHeight
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < dstWidth; x++ {
			x0 := x * width / dstWidth
			x1 := (x + 1) * width / dstWidth
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, count uint64
			for sy := y0; sy < y1; sy++ {
				row := rgba.PixOffset(x0, sy)
				for sx := x0; sx < x1; sx++ {
					r += uint64(rgba.Pix[row])
					g += uint64(rgba.Pix[row+1])
					b += uint64(rgba.Pix[row+2])
					row += 4
					count++
				}
			}
			offset := dst.PixOffset(x, y)
			dst.Pix[offset] = uint8(r / count)
			dst.Pix[offset+1] = uint8(g / count)
			dst.Pix[offset+2] = uint8(b / count)
			dst.Pix[offset+3] = 0xff
		}
	}
	return dst
}
