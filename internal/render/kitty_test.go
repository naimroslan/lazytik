package render

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

func TestKittyCellSize(t *testing.T) {
	w, h := Kitty{}.CellSize(20, 10)
	if w != 20*kittyCellW || h != 10*kittyCellH {
		t.Fatalf("CellSize(20,10)=(%d,%d) want (%d,%d)", w, h, 20*kittyCellW, 10*kittyCellH)
	}
}

func TestKittyRenderStructure(t *testing.T) {
	cols, rows := 4, 2
	w, h := Kitty{}.CellSize(cols, rows) // 24 x 24
	rgb := make([]byte, w*h*3)
	for i := range rgb {
		rgb[i] = byte(i)
	}
	out := Kitty{}.Render(rgb, w, h)

	// Wrapped in a synchronized update for tear-free painting.
	if !strings.HasPrefix(out, "\x1b[?2026h") || !strings.HasSuffix(out, "\x1b[?2026l") {
		t.Error("frame not wrapped in synchronized-update markers")
	}
	// No per-frame delete (that caused the clear→draw flicker).
	if strings.Contains(out, "a=d") {
		t.Error("should not delete the image each frame (causes flicker)")
	}
	// Transmit+display with stable image+placement ids, C=1 (don't move cursor),
	// and the correct source pixel (s/v) and display cell (c/r) dimensions.
	keys := []string{
		"a=T", "f=24", "o=z", "q=2", "C=1", "i=1", "p=1",
		fmt.Sprintf("s=%d", w), fmt.Sprintf("v=%d", h),
		fmt.Sprintf("c=%d", cols), fmt.Sprintf("r=%d", rows),
	}
	for _, key := range keys {
		if !strings.Contains(out, key) {
			t.Errorf("transmit escape missing %q", key)
		}
	}
	// Every graphics escape must be terminated.
	if strings.Count(out, "\x1b_G") != strings.Count(out, "\x1b\\") {
		t.Error("unbalanced graphics escape terminators")
	}
}

func TestKittyPayloadRoundTrips(t *testing.T) {
	cols, rows := 4, 2
	w, h := Kitty{}.CellSize(cols, rows)
	rgb := make([]byte, w*h*3)
	for i := range rgb {
		rgb[i] = byte(i * 7)
	}
	out := Kitty{}.Render(rgb, w, h)

	// Extract base64 payload(s): everything after the first ';' in each transmit
	// escape, concatenated. Skip the leading delete escape.
	re := regexp.MustCompile(`\x1b_G[^;]*;([^\x1b]*)\x1b\\`)
	var b64 strings.Builder
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		b64.WriteString(m[1])
	}
	raw, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("payload not valid base64: %v", err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("payload not valid zlib: %v", err)
	}
	got, _ := io.ReadAll(zr)
	if !bytes.Equal(got, rgb) {
		t.Errorf("round-tripped %d bytes, want %d identical", len(got), len(rgb))
	}
}

func TestKittyRejectsBadInput(t *testing.T) {
	if got := (Kitty{}).Render([]byte{1, 2, 3}, 24, 24); got != "" {
		t.Errorf("undersized buffer should render empty, got %d bytes", len(got))
	}
}

func TestForSelectsBackend(t *testing.T) {
	if For("kitty").Name() != "kitty" {
		t.Error("For(kitty) should return the kitty backend")
	}
	if For("halfblock").Name() != "halfblock" {
		t.Error("For(halfblock) should return half-blocks")
	}
	if For("anything-else").Name() != "halfblock" {
		t.Error("For(unknown) should fall back to half-blocks")
	}
}
