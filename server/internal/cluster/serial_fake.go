// Package cluster - fake serial terminal relay (TerminalRelay implementation).
//
// The fake serial relay is a minimal byte-pipe that echoes the browser's
// keystrokes back as Proxmox-style "0:len:data" frames so an xterm.js client
// sees its own input render. There is no OS underneath, so
// this is genuinely functional for the offline demo without pretending to be a
// real shell - it proves the ticket-store + relay + framing path end to end.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// serialFakeServe reads raw bytes from peer (the browser's xterm.js keystroke
// frames) and echoes each read back as a Proxmox-style "0:len:data" data frame.
// It blocks until peer closes or the context is cancelled. The browser-side
// xterm.js layer encodes keystrokes as "0:len:data"; this fake mirrors that
// framing on output so a connected terminal visibly renders typed characters.
func serialFakeServe(ctx context.Context, peer io.ReadWriteCloser) error {
	defer func() { _ = peer.Close() }()

	// A blocked Read never sees ctx: closing the peer on cancel unblocks it.
	stop := context.AfterFunc(ctx, func() { _ = peer.Close() })
	defer stop()

	buf := make([]byte, 4096)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		n, err := peer.Read(buf)
		if n > 0 {
			if werr := echoFrame(peer, buf[:n]); werr != nil {
				return werr
			}
		}

		if err != nil {
			return readError(ctx, err)
		}
	}
}

// echoFrame echoes the bytes back as a single "0:len:data" frame so xterm.js
// renders the typed characters. Written as one peer.Write so it arrives as one
// WebSocket message (NetConn maps each Write to a message); splitting the
// header and payload would deliver them as two separate frames.
func echoFrame(peer io.Writer, data []byte) error {
	frame := fmt.Sprintf("0:%d:", len(data))
	out := make([]byte, 0, len(frame)+len(data))
	out = append(out, frame...)
	out = append(out, data...)

	_, err := peer.Write(out)

	return err
}

// readError maps a peer read failure to the serve result: a cancellation wins
// over EOF and real errors so a blocked read ends cleanly.
func readError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}
