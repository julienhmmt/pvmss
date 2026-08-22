package cluster

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func newPipePeers() (local, remote net.Conn) {
	local, remote = net.Pipe()
	return local, remote
}

func TestSerialFakeServe_EchoesInputAsFramedData(t *testing.T) {
	t.Parallel()

	local, remote := newPipePeers()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() { done <- serialFakeServe(ctx, local) }()

	input := "hello"
	if _, err := remote.Write([]byte(input)); err != nil {
		t.Fatalf("remote write: %v", err)
	}

	buf := make([]byte, 64)
	remote.SetReadDeadline(time.Now().Add(time.Second))
	n, err := remote.Read(buf)
	if err != nil {
		t.Fatalf("remote read: %v", err)
	}
	remote.SetReadDeadline(time.Time{})

	want := "0:5:hello"
	if got := string(buf[:n]); got != want {
		t.Fatalf("echo = %q, want %q", got, want)
	}

	if err := remote.Close(); err != nil {
		t.Fatalf("remote close: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serialFakeServe returned %v, want nil on peer close", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serialFakeServe did not return after peer closed")
	}
}

func TestSerialFakeServe_MultipleReadsEchoEachAsFrame(t *testing.T) {
	t.Parallel()

	local, remote := newPipePeers()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() { done <- serialFakeServe(ctx, local) }()

	chunks := []string{"ab", "cde", "f"}
	var got strings.Builder

	for _, chunk := range chunks {
		if _, err := remote.Write([]byte(chunk)); err != nil {
			t.Fatalf("remote write %q: %v", chunk, err)
		}

		remote.SetReadDeadline(time.Now().Add(time.Second))
		frameBuf := make([]byte, 64)
		n, err := remote.Read(frameBuf)
		if err != nil {
			t.Fatalf("remote read after %q: %v", chunk, err)
		}
		remote.SetReadDeadline(time.Time{})
		got.Write(frameBuf[:n])
	}

	want := "0:2:ab0:3:cde0:1:f"
	if got.String() != want {
		t.Fatalf("frames = %q, want %q", got.String(), want)
	}

	if err := remote.Close(); err != nil {
		t.Fatalf("remote close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serialFakeServe did not return after peer closed")
	}
}

func TestSerialFakeServe_ReturnsContextErrOnCancel(t *testing.T) {
	t.Parallel()

	local, remote := newPipePeers()
	t.Cleanup(func() { _ = remote.Close() })

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- serialFakeServe(ctx, local) }()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, ctx.Err()) {
			t.Fatalf("err = %v, want %v", err, ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("serialFakeServe did not return after context cancel")
	}
}

func TestSerialFakeServe_ClosesPeerOnExit(t *testing.T) {
	t.Parallel()

	local, remote := newPipePeers()
	t.Cleanup(func() { _ = remote.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() { done <- serialFakeServe(ctx, local) }()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serialFakeServe did not return after context cancel")
	}

	if _, err := local.Write([]byte("x")); err == nil {
		t.Fatal("local peer was not closed after serialFakeServe exit")
	}
}

func TestSerialFakeServe_BufferPeerEchoesAllData(t *testing.T) {
	t.Parallel()

	local := newBufferPeer([]byte("xy"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := serialFakeServe(ctx, local)
	if err != nil {
		t.Fatalf("serialFakeServe: %v", err)
	}

	want := "0:2:xy"
	if got := local.written.String(); got != want {
		t.Fatalf("written = %q, want %q", got, want)
	}

	if !local.closed {
		t.Fatal("buffer peer was not closed after serialFakeServe exit")
	}
}

type bufferPeer struct {
	src     *bytes.Reader
	written bytes.Buffer
	closed  bool
}

func newBufferPeer(data []byte) *bufferPeer {
	return &bufferPeer{src: bytes.NewReader(data)}
}

func (b *bufferPeer) Read(p []byte) (int, error) {
	n, err := b.src.Read(p)
	if err == io.EOF && n == 0 {
		return 0, io.EOF
	}
	return n, err
}

func (b *bufferPeer) Write(p []byte) (int, error) {
	return b.written.Write(p)
}

func (b *bufferPeer) Close() error {
	b.closed = true
	return nil
}
