package config

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Delivery tuning for the SIEM sink. Backoff runs 1s, 2s, ... capped at 30s.
const (
	syslogQueueSize      = 1024
	syslogDialTimeout    = 5 * time.Second
	syslogWriteTimeout   = 10 * time.Second
	syslogBackoffFloor   = time.Second
	syslogBackoffCeiling = 30 * time.Second
	syslogDrainTimeout   = 2 * time.Second
	syslogWarnEvery      = time.Minute
)

// syslogSink owns the single writer goroutine and the TLS connection. Handle
// only ever enqueues without blocking. The goroutine dials lazily, on the first
// frame, so NewLogger never touches the network.
type syslogSink struct {
	addr     string
	tlsCfg   *tls.Config
	hostname string
	pid      int
	queue    chan []byte
	stop     chan struct{}
	finished chan struct{}
	stopOnce sync.Once
	dropped  atomic.Uint64
	lastWarn atomic.Int64 // unix nanos of the last drop warning
}

func newSyslogSink(addr string, tlsCfg *tls.Config) *syslogSink {
	// A failed lookup leaves the hostname empty; the formatter then writes "-".
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}

	s := &syslogSink{
		addr:     addr,
		tlsCfg:   tlsCfg,
		hostname: host,
		pid:      os.Getpid(),
		queue:    make(chan []byte, syslogQueueSize),
		stop:     make(chan struct{}),
		finished: make(chan struct{}),
	}

	go s.run()

	return s
}

// offer queues one frame without blocking. False means the frame was dropped.
func (s *syslogSink) offer(frame []byte) bool {
	select {
	case s.queue <- frame:
		return true
	default:
		s.dropped.Add(1)

		return false
	}
}

// warnDrop writes the drop warning at most once a minute. It goes straight to
// the inner (stdout) handler so a warning never re-enters the sink.
func (s *syslogSink) warnDrop(ctx context.Context, inner slog.Handler) {
	now := time.Now()
	last := s.lastWarn.Load()

	if now.UnixNano()-last < int64(syslogWarnEvery) || !s.lastWarn.CompareAndSwap(last, now.UnixNano()) {
		return
	}

	if !inner.Enabled(ctx, slog.LevelWarn) {
		return
	}

	r := slog.NewRecord(now, slog.LevelWarn, "syslog sink dropping records", 0)
	r.AddAttrs(
		slog.String("component", "config"),
		slog.String("event", "syslog_drop"),
		slog.Uint64("dropped", s.dropped.Load()),
	)

	// A stdout write failure has nowhere else to be reported.
	_ = inner.Handle(ctx, r)
}

// Close stops the writer, lets it drain for at most syslogDrainTimeout, and
// waits for it. The wait is bounded so shutdown never hangs on the network.
func (s *syslogSink) Close() {
	s.stopOnce.Do(func() { close(s.stop) })

	t := time.NewTimer(syslogDrainTimeout + time.Second)
	defer t.Stop()

	select {
	case <-s.finished:
	case <-t.C:
	}
}

func (s *syslogSink) run() {
	defer close(s.finished)

	var conn net.Conn

	backoff := syslogBackoffFloor

	for {
		select {
		case <-s.stop:
			closeConn(s.drain(conn))

			return
		case frame := <-s.queue:
			var err error

			conn, err = s.deliver(context.Background(), conn, frame)
			if err != nil {
				s.dropped.Add(1)
				s.pause(backoff)
				backoff = min(2*backoff, syslogBackoffCeiling)

				continue
			}

			backoff = syslogBackoffFloor
		}
	}
}

// drain sends what is still queued until the drain deadline passes.
func (s *syslogSink) drain(conn net.Conn) net.Conn {
	ctx, cancel := context.WithTimeout(context.Background(), syslogDrainTimeout)
	defer cancel()

	for ctx.Err() == nil {
		select {
		case frame := <-s.queue:
			var err error

			conn, err = s.deliver(ctx, conn, frame)
			if err != nil {
				s.dropped.Add(1)
			}
		default:
			return conn
		}
	}

	return conn
}

// deliver writes one frame, dialing first when there is no connection. On any
// failure the connection is closed and nil returned, so the next frame redials.
func (s *syslogSink) deliver(ctx context.Context, conn net.Conn, frame []byte) (net.Conn, error) {
	if conn == nil {
		c, err := s.dial(ctx)
		if err != nil {
			return nil, err
		}

		conn = c
	}

	if err := conn.SetWriteDeadline(writeDeadline(ctx)); err != nil {
		_ = conn.Close()

		return nil, err
	}

	if _, err := conn.Write(frame); err != nil {
		_ = conn.Close()

		return nil, err
	}

	return conn, nil
}

func (s *syslogSink) dial(ctx context.Context) (net.Conn, error) {
	d := tls.Dialer{NetDialer: &net.Dialer{Timeout: syslogDialTimeout}, Config: s.tlsCfg}

	return d.DialContext(ctx, "tcp", s.addr)
}

// pause waits out a backoff, but returns early on shutdown.
func (s *syslogSink) pause(d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
	case <-s.stop:
	}
}

func writeDeadline(ctx context.Context) time.Time {
	if d, ok := ctx.Deadline(); ok {
		return d
	}

	return time.Now().Add(syslogWriteTimeout)
}

// closeConn closes a connection at shutdown. Nothing is left to report to.
func closeConn(c net.Conn) {
	if c != nil {
		_ = c.Close()
	}
}
