package cluster

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

// TestRFBFakeHandshake_VersionAndSecurity — T007: the fake server sends the
// RFB 003.008 version string, then offers exactly one security type (None=1),
// then a SecurityResult of 0 (OK). A real noVNC client expects exactly this
// sequence to complete its handshake.
//
//nolint:paralleltest // serial: shared fake fixture
func TestRFBFakeHandshake_VersionAndSecurity(t *testing.T) {
	client, server := netPipe(t)
	defer func() { _ = client.Close(); _ = server.Close() }()

	done := serveFake(t, server)

	// Step 1: server sends ProtocolVersion.
	gotVersion := make([]byte, len(rfbProtocolVersion))
	if _, err := io.ReadFull(client, gotVersion); err != nil {
		t.Fatalf("read ProtocolVersion: %v", err)
	}

	if string(gotVersion) != rfbProtocolVersion {
		t.Fatalf("ProtocolVersion = %q, want %q", gotVersion, rfbProtocolVersion)
	}

	// Step 2: client sends its ProtocolVersion (we echo 3.8 back).
	if _, err := client.Write([]byte(rfbProtocolVersion)); err != nil {
		t.Fatalf("write client ProtocolVersion: %v", err)
	}

	// Step 3: server sends SecurityTypes — [count, types...].
	count := make([]byte, 1)
	if _, err := io.ReadFull(client, count); err != nil {
		t.Fatalf("read SecurityTypes count: %v", err)
	}

	if count[0] != 1 {
		t.Fatalf("SecurityTypes count = %d, want 1 (None only)", count[0])
	}

	types := make([]byte, count[0])
	if _, err := io.ReadFull(client, types); err != nil {
		t.Fatalf("read SecurityTypes: %v", err)
	}

	if types[0] != rfbSecurityNone {
		t.Fatalf("security type = %d, want %d (None)", types[0], rfbSecurityNone)
	}

	// Step 4: client picks the only offered type.
	if _, err := client.Write([]byte{rfbSecurityNone}); err != nil {
		t.Fatalf("write security choice: %v", err)
	}

	// Step 5: server sends SecurityResult (4 bytes, 0 = OK).
	result := make([]byte, 4)
	if _, err := io.ReadFull(client, result); err != nil {
		t.Fatalf("read SecurityResult: %v", err)
	}

	if binary.BigEndian.Uint32(result) != 0 {
		t.Fatalf("SecurityResult = %d, want 0 (OK)", binary.BigEndian.Uint32(result))
	}

	// Step 6: client sends ClientInit (shared-flag byte = 0).
	if _, err := client.Write([]byte{0}); err != nil {
		t.Fatalf("write ClientInit: %v", err)
	}

	// Step 7: server sends ServerInit.
	var init rfbServerInit
	if err := binary.Read(client, binary.BigEndian, &init); err != nil {
		t.Fatalf("read ServerInit: %v", err)
	}

	if init.Width != rfbFakeWidth || init.Height != rfbFakeHeight {
		t.Fatalf("ServerInit resolution = %dx%d, want %dx%d", init.Width, init.Height, rfbFakeWidth, rfbFakeHeight)
	}

	if init.PixelFormat.BitsPerPixel != 32 {
		t.Fatalf("bitsPerPixel = %d, want 32", init.PixelFormat.BitsPerPixel)
	}

	if init.PixelFormat.TrueColor != 1 {
		t.Fatalf("trueColor = %d, want 1", init.PixelFormat.TrueColor)
	}

	if init.NameLength != uint32(len(rfbFakeName)) {
		t.Fatalf("nameLength = %d, want %d", init.NameLength, len(rfbFakeName))
	}

	name := make([]byte, init.NameLength)
	if _, err := io.ReadFull(client, name); err != nil {
		t.Fatalf("read server name: %v", err)
	}

	if string(name) != rfbFakeName {
		t.Fatalf("name = %q, want %q", name, rfbFakeName)
	}

	_ = done // allow server goroutine to finish on close
}

// TestRFBFakeHandshake_ServerCutTextAfterInit — T007: right after ServerInit,
// the fake server sends one ServerCutText with the fixed fixture string so
// User Story 3's "copy from VM" has real content.
//
//nolint:paralleltest // serial: shared fake fixture
func TestRFBFakeHandshake_ServerCutTextAfterInit(t *testing.T) {
	client := connectAndCompleteHandshake(t)

	// The first message after ServerInit is ServerCutText (msg-type 3).
	header := make([]byte, 1)
	if _, err := io.ReadFull(client, header); err != nil {
		t.Fatalf("read msg-type: %v", err)
	}

	if header[0] != rfbMsgServerCutText {
		t.Fatalf("msg-type = %d, want %d (ServerCutText)", header[0], rfbMsgServerCutText)
	}

	// 3 bytes padding + 4 bytes length (big-endian).
	padding := make([]byte, 3)
	if _, err := io.ReadFull(client, padding); err != nil {
		t.Fatalf("read padding: %v", err)
	}

	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(client, lengthBuf); err != nil {
		t.Fatalf("read length: %v", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if int(length) != len(rfbFakeClipboardText) {
		t.Fatalf("ServerCutText length = %d, want %d", length, len(rfbFakeClipboardText))
	}

	text := make([]byte, length)
	if _, err := io.ReadFull(client, text); err != nil {
		t.Fatalf("read clipboard text: %v", err)
	}

	if string(text) != rfbFakeClipboardText {
		t.Fatalf("clipboard text = %q, want %q", text, rfbFakeClipboardText)
	}
}

// TestRFBFakeHandshake_FramebufferUpdateIsCheckerboard — T007: a
// FramebufferUpdateRequest is answered with one Raw-encoded rectangle covering
// the whole framebuffer, filled with the checkerboard pattern.
//
//nolint:paralleltest // serial: shared fake fixture
func TestRFBFakeHandshake_FramebufferUpdateIsCheckerboard(t *testing.T) {
	client := connectAndCompleteHandshake(t)
	skipServerCutText(t, client)

	// Client sends SetPixelFormat (msg-type 0): 1 byte type + 3 padding + 16 bytes format.
	pixelFormat := make([]byte, 20)

	pixelFormat[0] = rfbMsgSetPixelFormat
	if _, err := client.Write(pixelFormat); err != nil {
		t.Fatalf("write SetPixelFormat: %v", err)
	}

	// Client sends SetEncodings (msg-type 2): 1 type + 2 padding + 2 count(=0) = 5 bytes.
	encodings := make([]byte, 5)

	encodings[0] = rfbMsgSetEncodings
	if _, err := client.Write(encodings); err != nil {
		t.Fatalf("write SetEncodings: %v", err)
	}

	// Client sends FramebufferUpdateRequest (msg-type 3): type + 1 incremental + 8 rect.
	fbur := make([]byte, 10)

	fbur[0] = rfbMsgFramebufferUpdateRequest
	if _, err := client.Write(fbur); err != nil {
		t.Fatalf("write FramebufferUpdateRequest: %v", err)
	}

	// Server responds with FramebufferUpdate (msg-type 0).
	header := make([]byte, 1)
	if _, err := io.ReadFull(client, header); err != nil {
		t.Fatalf("read FramebufferUpdate type: %v", err)
	}

	if header[0] != rfbMsgFramebufferUpdate {
		t.Fatalf("msg-type = %d, want %d (FramebufferUpdate)", header[0], rfbMsgFramebufferUpdate)
	}

	padding := make([]byte, 1)
	if _, err := io.ReadFull(client, padding); err != nil {
		t.Fatalf("read padding: %v", err)
	}

	numRects := make([]byte, 2)
	if _, err := io.ReadFull(client, numRects); err != nil {
		t.Fatalf("read num-rectangles: %v", err)
	}

	if binary.BigEndian.Uint16(numRects) != 1 {
		t.Fatalf("num-rectangles = %d, want 1", binary.BigEndian.Uint16(numRects))
	}

	// Rectangle header: x(2) + y(2) + width(2) + height(2) + encoding(4).
	rect := make([]byte, 12)
	if _, err := io.ReadFull(client, rect); err != nil {
		t.Fatalf("read rect header: %v", err)
	}

	width := binary.BigEndian.Uint16(rect[4:6])
	height := binary.BigEndian.Uint16(rect[6:8])
	encoding := binary.BigEndian.Uint32(rect[8:12])

	if width != rfbFakeWidth || height != rfbFakeHeight {
		t.Fatalf("rect = %dx%d, want %dx%d", width, height, rfbFakeWidth, rfbFakeHeight)
	}

	if encoding != rfbEncodingRaw {
		t.Fatalf("encoding = %d, want %d (Raw)", encoding, rfbEncodingRaw)
	}

	// Pixel data: width*height*4 bytes (32bpp), matching the checkerboard.
	pixelBytes := make([]byte, int(width)*int(height)*4)
	if _, err := io.ReadFull(client, pixelBytes); err != nil {
		t.Fatalf("read pixel data: %v", err)
	}

	want := checkerboardPixels(int(width), int(height))
	if !bytes.Equal(pixelBytes, want) {
		t.Fatalf("pixel data does not match checkerboard fixture (got %d bytes, want %d)", len(pixelBytes), len(want))
	}
}

// TestRFBFakeHandshake_AcceptsInputMessages — T007/T029: PointerEvent,
// KeyEvent, and ClientCutText are all accepted without closing the connection.
//
//nolint:paralleltest // serial: shared fake fixture
func TestRFBFakeHandshake_AcceptsInputMessages(t *testing.T) {
	client := connectAndCompleteHandshake(t)
	skipServerCutText(t, client)

	// PointerEvent (msg-type 5): type + 1 mask + 2 x + 2 y.
	pointer := make([]byte, 6)

	pointer[0] = rfbMsgPointerEvent
	if _, err := client.Write(pointer); err != nil {
		t.Fatalf("write PointerEvent: %v", err)
	}

	// KeyEvent (msg-type 4): type + 1 down + 2 padding + 4 keysym.
	key := make([]byte, 8)

	key[0] = rfbMsgKeyEvent
	if _, err := client.Write(key); err != nil {
		t.Fatalf("write KeyEvent: %v", err)
	}

	// ClientCutText (msg-type 6): type + 3 padding + 4 length + length bytes.
	text := "hello from client"
	cut := make([]byte, 8+len(text))
	cut[0] = rfbMsgClientCutText
	binary.BigEndian.PutUint32(cut[4:8], uint32(len(text))) //nolint:gosec // len(text) is a fixed small test fixture
	copy(cut[8:], text)

	if _, err := client.Write(cut); err != nil {
		t.Fatalf("write ClientCutText: %v", err)
	}

	// The connection must still be alive — send a FramebufferUpdateRequest and
	// expect a FramebufferUpdate response.
	fbur := make([]byte, 10)

	fbur[0] = rfbMsgFramebufferUpdateRequest
	if _, err := client.Write(fbur); err != nil {
		t.Fatalf("write FramebufferUpdateRequest: %v", err)
	}

	header := make([]byte, 1)
	if _, err := io.ReadFull(client, header); err != nil {
		t.Fatalf("connection died after input messages: %v", err)
	}

	if header[0] != rfbMsgFramebufferUpdate {
		t.Fatalf("msg-type = %d, want %d (FramebufferUpdate)", header[0], rfbMsgFramebufferUpdate)
	}
}

// --- helpers ---

// netPipe returns a synchronous, in-memory, full-duplex byte pipe. The fake
// RFB server reads and writes raw bytes — it does not need real WebSocket
// framing, so a plain net.Pipe is the most faithful test double for the
// io.ReadWriteCloser the real relay hands to Serve().
func netPipe(t *testing.T) (io.ReadWriteCloser, io.ReadWriteCloser) {
	t.Helper()

	a, b := net.Pipe()

	return a, b
}

// serveFake starts rfbFakeServe against server in a goroutine, returning a
// channel that yields the serve result. The test closes the client to end the
// goroutine.
func serveFake(t *testing.T, server io.ReadWriteCloser) <-chan error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- rfbFakeServe(context.Background(), server)

		_ = server.Close()
	}()

	return done
}

// connectAndCompleteHandshake returns a client pipe whose server end has
// already completed the RFB 3.8 handshake through ServerInit. The caller is
// positioned to read the first post-handshake message (ServerCutText).
func connectAndCompleteHandshake(t *testing.T) io.ReadWriteCloser {
	t.Helper()
	client, server := netPipe(t)
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	done := serveFake(t, server)

	// Read and echo the version string.
	gotVersion := make([]byte, len(rfbProtocolVersion))
	if _, err := io.ReadFull(client, gotVersion); err != nil {
		t.Fatalf("read ProtocolVersion: %v", err)
	}

	if _, err := client.Write([]byte(rfbProtocolVersion)); err != nil {
		t.Fatalf("write ProtocolVersion: %v", err)
	}

	// Read SecurityTypes, pick None.
	count := make([]byte, 1)
	if _, err := io.ReadFull(client, count); err != nil {
		t.Fatalf("read SecurityTypes count: %v", err)
	}

	types := make([]byte, count[0])
	if _, err := io.ReadFull(client, types); err != nil {
		t.Fatalf("read SecurityTypes: %v", err)
	}

	if _, err := client.Write([]byte{rfbSecurityNone}); err != nil {
		t.Fatalf("write security choice: %v", err)
	}

	// Read SecurityResult.
	result := make([]byte, 4)
	if _, err := io.ReadFull(client, result); err != nil {
		t.Fatalf("read SecurityResult: %v", err)
	}

	// Send ClientInit.
	if _, err := client.Write([]byte{0}); err != nil {
		t.Fatalf("write ClientInit: %v", err)
	}

	// Read ServerInit (struct) + name.
	var init rfbServerInit
	if err := binary.Read(client, binary.BigEndian, &init); err != nil {
		t.Fatalf("read ServerInit: %v", err)
	}

	name := make([]byte, init.NameLength)
	if _, err := io.ReadFull(client, name); err != nil {
		t.Fatalf("read server name: %v", err)
	}

	_ = done

	return client
}

// skipServerCutText reads and discards the single ServerCutText the fake sends
// right after ServerInit, so the caller is positioned at the first
// client-driven message exchange.
func skipServerCutText(t *testing.T, client io.ReadWriteCloser) {
	t.Helper()

	header := make([]byte, 4) // type + 3 padding
	if _, err := io.ReadFull(client, header); err != nil {
		t.Fatalf("read ServerCutText header: %v", err)
	}

	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(client, lengthBuf); err != nil {
		t.Fatalf("read ServerCutText length: %v", err)
	}

	text := make([]byte, binary.BigEndian.Uint32(lengthBuf))
	if _, err := io.ReadFull(client, text); err != nil {
		t.Fatalf("read ServerCutText text: %v", err)
	}
}
