package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

var syslogTestTime = time.Date(2026, 10, 30, 10, 0, 0, 123456000, time.UTC)

func TestFormatSyslogRecord_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		level  slog.Level
		attrs  []syslogKV
		msg    string
		want   string // exact line, when set
		prefix string // checked with HasPrefix, when set
		has    []string
	}{
		{name: "debug PRI", level: slog.LevelDebug, prefix: "<135>1 "},
		{name: "info PRI", level: slog.LevelInfo, prefix: "<134>1 "},
		{name: "warn PRI", level: slog.LevelWarn, prefix: "<132>1 "},
		{name: "error PRI", level: slog.LevelError, prefix: "<131>1 "},
		{
			name:  "no attributes gives NILVALUE SD and no trailing space",
			level: slog.LevelInfo,
			msg:   "m",
			want:  "<134>1 2026-10-30T10:00:00.123456Z h pvmss 7 - - m",
		},
		{
			name:  "event becomes MSGID",
			level: slog.LevelInfo,
			attrs: []syslogKV{{"event", "audit"}, {"user", "alice"}},
			has:   []string{" pvmss 7 audit [pvmss@32473 event=\"audit\" user=\"alice\"]"},
		},
		{
			name:  "no event gives MSGID dash",
			level: slog.LevelInfo,
			attrs: []syslogKV{{"user", "alice"}},
			has:   []string{" pvmss 7 - [pvmss@32473 user=\"alice\"]"},
		},
		{
			name:  "value escaping of quote backslash and bracket",
			level: slog.LevelInfo,
			attrs: []syslogKV{{"k", `a"b]c\d`}},
			has:   []string{`k="a\"b\]c\\d"`},
		},
		{
			name:  "control characters become spaces",
			level: slog.LevelInfo,
			attrs: []syslogKV{{"k", "a\nb\x1bc"}},
			has:   []string{`k="a b c"`},
		},
		{
			name:  "value cut at 512 bytes and flagged",
			level: slog.LevelInfo,
			attrs: []syslogKV{{"k", strings.Repeat("a", 2000)}},
			has:   []string{`k="` + strings.Repeat("a", 512) + `" truncated="true"]`},
		},
		{
			name:  "message with newline stays one line",
			level: slog.LevelInfo,
			msg:   "first\nsecond\r\nthird",
			has:   []string{"first second  third"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			line := formatSyslogRecord(syslogFields{
				Time: syslogTestTime, Level: c.level, Hostname: "h", PID: 7,
				Attrs: c.attrs, Message: c.msg,
			})

			if strings.ContainsAny(line, "\n\r") {
				t.Fatalf("line contains a raw newline: %q", line)
			}

			if c.want != "" && line != c.want {
				t.Fatalf("line = %q, want %q", line, c.want)
			}

			if c.prefix != "" && !strings.HasPrefix(line, c.prefix) {
				t.Fatalf("line = %q, want prefix %q", line, c.prefix)
			}

			for _, sub := range c.has {
				if !strings.Contains(line, sub) {
					t.Fatalf("line = %q, want it to contain %q", line, sub)
				}
			}
		})
	}
}

func TestFormatSyslogRecord_LineLimit(t *testing.T) {
	t.Parallel()

	line := formatSyslogRecord(syslogFields{
		Time: syslogTestTime, Level: slog.LevelInfo, Hostname: "h", PID: 7,
		Attrs:   []syslogKV{{"k", "v"}},
		Message: strings.Repeat("m", 20000),
	})

	if len(line) > syslogMaxLine {
		t.Fatalf("line length = %d, want <= %d", len(line), syslogMaxLine)
	}

	if !strings.Contains(line, `truncated="true"`) {
		t.Fatalf("cut message not flagged: %.120q", line)
	}
}

func TestSyslogFrame_OctetCount(t *testing.T) {
	t.Parallel()

	if got := string(syslogFrame("abc")); got != "3 abc" {
		t.Fatalf("frame = %q, want %q", got, "3 abc")
	}
}

func TestSyslogParamName_Sanitizes(t *testing.T) {
	t.Parallel()

	if got := syslogParamName(`a=b c]"d`); got != "abcd" {
		t.Fatalf("name = %q, want %q", got, "abcd")
	}

	if got := syslogParamName(strings.Repeat("x", 40)); len(got) != syslogMaxParamName {
		t.Fatalf("name length = %d, want %d", len(got), syslogMaxParamName)
	}
}
