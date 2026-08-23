package assets

import (
	"bytes"
	"image"
	_ "image/png"
	"testing"
)

// The icon used to be loaded from "lunefetch.ico" in the working directory,
// which the installer never shipped and which Go cannot decode. Assert the
// embedded replacement is present and actually decodable, so a blank window or
// tray icon fails the build instead of shipping.
func TestAppIconIsDecodablePNG(t *testing.T) {
	if AppIcon == nil {
		t.Fatal("AppIcon is nil")
	}
	if AppIcon.Name() == "" {
		t.Fatal("AppIcon has no name")
	}
	content := AppIcon.Content()
	if len(content) == 0 {
		t.Fatal("AppIcon has no bytes; the embed did not resolve")
	}

	img, format, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode icon: %v", err)
	}
	if format != "png" {
		t.Fatalf("icon format = %q, want png (Fyne cannot decode ICO)", format)
	}
	bounds := img.Bounds()
	if bounds.Dx() < 64 || bounds.Dy() < 64 {
		t.Fatalf("icon is %dx%d, want at least 64x64 so the tray render stays sharp", bounds.Dx(), bounds.Dy())
	}
	if bounds.Dx() != bounds.Dy() {
		t.Fatalf("icon is %dx%d, want a square image", bounds.Dx(), bounds.Dy())
	}
}
