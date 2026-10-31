package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
)

// parseSyslogAddr requires host:port with a port in 1-65535.
func parseSyslogAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("PVMSS_SYSLOG_ADDR must be host:port, got %q: %w", addr, err)
	}

	if host == "" {
		return "", fmt.Errorf("PVMSS_SYSLOG_ADDR has no host, got %q", addr)
	}

	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("PVMSS_SYSLOG_ADDR port must be 1-65535, got %q", port)
	}

	return host, nil
}

// syslogTLSConfig builds the receiver connection's TLS settings. Verification
// is always on: there is no skip-verify knob, and an empty CA file means the
// system roots.
func (c Configuration) syslogTLSConfig() (*tls.Config, error) {
	host, err := parseSyslogAddr(c.SyslogAddr)
	if err != nil {
		return nil, err
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}

	if c.SyslogTLSCertFile != "" {
		cert, err := tls.LoadX509KeyPair(c.SyslogTLSCertFile, c.SyslogTLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("PVMSS_SYSLOG_TLS_CERT_FILE/PVMSS_SYSLOG_TLS_KEY_FILE: %w", err)
		}

		cfg.Certificates = []tls.Certificate{cert}
	}

	if c.SyslogTLSCAFile != "" {
		pemBytes, err := os.ReadFile(c.SyslogTLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("PVMSS_SYSLOG_TLS_CA_FILE: %w", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, errors.New("PVMSS_SYSLOG_TLS_CA_FILE holds no PEM certificate")
		}

		cfg.RootCAs = pool
	}

	return cfg, nil
}

// SyslogHost returns the receiver host for the startup banner, never the port,
// and "" when the sink is off.
func (c Configuration) SyslogHost() string {
	host, err := parseSyslogAddr(c.SyslogAddr)
	if err != nil {
		return ""
	}

	return host
}

// syslogHandler sends every record to the wrapped handler first, then offers a
// formatted copy to the sink. It keeps its own attrs and group prefix, so With
// and WithGroup reach the SIEM as they reach stdout.
type syslogHandler struct {
	next  slog.Handler
	sink  *syslogSink
	attrs []syslogKV
	group []string
}

func newSyslogHandler(next slog.Handler, sink *syslogSink) slog.Handler {
	return syslogHandler{next: next, sink: sink}
}

func (h syslogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

// Handle never blocks: the stdout write comes first and the sink only gets a
// non-blocking enqueue.
func (h syslogHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.next.Handle(ctx, r)

	attrs := make([]syslogKV, 0, len(h.attrs)+r.NumAttrs()+1)
	attrs = append(attrs, syslogKV{Key: "level", Value: r.Level.String()})
	attrs = append(attrs, h.attrs...)

	r.Attrs(func(a slog.Attr) bool {
		attrs = flattenAttrs(h.group, a, attrs)

		return true
	})

	frame := syslogFrame(formatSyslogRecord(syslogFields{
		Time:     r.Time,
		Level:    r.Level,
		Hostname: h.sink.hostname,
		PID:      h.sink.pid,
		Attrs:    attrs,
		Message:  r.Message,
	}))

	if !h.sink.offer(frame) {
		h.sink.warnDrop(ctx, h.next)
	}

	return err
}

func (h syslogHandler) WithAttrs(as []slog.Attr) slog.Handler {
	out := slices.Clone(h.attrs)
	for _, a := range as {
		out = flattenAttrs(h.group, a, out)
	}

	return syslogHandler{next: h.next.WithAttrs(as), sink: h.sink, attrs: out, group: h.group}
}

func (h syslogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	return syslogHandler{
		next:  h.next.WithGroup(name),
		sink:  h.sink,
		attrs: h.attrs,
		group: append(slices.Clone(h.group), name),
	}
}

// flattenAttrs appends a's leaves as "group.key" pairs. Redaction runs on each
// leaf with the same redactSecrets rule the stdout handler uses, so the SIEM
// never sees a value that stdout would hide.
func flattenAttrs(groups []string, a slog.Attr, out []syslogKV) []syslogKV {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return out
	}

	if a.Value.Kind() == slog.KindGroup {
		sub := groups
		if a.Key != "" {
			sub = append(slices.Clone(groups), a.Key)
		}

		for _, ga := range a.Value.Group() {
			out = flattenAttrs(sub, ga, out)
		}

		return out
	}

	a = redactSecrets(groups, a)

	return append(out, syslogKV{
		Key:   strings.Join(append(slices.Clone(groups), a.Key), "."),
		Value: a.Value.String(),
	})
}

// multiCloser closes the sink (draining it) before the stdout or file writer.
type multiCloser struct {
	io.WriteCloser
	sink *syslogSink
}

func (m multiCloser) Close() error {
	m.sink.Close()

	return m.WriteCloser.Close()
}
