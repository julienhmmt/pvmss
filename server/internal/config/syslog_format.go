package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// RFC 5424 shape and limits for the SIEM sink. The structured-data ID uses
// the RFC 5612 documentation PEN, the convention PegaProx and ProxCenter use.
const (
	syslogFacilityLocal0 = 16
	syslogApp            = "pvmss"
	syslogSDID           = "pvmss@32473"
	syslogMaxValue       = 512
	syslogMaxParamName   = 32
	syslogMaxMsgID       = 32
	syslogMaxHostname    = 255
	syslogMaxSD          = 4096
	syslogMaxLine        = 8192
	syslogTimeLayout     = "2006-01-02T15:04:05.000000Z07:00"
	syslogTruncatedParam = ` truncated="true"`
)

// syslogKV is one flattened, already redacted attribute.
type syslogKV struct {
	Key   string
	Value string
}

// syslogFields is everything one RFC 5424 line needs.
type syslogFields struct {
	Time     time.Time
	Level    slog.Level
	Hostname string
	PID      int
	Attrs    []syslogKV
	Message  string
}

// formatSyslogRecord renders one RFC 5424 message. The RFC 5425 octet-count
// frame is added by syslogFrame. Control characters never reach the output, so
// a value containing a newline cannot forge a second record (CWE-117).
func formatSyslogRecord(f syslogFields) string {
	head := syslogHeader(f)
	params, cut := syslogParams(f.Attrs)
	msg := replaceControl(f.Message)

	// Reserve room for the brackets, the truncated flag and the separating
	// space, so a cut message never pushes the line past its limit.
	room := syslogMaxLine - len(head) - len(syslogSDID) - 2 - len(params) - len(syslogTruncatedParam) - 1
	if len(msg) > room {
		msg, _ = cutUTF8(msg, max(room, 0))
		cut = true
	}

	sd := "-"
	if params != "" || cut {
		sd = "[" + syslogSDID + params
		if cut {
			sd += syslogTruncatedParam
		}

		sd += "]"
	}

	line := head + sd
	if msg != "" {
		line += " " + msg
	}

	return line
}

// syslogFrame applies the RFC 5425 octet-counting frame: "<len> <message>".
func syslogFrame(line string) []byte {
	return []byte(strconv.Itoa(len(line)) + " " + line)
}

func syslogHeader(f syslogFields) string {
	pri := syslogFacilityLocal0*8 + syslogSeverity(f.Level)

	msgID := "-"

	for _, kv := range f.Attrs {
		if kv.Key == "event" {
			msgID = syslogToken(kv.Value, syslogMaxMsgID)
			break
		}
	}

	return fmt.Sprintf("<%d>1 %s %s %s %d %s ",
		pri,
		f.Time.UTC().Format(syslogTimeLayout),
		syslogToken(f.Hostname, syslogMaxHostname),
		syslogApp,
		f.PID,
		msgID,
	)
}

// syslogSeverity maps a slog level to the RFC 5424 severity (debug 7, info 6,
// warn 4, error 3); levels between the named ones round down to their bucket.
func syslogSeverity(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 3
	case l >= slog.LevelWarn:
		return 4
	case l >= slog.LevelInfo:
		return 6
	default:
		return 7
	}
}

// syslogParams renders the SD-PARAMs. cut reports that a value was shortened
// or a parameter was dropped for size.
func syslogParams(attrs []syslogKV) (string, bool) {
	var b strings.Builder

	cut := false

	for _, kv := range attrs {
		name := syslogParamName(kv.Key)
		if name == "" {
			cut = true
			continue
		}

		val, valCut := sanitizeSyslogValue(kv.Value)
		esc := escapeSyslogValue(val)

		// " name=\"value\"" adds len(name) + 4 bytes around the escaped value.
		if b.Len()+len(name)+len(esc)+4 > syslogMaxSD {
			cut = true
			continue
		}

		cut = cut || valCut

		b.WriteString(" " + name + `="` + esc + `"`)
	}

	return b.String(), cut
}

// sanitizeSyslogValue strips control characters to spaces and caps the value at
// syslogMaxValue bytes, on a UTF-8 boundary.
func sanitizeSyslogValue(v string) (string, bool) {
	return cutUTF8(replaceControl(v), syslogMaxValue)
}

// escapeSyslogValue applies RFC 5424 section 6.3.3: backslash, quote and
// closing bracket get a backslash in front. These are ASCII, so a byte loop is
// UTF-8 safe.
func escapeSyslogValue(v string) string {
	var b strings.Builder

	for i := range len(v) {
		c := v[i]
		if c == '\\' || c == '"' || c == ']' {
			b.WriteByte('\\')
		}

		b.WriteByte(c)
	}

	return b.String()
}

// syslogParamName keeps the printable ASCII that an SD-NAME allows (no '=',
// space, ']' or '"'), capped at 32 bytes. Empty means the key is unusable.
func syslogParamName(k string) string {
	return printableASCII(k, syslogMaxParamName, func(c byte) bool {
		return c != '=' && c != ']' && c != '"'
	})
}

// syslogToken returns printable ASCII capped at max bytes, or "-" (the NILVALUE)
// when nothing usable is left.
func syslogToken(v string, limit int) string {
	if t := printableASCII(v, limit, func(byte) bool { return true }); t != "" {
		return t
	}

	return "-"
}

func printableASCII(s string, limit int, allowed func(byte) bool) string {
	var b strings.Builder

	for i := 0; i < len(s) && b.Len() < limit; i++ {
		if c := s[i]; c > ' ' && c < 0x7f && allowed(c) {
			b.WriteByte(c)
		}
	}

	return b.String()
}

// replaceControl maps C0 controls and DEL to spaces.
func replaceControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}

		return r
	}, s)
}

// cutUTF8 shortens s to at most n bytes without splitting a rune.
func cutUTF8(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}

	i := n
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}

	return s[:i], true
}
