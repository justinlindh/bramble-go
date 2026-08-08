package bramble

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// pagedFetch serves a frame the way the firmware does: capture:true returns the
// first chunk, an offset returns the chunk starting there, each capped at
// chunkSize. It also records the params of every call so a test can assert the
// capture happened exactly once.
type pagedFetch struct {
	frame     []byte
	chunkSize int
	width     int
	height    int
	calls     []map[string]any
}

func (p *pagedFetch) fetch(_ context.Context, params map[string]any) (*ScreenshotChunk, error) {
	p.calls = append(p.calls, params)

	offset := 0
	if v, ok := params["offset"]; ok {
		offset = v.(int)
	}
	end := offset + p.chunkSize
	if end > len(p.frame) {
		end = len(p.frame)
	}
	payload := p.frame[offset:end]
	return &ScreenshotChunk{
		Width:  p.width,
		Height: p.height,
		Format: "rgb565",
		Total:  len(p.frame),
		Offset: offset,
		Len:    len(payload),
		Data:   base64.StdEncoding.EncodeToString(payload),
	}, nil
}

func testFrame(n int) []byte {
	f := make([]byte, n)
	for i := range f {
		f[i] = byte(i % 251)
	}
	return f
}

func TestAssembleScreenshotReassemblesEveryChunkInOrder(t *testing.T) {
	frame := testFrame(320 * 240 * 2)
	p := &pagedFetch{frame: frame, chunkSize: 4096, width: 320, height: 240}

	shot, err := assembleScreenshot(context.Background(), p.fetch)
	if err != nil {
		t.Fatalf("assembleScreenshot: %v", err)
	}
	if len(shot.Pixels) != len(frame) {
		t.Fatalf("got %d bytes, want %d", len(shot.Pixels), len(frame))
	}
	if string(shot.Pixels) != string(frame) {
		t.Error("reassembled frame does not match the source frame")
	}
	if shot.Width != 320 || shot.Height != 240 || shot.Format != "rgb565" {
		t.Errorf("got %dx%d %s, want 320x240 rgb565", shot.Width, shot.Height, shot.Format)
	}
}

func TestAssembleScreenshotCapturesOnceThenPagesByOffset(t *testing.T) {
	// Recapturing mid transfer would splice together frames from different
	// instants, so the capture must happen on the first call and never again.
	frame := testFrame(10000)
	p := &pagedFetch{frame: frame, chunkSize: 4096}

	if _, err := assembleScreenshot(context.Background(), p.fetch); err != nil {
		t.Fatalf("assembleScreenshot: %v", err)
	}
	if len(p.calls) != 3 {
		t.Fatalf("made %d calls, want 3 for a 10000 byte frame in 4096 byte chunks", len(p.calls))
	}
	if p.calls[0]["capture"] != true {
		t.Error("first call did not request a capture")
	}
	for i, c := range p.calls[1:] {
		if _, ok := c["capture"]; ok {
			t.Errorf("call %d requested another capture", i+1)
		}
	}
	if p.calls[1]["offset"] != 4096 || p.calls[2]["offset"] != 8192 {
		t.Errorf("offsets were %v and %v, want 4096 and 8192", p.calls[1]["offset"], p.calls[2]["offset"])
	}
}

func TestAssembleScreenshotSingleChunkFrameMakesOneCall(t *testing.T) {
	frame := testFrame(64)
	p := &pagedFetch{frame: frame, chunkSize: 4096}

	shot, err := assembleScreenshot(context.Background(), p.fetch)
	if err != nil {
		t.Fatalf("assembleScreenshot: %v", err)
	}
	if len(p.calls) != 1 {
		t.Errorf("made %d calls, want 1", len(p.calls))
	}
	if string(shot.Pixels) != string(frame) {
		t.Error("reassembled frame does not match the source frame")
	}
}

func TestAssembleScreenshotExactMultipleOfChunkSizeTerminates(t *testing.T) {
	// The boundary case: the last chunk fills the frame exactly, so the loop
	// has to stop on the byte count rather than waiting for a short chunk.
	frame := testFrame(8192)
	p := &pagedFetch{frame: frame, chunkSize: 4096}

	shot, err := assembleScreenshot(context.Background(), p.fetch)
	if err != nil {
		t.Fatalf("assembleScreenshot: %v", err)
	}
	if len(p.calls) != 2 {
		t.Errorf("made %d calls, want 2", len(p.calls))
	}
	if len(shot.Pixels) != 8192 {
		t.Errorf("got %d bytes, want 8192", len(shot.Pixels))
	}
}

func TestAssembleScreenshotRejectsEmptyChunkMidFrame(t *testing.T) {
	// A chunk that carries nothing never advances the offset, so without this
	// guard the loop spins until the context expires.
	calls := 0
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		calls++
		if calls == 1 {
			return &ScreenshotChunk{Total: 100, Offset: 0, Data: base64.StdEncoding.EncodeToString(testFrame(50))}, nil
		}
		return &ScreenshotChunk{Total: 100, Offset: 50, Data: ""}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil {
		t.Fatal("expected an error for an empty chunk mid frame")
	}
	if !strings.Contains(err.Error(), "empty chunk") {
		t.Errorf("error was %q, want it to name the empty chunk", err)
	}
}

func TestAssembleScreenshotRejectsOutOfOrderChunk(t *testing.T) {
	calls := 0
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		calls++
		if calls == 1 {
			return &ScreenshotChunk{Total: 100, Offset: 0, Data: base64.StdEncoding.EncodeToString(testFrame(50))}, nil
		}
		// Offset does not continue from where the first chunk ended.
		return &ScreenshotChunk{Total: 100, Offset: 70, Data: base64.StdEncoding.EncodeToString(testFrame(30))}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "out of order") {
		t.Fatalf("error was %v, want an out of order rejection", err)
	}
}

func TestAssembleScreenshotRejectsOverrunningChunk(t *testing.T) {
	calls := 0
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		calls++
		if calls == 1 {
			return &ScreenshotChunk{Total: 100, Offset: 0, Data: base64.StdEncoding.EncodeToString(testFrame(50))}, nil
		}
		return &ScreenshotChunk{Total: 100, Offset: 50, Data: base64.StdEncoding.EncodeToString(testFrame(80))}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "overruns") {
		t.Fatalf("error was %v, want an overrun rejection", err)
	}
}

func TestAssembleScreenshotRejectsFrameThatChangesSize(t *testing.T) {
	calls := 0
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		calls++
		if calls == 1 {
			return &ScreenshotChunk{Total: 100, Offset: 0, Data: base64.StdEncoding.EncodeToString(testFrame(50))}, nil
		}
		return &ScreenshotChunk{Total: 200, Offset: 50, Data: base64.StdEncoding.EncodeToString(testFrame(50))}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "changed size") {
		t.Fatalf("error was %v, want a size change rejection", err)
	}
}

func TestAssembleScreenshotSurfacesNoGraphicalUI(t *testing.T) {
	// A headless build answers with a result carrying an error field rather
	// than an RPC error, so it has to be turned into an error here or the
	// caller gets a zero sized image and no explanation.
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		return &ScreenshotChunk{Error: "no graphical ui"}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "no graphical ui") {
		t.Fatalf("error was %v, want it to report no graphical ui", err)
	}
}

func TestAssembleScreenshotRejectsEmptyFrame(t *testing.T) {
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		return &ScreenshotChunk{Total: 0}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "empty frame") {
		t.Fatalf("error was %v, want an empty frame rejection", err)
	}
}

// Total drives the pre-allocation, so it has to be rejected before it is
// trusted: an endpoint reached through Client.Call is not necessarily real
// firmware, and a huge claimed total would otherwise be allocated outright.
// The fetch fails the test if it is called a second time, since rejecting the
// frame means never paging it.
func TestAssembleScreenshotRejectsImplausibleFrameSize(t *testing.T) {
	calls := 0
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		calls++
		if calls > 1 {
			t.Fatal("kept paging a frame whose size was already rejected")
		}
		return &ScreenshotChunk{Width: 4096, Height: 4096, Total: maxScreenshotBytes + 1}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "implausible frame size") {
		t.Fatalf("error was %v, want an implausible frame size rejection", err)
	}
}

// The bound is a ceiling, not a limit on ordinary frames: a frame exactly at
// the cap still assembles.
func TestAssembleScreenshotAcceptsFrameExactlyAtTheCap(t *testing.T) {
	fetch := func(_ context.Context, params map[string]any) (*ScreenshotChunk, error) {
		offset := 0
		if v, ok := params["offset"].(int); ok {
			offset = v
		}
		remaining := maxScreenshotBytes - offset
		return &ScreenshotChunk{
			Width: 4096, Height: 4096, Format: "rgb565",
			Total:  maxScreenshotBytes,
			Offset: offset,
			Len:    remaining,
			Data:   base64.StdEncoding.EncodeToString(make([]byte, remaining)),
		}, nil
	}

	shot, err := assembleScreenshot(context.Background(), fetch)
	if err != nil {
		t.Fatalf("a frame exactly at the cap was rejected: %v", err)
	}
	if len(shot.Pixels) != maxScreenshotBytes {
		t.Fatalf("assembled %d bytes, want %d", len(shot.Pixels), maxScreenshotBytes)
	}
}

func TestAssembleScreenshotRejectsMalformedBase64(t *testing.T) {
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		return &ScreenshotChunk{Total: 100, Offset: 0, Data: "!!!not base64!!!"}, nil
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if err == nil || !strings.Contains(err.Error(), "decode chunk") {
		t.Fatalf("error was %v, want a decode failure", err)
	}
}

func TestAssembleScreenshotPropagatesTransportError(t *testing.T) {
	want := errors.New("transport is down")
	fetch := func(_ context.Context, _ map[string]any) (*ScreenshotChunk, error) {
		return nil, want
	}

	_, err := assembleScreenshot(context.Background(), fetch)
	if !errors.Is(err, want) {
		t.Fatalf("error was %v, want it to wrap %v", err, want)
	}
}
