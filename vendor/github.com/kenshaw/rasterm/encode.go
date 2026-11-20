package rasterm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-sixel"
)

// DefaultJPEGQuality is the default JPEG encode quality.
var DefaultJPEGQuality = 93

// Encoder provides a common interface for terminal graphic encoders.
type Encoder interface {
	Available() bool
	Encode(io.Writer, image.Image) error
}

// KittyEncoder is a Kitty terminal graphics encoder.
type KittyEncoder struct {
	NoNewline bool
}

// NewKittyEncoder creates a Kitty terminal graphics encoder.
//
// See: https://sw.kovidgoyal.net/kitty/graphics-protocol.html
func NewKittyEncoder() Encoder {
	return KittyEncoder{}
}

// Available satisfies the [Encoder] interface.
func (KittyEncoder) Available() bool {
	return !hasTermGraphics("none") &&
		(hasTermGraphics("kitty") ||
			(os.Getenv("KITTY_PID") != "" && os.Getenv("KITTY_WINDOW_ID") != "") ||
			strings.ToLower(os.Getenv("TERM")) == "xterm-kitty" ||
			strings.ToLower(os.Getenv("TERM_PROGRAM")) == "ghostty")
}

// Encode satisfies the [Encoder] interface.
func (r KittyEncoder) Encode(w io.Writer, img image.Image) error {
	buf := new(bytes.Buffer)
	enc := base64.NewEncoder(base64.StdEncoding, buf)
	if err := png.Encode(enc, img); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if err := chunkEncode(w, buf.Bytes(), 4096); err != nil {
		return err
	}
	if r.NoNewline {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return err
}

// ITermEncoder is a iTerm terminal graphics encoder.
//
// See: https://iterm2.com/documentation-images.html
type ITermEncoder struct {
	NoNewline bool
}

// NewITermEncoder creates a iTerm terminal graphics encoder.
func NewITermEncoder() Encoder {
	return ITermEncoder{}
}

// Available satisfies the [Encoder] interface.
func (ITermEncoder) Available() bool {
	return !hasTermGraphics("none") &&
		(hasTermGraphics("iterm") ||
			strings.ToLower(os.Getenv("TERM")) == "mintty" ||
			strings.ToLower(os.Getenv("LC_TERMINAL")) == "iterm2" ||
			strings.ToLower(os.Getenv("TERM_PROGRAM")) == "wezterm")
}

// Encode satisfies the [Encoder] interface.
func (r ITermEncoder) Encode(w io.Writer, img image.Image) error {
	f := png.Encode
	if _, ok := img.(*image.Paletted); !ok {
		f = jpegEncode
	}
	buf := new(bytes.Buffer)
	enc := base64.NewEncoder(base64.StdEncoding, buf)
	if err := f(enc, img); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\x1b]1337;File=inline=1:%s\a", buf.Bytes()); err != nil {
		return err
	}
	if r.NoNewline {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return err
}

// SixelEncoder is a Sixel terminal graphics encoder.
//
// See: https://saitoha.github.io/libsixel/
type SixelEncoder struct {
	NoNewline bool
}

// NewSixelEncoder creates a Sixel terminal graphics encoder.
func NewSixelEncoder() Encoder {
	return SixelEncoder{}
}

// Available satisfies the [Encoder] interface.
func (SixelEncoder) Available() bool {
	return !hasTermGraphics("none") && (hasTermGraphics("sixel") || hasSixelSupport())
}

// Encode satisfies the [Encoder] interface.
func (r SixelEncoder) Encode(w io.Writer, img image.Image) error {
	if err := sixel.NewEncoder(w).Encode(img); err != nil {
		return err
	}
	if r.NoNewline {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return err
}

// DefaultEncoder wraps multiple terminal graphic encoders.
type DefaultEncoder struct {
	v    []Encoder
	r    Encoder
	err  error
	once sync.Once
}

// NewDefaultEncoder creates a wrapper for multiple terminal graphic encoders.
func NewDefaultEncoder(v ...Encoder) *DefaultEncoder {
	return &DefaultEncoder{
		v: v,
	}
}

// init initializes the default encoder.
func (r *DefaultEncoder) init() {
	for _, z := range r.v {
		if z.Available() {
			r.r = z
			return
		}
	}
	if r.r == nil {
		r.err = ErrTermGraphicsNotAvailable
	}
}

// Available satisfies the [Encoder] interface.
func (r *DefaultEncoder) Available() bool {
	r.once.Do(r.init)
	return r.r != nil && r.err == nil
}

// Encode satisfies the [Encoder] interface.
func (r *DefaultEncoder) Encode(w io.Writer, img image.Image) error {
	switch r.once.Do(r.init); {
	case r.err != nil:
		return r.err
	case r.r != nil:
		return r.r.Encode(w, img)
	}
	return ErrTermGraphicsNotAvailable
}

// jpegEncode encodes a image to w as a jpeg using [DefaultJPEGQuality].
func jpegEncode(w io.Writer, img image.Image) error {
	return jpeg.Encode(w, img, &jpeg.Options{
		Quality: DefaultJPEGQuality,
	})
}

// chunkEncode writes buf to w in chunks.
func chunkEncode(w io.Writer, buf []byte, size int) error {
	if _, err := fmt.Fprintf(w, "\x1b_Ga=T,f=100,m=1;\x1b\\"); err != nil {
		return err
	}
	n := len(buf)
	for i, j, m := 0, min(size, n), 0; i < n; i, j = j, min(j+size, n) {
		if m = 0; j < n {
			m = 1
		}
		if _, err := fmt.Fprintf(w, "\x1b_Gm=%d;%s\x1b\\", m, buf[i:j]); err != nil {
			return err
		}
	}
	return nil
}

// hasTermGraphics returns true when the $ENV{TERM_GRAPHICS} is the specified
// type.
func hasTermGraphics(typ string) bool {
	tgOnce.Do(func() {
		tgType = strings.ToLower(os.Getenv("TERM_GRAPHICS"))
	})
	return typ == tgType
}

var (
	tgType string
	tgOnce sync.Once
)
