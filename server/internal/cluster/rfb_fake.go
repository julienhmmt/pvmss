// Package cluster — T10 fake RFB 3.8 server.
//
// This is the single most scrutinized piece of tranche T10: it is what makes
// constitution XI's "le fake path must be genuinely functional, not a stub"
// gate TRUE rather than merely asserted. A real noVNC client connects to this
// server, completes a real RFB 3.8 handshake, and decodes a real
// FramebufferUpdate — not a canned HTTP response, not a stub that closes after
// the upgrade.
//
// The scope is deliberately minimal (plan.md research decisions): exactly the
// message set a noVNC client needs to handshake, render one static checkerboard
// framebuffer, and exercise the clipboard + input round-trips. No multiple
// encodings, no resizing, no animating framebuffer — there is no OS underneath
// to animate, and each of those would be a stub pretending to be a feature,
// which is worse than not having it (constitution VIII).
package cluster

import (
	"context"
	"encoding/binary"
	"io"
)

// RFB protocol constants used by the fake handshake. See RFC 6143 (RFB 3.8).
const (
	rfbProtocolVersion = "RFB 003.008\n"
	rfbSecurityNone    = 1
	// Message types — server-to-client.
	rfbMsgFramebufferUpdate = 0
	rfbMsgServerCutText     = 3
	// Message types — client-to-server.
	rfbMsgSetPixelFormat           = 0
	rfbMsgSetEncodings             = 2
	rfbMsgFramebufferUpdateRequest = 3
	rfbMsgKeyEvent                 = 4
	rfbMsgPointerEvent             = 5
	rfbMsgClientCutText            = 6
	// Encodings.
	rfbEncodingRaw = 0
)

// Fake framebuffer geometry and content (plan.md research decisions). Small
// and fixed so the checkerboard is visually distinct from a blank canvas
// without any dirty-region tracking.
const (
	rfbFakeWidth  = 640
	rfbFakeHeight = 480
	rfbFakeName   = "PVMSS fake console"
	// rfbFakeClipboardText is the single ServerCutText payload sent right
	// after ServerInit — User Story 3's "copy from VM" content.
	rfbFakeClipboardText = "hello from the fake console"
)

// rfbPixelFormat is the 16-byte PIXEL_FORMAT structure (RFC 6143 §7.7). The
// fake server always speaks 32bpp true-color, which is what noVNC defaults to
// requesting — the client's SetPixelFormat is read and discarded.
type rfbPixelFormat struct {
	BitsPerPixel uint8
	Depth        uint8
	BigEndian    uint8
	TrueColor    uint8
	RedMax       uint16
	GreenMax     uint16
	BlueMax      uint16
	RedShift     uint8
	GreenShift   uint8
	BlueShift    uint8
	Padding1     uint8
	Padding2     uint8
	Padding3     uint8
}

// rfbServerInit is the ServerInit message body (RFC 6143 §7.3.2). Fixed
// resolution, 32bpp true-color, and the fake console name.
type rfbServerInit struct {
	Width       uint16
	Height      uint16
	PixelFormat rfbPixelFormat
	NameLength  uint32
}

// rfbFakeServe speaks the minimal RFB 3.8 handshake and serves one static
// checkerboard framebuffer against peer. It blocks until peer closes or a
// malformed initial handshake byte is seen (in which case it closes rather
// than guessing). This is the fake ConsoleRelay.RelayConsole implementation —
// there is no second, separately-dialed connection in the fake path; the
// "relay" IS the fake server (data-model.md).
func rfbFakeServe(ctx context.Context, peer io.ReadWriteCloser) error {
	defer func() { _ = peer.Close() }()

	if err := rfbHandshake(peer); err != nil {
		return err
	}

	if err := rfbSendServerCutText(peer, rfbFakeClipboardText); err != nil {
		return err
	}

	return rfbServeMessages(ctx, peer)
}

// rfbHandshake performs the version exchange, security negotiation, and
// ServerInit — exactly the sequence a real noVNC client needs to complete
// before it will process any further messages.
func rfbHandshake(peer io.ReadWriteCloser) error {
	// Step 1: server sends ProtocolVersion.
	if _, err := peer.Write([]byte(rfbProtocolVersion)); err != nil {
		return err
	}

	// Step 2: read and ignore the client's ProtocolVersion (we always speak 3.8).
	clientVersion := make([]byte, len(rfbProtocolVersion))
	if _, err := io.ReadFull(peer, clientVersion); err != nil {
		return err
	}

	// Step 3: server sends SecurityTypes — one type: None.
	if _, err := peer.Write([]byte{1, rfbSecurityNone}); err != nil {
		return err
	}

	// Step 4: read and ignore the client's security choice (only None was offered).
	choice := make([]byte, 1)
	if _, err := io.ReadFull(peer, choice); err != nil {
		return err
	}

	// Step 5: server sends SecurityResult (0 = OK). RFB 3.8 requires this even
	// for the None security type.
	if err := binary.Write(peer, binary.BigEndian, uint32(0)); err != nil {
		return err
	}

	// Step 6: read and ignore ClientInit (shared-flag byte).
	clientInit := make([]byte, 1)
	if _, err := io.ReadFull(peer, clientInit); err != nil {
		return err
	}

	// Step 7: server sends ServerInit.
	init := rfbServerInit{
		Width:  rfbFakeWidth,
		Height: rfbFakeHeight,
		PixelFormat: rfbPixelFormat{
			BitsPerPixel: 32,
			Depth:        24,
			TrueColor:    1,
			RedMax:       255,
			GreenMax:     255,
			BlueMax:      255,
			RedShift:     16,
			GreenShift:   8,
			BlueShift:    0,
		},
		NameLength: uint32(len(rfbFakeName)),
	}
	if err := binary.Write(peer, binary.BigEndian, &init); err != nil {
		return err
	}

	if _, err := peer.Write([]byte(rfbFakeName)); err != nil {
		return err
	}

	return nil
}

// rfbSendServerCutText sends one ServerCutText message. Called once right
// after ServerInit so User Story 3's "copy from VM" has real content.
func rfbSendServerCutText(peer io.Writer, text string) error {
	header := make([]byte, 8)
	header[0] = rfbMsgServerCutText
	// header[1:4] is padding.
	binary.BigEndian.PutUint32(header[4:8], uint32(len(text))) //nolint:gosec // len(text) is bounded by a small server-cut-text payload

	if _, err := peer.Write(header); err != nil {
		return err
	}

	_, err := peer.Write([]byte(text))

	return err
}

// rfbServeMessages is the post-handshake message loop. It reads one
// client-to-server message at a time, dispatches on the message-type byte, and
// answers FramebufferUpdateRequest with the checkerboard. SetPixelFormat,
// SetEncodings, PointerEvent, KeyEvent, and ClientCutText are read and
// discarded without validation — there is no OS underneath for input to
// affect (plan.md research decisions, constitution VIII).
func rfbServeMessages(ctx context.Context, peer io.ReadWriteCloser) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		msgType := make([]byte, 1)
		if _, err := io.ReadFull(peer, msgType); err != nil {
			return err
		}

		if err := handleRFBMessage(peer, msgType[0]); err != nil {
			return err
		}
	}
}

// handleRFBMessage dispatches a single client-to-server message by type.
// SetPixelFormat, SetEncodings, PointerEvent, KeyEvent, and ClientCutText are
// read and discarded without validation — there is no OS underneath for input
// to affect (plan.md research decisions, constitution VIII).
func handleRFBMessage(peer io.ReadWriteCloser, msgType byte) error {
	switch msgType {
	case rfbMsgSetPixelFormat:
		// 3 bytes padding + 16 bytes PIXEL_FORMAT.
		_, err := io.CopyN(io.Discard, peer, 19)

		return err
	case rfbMsgSetEncodings:
		// 2 bytes padding + 2 bytes count + count*4 bytes encodings.
		return discardEncodings(peer)
	case rfbMsgFramebufferUpdateRequest:
		// 1 byte incremental + 8 bytes rect.
		if _, err := io.CopyN(io.Discard, peer, 9); err != nil {
			return err
		}

		return rfbSendFramebufferUpdate(peer)
	case rfbMsgKeyEvent:
		// 1 byte down + 2 padding + 4 keysym.
		_, err := io.CopyN(io.Discard, peer, 7)

		return err
	case rfbMsgPointerEvent:
		// 1 byte mask + 2 x + 2 y.
		_, err := io.CopyN(io.Discard, peer, 5)

		return err
	case rfbMsgClientCutText:
		// 3 padding + 4 length + length bytes.
		return discardClientCutText(peer)
	default:
		// Unknown message type — the spec says close rather than guess.
		return errUnknownRFBMessage(msgType)
	}
}

// rfbSendFramebufferUpdate sends one FramebufferUpdate containing a single
// Raw-encoded rectangle covering the whole framebuffer, filled with the
// checkerboard pattern generated once at start.
func rfbSendFramebufferUpdate(peer io.Writer) error {
	header := make([]byte, 4)
	header[0] = rfbMsgFramebufferUpdate
	// header[1] is padding.
	binary.BigEndian.PutUint16(header[2:4], 1) // 1 rectangle

	if _, err := peer.Write(header); err != nil {
		return err
	}

	rect := make([]byte, 12)
	binary.BigEndian.PutUint16(rect[0:2], 0) // x
	binary.BigEndian.PutUint16(rect[2:4], 0) // y
	binary.BigEndian.PutUint16(rect[4:6], rfbFakeWidth)
	binary.BigEndian.PutUint16(rect[6:8], rfbFakeHeight)
	binary.BigEndian.PutUint32(rect[8:12], rfbEncodingRaw)

	if _, err := peer.Write(rect); err != nil {
		return err
	}

	_, err := peer.Write(checkerboardPixels(rfbFakeWidth, rfbFakeHeight))

	return err
}

// discardEncodings reads the 2 padding bytes, the 2-byte encoding count, and
// that many 4-byte encoding values from peer.
func discardEncodings(peer io.Reader) error {
	header := make([]byte, 4) // 2 padding + 2 count
	if _, err := io.ReadFull(peer, header); err != nil {
		return err
	}

	count := binary.BigEndian.Uint16(header[2:4])
	if _, err := io.CopyN(io.Discard, peer, int64(count)*4); err != nil {
		return err
	}

	return nil
}

// discardClientCutText reads the 3 padding bytes, the 4-byte length, and that
// many bytes of text from peer.
func discardClientCutText(peer io.Reader) error {
	header := make([]byte, 7) // 3 padding + 4 length
	if _, err := io.ReadFull(peer, header); err != nil {
		return err
	}

	length := binary.BigEndian.Uint32(header[3:7])
	if _, err := io.CopyN(io.Discard, peer, int64(length)); err != nil {
		return err
	}

	return nil
}

// checkerboardPixels returns the Raw pixel bytes for a width×height 32bpp
// framebuffer filled with a 32×32 checkerboard of two distinct colors. The
// pattern is visually distinct from a blank canvas so a demo can tell
// "connected and rendering" from "nothing happened" (plan.md research
// decisions). Generated deterministically — the same fixture the byte-level
// test asserts against.
func checkerboardPixels(width, height int) []byte {
	// Two colors in 32bpp BGRA (little-endian) / RGBX layout matching the
	// PIXEL_FORMAT declared in ServerInit (red shift 16, green 8, blue 0).
	// noVNC reads pixels according to the negotiated pixel format, so we lay
	// them out as R,G,B,unused per pixel.
	white := [4]byte{255, 255, 255, 255}
	black := [4]byte{0, 0, 0, 255}

	pixels := make([]byte, width*height*4)
	for y := range height {
		for x := range width {
			cell := ((x/32)+(y/32))%2 == 0

			color := white
			if !cell {
				color = black
			}

			offset := (y*width + x) * 4
			copy(pixels[offset:offset+4], color[:])
		}
	}

	return pixels
}

// errUnknownRFBMessage is returned when the fake server receives a message
// type it does not implement — the spec says close rather than guess.
type rfbMessageError byte

func (e rfbMessageError) Error() string { return "rfb: unknown message type" }
func errUnknownRFBMessage(b byte) error { return rfbMessageError(b) }
