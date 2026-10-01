package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/telemetry"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "pvmss/cluster"

// templatedSegments maps the path segment that precedes an identifier to the
// placeholder logged in its place, so the log never carries node names, user
// ids or task ids and stays low-cardinality.
var templatedSegments = map[string]string{
	"nodes":   "{node}",
	"storage": "{storage}",
	"pools":   "{pool}",
	"tasks":   "{upid}",
	"users":   "{user}",
	"tokens":  "{token}",
	"groups":  "{group}",
	"roles":   "{role}",
}

// templateProxmoxPath replaces identifiers in a Proxmox API path with
// placeholders: /nodes/pve1/qemu/101/status -> /nodes/{node}/qemu/{vmid}/status.
func templateProxmoxPath(path string) string {
	segs := strings.Split(path, "/")

	for i := 1; i < len(segs); i++ {
		prev := segs[i-1]

		switch {
		case strings.HasPrefix(segs[i], "UPID:"):
			segs[i] = "{upid}"
		case (prev == "qemu" || prev == "lxc") && isDigits(segs[i]):
			segs[i] = "{vmid}"
		case templatedSegments[prev] != "":
			segs[i] = templatedSegments[prev]
		case isDigits(segs[i]):
			segs[i] = "{id}"
		}
	}

	return strings.Join(segs, "/")
}

func isDigits(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)

	return err == nil
}

// logger prefers the request-scoped logger (requestId, user, clientIp) and
// falls back to the registry-injected one for background calls.
func (c proxmoxRESTClient) logger(ctx context.Context) *slog.Logger {
	fallback := c.log
	if fallback == nil {
		fallback = slog.Default()
	}

	return logctx.FromOr(ctx, fallback)
}

// attempt runs one HTTP attempt and records it at Debug. It logs no error
// text on failure: the final failure belongs to the caller, and retries are
// announced separately by the loop. Bodies, headers and tokens are never read
// here.
func (c proxmoxRESTClient) attempt(ctx context.Context, method, path string, form url.Values, n int) (json.RawMessage, int, error) {
	templated := templateProxmoxPath(path)

	// A client span per attempt, named by the templated path. Set by hand
	// rather than an otelhttp transport so it can carry the cluster and never
	// records the raw path or Proxmox's response message.
	ctx, span := otel.Tracer(tracerName).Start(ctx, method+" "+templated,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("cluster", c.cluster), attribute.String("http.request.method", method),
			attribute.String("url.template", templated), attribute.Int("http.request.resend_count", n-1)))
	defer span.End()

	start := time.Now()
	raw, status, err := c.doOnce(ctx, method, path, form)

	elapsed := time.Since(start)
	telemetry.RecordProxmoxRequest(ctx, c.cluster, method, status, elapsed)
	span.SetAttributes(attribute.Int("http.response.status_code", status))

	if err != nil {
		span.SetStatus(codes.Error, retryReason(err))
	}

	c.logger(ctx).LogAttrs(ctx, slog.LevelDebug, "proxmox request",
		slog.String("component", "cluster"),
		slog.String("cluster", c.cluster),
		slog.String("method", method),
		slog.String("path", templated),
		slog.Int("status", status),
		slog.Int64("durationMs", elapsed.Milliseconds()),
		slog.Int("attempt", n),
	)

	return raw, status, err
}

func (c proxmoxRESTClient) warnRetry(ctx context.Context, method, path string, n int, err error) {
	c.logger(ctx).WarnContext(ctx, "proxmox request retrying",
		"component", "cluster", "cluster", c.cluster, "method", method, "path", templateProxmoxPath(path),
		"attempt", n, "error", retryReason(err))
}

// retryReason is a scrubbed cause for the retry line. The raw error embeds the
// full request path (node names, vmids) and, for rejections, Proxmox's response
// message, so neither is logged: a rejection becomes "HTTP <status>" and a
// transport failure keeps only the inner network error, without the URL.
func retryReason(err error) string {
	if rej, ok := errors.AsType[*RejectionError](err); ok {
		return "HTTP " + strconv.Itoa(rej.Status)
	}

	if uerr, ok := errors.AsType[*url.Error](err); ok {
		return uerr.Err.Error()
	}

	return "request failed"
}
