package unicornsignage

import (
	"image"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

func ImageFromText(text string, fontBytes []byte, x int) (outimg image.Image, err error) {
	newImage := image.NewRGBA(image.Rect(0, 0, 16, 16))
	labelImage, err := addLabel(newImage, -x, 12, text, 15, fontBytes)
	if err != nil {
		return nil, err
	}

	// rotate the image by 90 degrees
	dstImage := imaging.Rotate90(labelImage)
	return dstImage, nil
}

func loadFontFaceReader(fontBytes []byte, points float64) (font.Face, error) {
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	face := truetype.NewFace(f, &truetype.Options{
		Size: points,
		// Hinting: font.HintingFull,
	})
	return face, nil
}

func addLabel(img image.Image, x, y int, label string, size int, fontBytes []byte) (outimage image.Image, err error) {
	var w = img.Bounds().Dx()
	var h = img.Bounds().Dy()
	dc := gg.NewContext(w, h)
	// Text color - white
	dc.SetRGB(1, 1, 1)

	face, err := loadFontFaceReader(fontBytes, float64(size))
	if err != nil {
		return nil, err
	}
	dc.SetFontFace(face)

	// Draw the background
	dc.DrawImage(img, 0, 0)
	// Draw text at position - anchor on the top left corner of the text
	dc.DrawStringAnchored(label, float64(x), float64(y), 0, 0)
	dc.Clip()

	outimage = dc.Image()
	return outimage, nil
}