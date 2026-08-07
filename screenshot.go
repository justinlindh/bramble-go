package bramble

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ScreenshotChunk is one response from bramble.screenshot.
//
// The framebuffer does not fit in a single RPC response, so the firmware
// returns it in pieces: Total is the size of the whole frame, Offset is where
// this piece starts, and Data is Len bytes of it, base64 encoded.
type ScreenshotChunk struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Len    int    `json:"len"`
	Data   string `json:"data"`
	// Error is set (with no other field populated) on a build without a
	// graphical UI, where the firmware answers "no graphical ui" as a
	// successful result rather than an RPC error.
	Error string `json:"error,omitempty"`
}

// Screenshot is a complete captured frame.
//
// Pixels is the raw framebuffer exactly as the device holds it, not decoded
// image data. For Format "rgb565" that is two bytes per pixel, LITTLE endian,
// row major: the low byte of each pixel comes first. Decoding it big endian
// produces a recognisable but wrongly coloured image, which is the failure that
// looks like a hardware problem and is not.
type Screenshot struct {
	Width  int
	Height int
	Format string
	Pixels []byte
}

// Screenshot captures the device display and returns the assembled frame.
//
// It issues one capture call followed by as many fetches as the frame needs.
// The capture is taken once, at the start; later calls only page out the frame
// already held on the device, so the result is a single consistent image rather
// than a composite of several.
func (c *Client) Screenshot(ctx context.Context) (*Screenshot, error) {
	return assembleScreenshot(ctx, func(ctx context.Context, params map[string]any) (*ScreenshotChunk, error) {
		var chunk ScreenshotChunk
		raw, err := c.Call(ctx, "bramble.screenshot", params)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil, fmt.Errorf("bramble: decode ScreenshotChunk: %w", err)
		}
		return &chunk, nil
	})
}

// screenshotFetch fetches one chunk with the given params.
type screenshotFetch func(ctx context.Context, params map[string]any) (*ScreenshotChunk, error)

// assembleScreenshot drives the paged fetch. Split from Screenshot so the
// paging contract can be tested without a transport: every loop-termination
// and consistency rule below is a way a real device response has to be
// rejected rather than turned into a silently corrupt image.
func assembleScreenshot(ctx context.Context, fetch screenshotFetch) (*Screenshot, error) {
	// capture:true takes a fresh frame and returns its first chunk. Passing an
	// offset alongside it would be contradictory, and the firmware resolves
	// that by ignoring the offset, so the first call sends capture alone.
	first, err := fetch(ctx, map[string]any{"capture": true})
	if err != nil {
		return nil, err
	}
	if first.Error != "" {
		return nil, fmt.Errorf("bramble: screenshot: %s", first.Error)
	}
	if first.Total <= 0 {
		return nil, fmt.Errorf("bramble: screenshot: device reported an empty frame (total=%d)", first.Total)
	}

	shot := &Screenshot{
		Width:  first.Width,
		Height: first.Height,
		Format: first.Format,
		Pixels: make([]byte, 0, first.Total),
	}

	chunk := first
	for {
		payload, err := base64.StdEncoding.DecodeString(chunk.Data)
		if err != nil {
			return nil, fmt.Errorf("bramble: screenshot: decode chunk at offset %d: %w", chunk.Offset, err)
		}
		if chunk.Offset != len(shot.Pixels) {
			return nil, fmt.Errorf("bramble: screenshot: chunk out of order (want offset %d, got %d)",
				len(shot.Pixels), chunk.Offset)
		}
		if len(payload)+len(shot.Pixels) > first.Total {
			return nil, fmt.Errorf("bramble: screenshot: chunk at offset %d overruns the frame (%d bytes past %d)",
				chunk.Offset, len(payload)+len(shot.Pixels)-first.Total, first.Total)
		}
		// A zero-length chunk with bytes still outstanding never advances, so
		// accepting one would loop until the context expired.
		if len(payload) == 0 {
			return nil, fmt.Errorf("bramble: screenshot: empty chunk at offset %d with %d bytes outstanding",
				chunk.Offset, first.Total-len(shot.Pixels))
		}
		shot.Pixels = append(shot.Pixels, payload...)

		if len(shot.Pixels) == first.Total {
			return shot, nil
		}

		// capture is deliberately absent here: the firmware only recaptures
		// when asked, so the rest of the frame comes from the frame already
		// held on the device.
		chunk, err = fetch(ctx, map[string]any{"offset": len(shot.Pixels)})
		if err != nil {
			return nil, err
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("bramble: screenshot: %s", chunk.Error)
		}
		if chunk.Total != first.Total {
			return nil, fmt.Errorf("bramble: screenshot: frame changed size mid transfer (%d then %d)",
				first.Total, chunk.Total)
		}
	}
}
