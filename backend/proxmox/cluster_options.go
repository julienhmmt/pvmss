package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"pvmss/logger"
)

// TagColor holds the background and (optional) text color of a single tag, in
// Proxmox's 6-char hex format (no leading '#').
type TagColor struct {
	Background string `json:"background"`
	Text       string `json:"text"`
}

// clusterOptionsResponse models the fields we care about from
// GET /cluster/options.
type clusterOptionsResponse struct {
	TagStyle string `json:"tag-style"`
}

// tagStyleBuilder keeps the non-color-map pieces of a tag-style string so they
// can be preserved across a read-modify-write cycle.
type tagStyleBuilder struct {
	extras []string // "key=value" entries other than color-map, in source order
	colors map[string]TagColor
}

// GetTagColorsResty returns the color map defined in the Proxmox datacenter
// options (`tag-style.color-map`). When no color is configured or the endpoint
// fails, an empty map is returned without an error being propagated so callers
// can fall back to a deterministic color.
func GetTagColorsResty(ctx context.Context, restyClient *RestyClient) (map[string]TagColor, error) {
	if restyClient == nil {
		return map[string]TagColor{}, fmt.Errorf("resty client is nil")
	}
	raw, err := fetchRawTagStyle(ctx, restyClient)
	if err != nil {
		logger.Get().Debug().Err(err).Msg("Failed to fetch cluster options (tag colors)")
		return map[string]TagColor{}, nil
	}
	return parseTagStyle(raw), nil
}

// SetTagColorResty upserts (or removes) the color entry for a single tag by
// read-modify-writing `/cluster/options.tag-style`. Passing an empty
// background removes the entry. Text may be empty.
//
// NOTE: This performs a read-modify-write operation without locking. In the
// unlikely event of concurrent updates to different tags, the last write wins.
// This is acceptable for admin operations with low concurrency.
func SetTagColorResty(ctx context.Context, restyClient *RestyClient, tag, background, text string) error {
	if restyClient == nil {
		return fmt.Errorf("resty client is nil")
	}
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	builder, err := fetchTagStyleBuilder(ctx, restyClient)
	if err != nil {
		return err
	}
	bg := normalizeHex(background)
	txt := normalizeHex(text)
	if bg == "" {
		delete(builder.colors, tag)
	} else {
		builder.colors[tag] = TagColor{Background: bg, Text: txt}
	}
	serialized := builder.serialize()
	form := url.Values{}
	form.Set("tag-style", serialized)
	if err := restyClient.Put(ctx, "/cluster/options", form, nil); err != nil {
		logger.Get().Error().Err(err).Str("tag", tag).Msg("Failed to update tag-style")
		return fmt.Errorf("failed to update tag-style: %w", err)
	}
	return nil
}

// fetchRawTagStyle returns the raw `tag-style` string from /cluster/options.
func fetchRawTagStyle(ctx context.Context, restyClient *RestyClient) (string, error) {
	var resp Response[clusterOptionsResponse]
	if err := restyClient.Get(ctx, "/cluster/options", &resp); err != nil {
		return "", err
	}
	return resp.Data.TagStyle, nil
}

// fetchTagStyleBuilder fetches the current tag-style and splits it into a
// builder preserving extras + parsed colors.
func fetchTagStyleBuilder(ctx context.Context, restyClient *RestyClient) (*tagStyleBuilder, error) {
	raw, err := fetchRawTagStyle(ctx, restyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to read current tag-style: %w", err)
	}
	return splitTagStyle(raw), nil
}

// parseTagStyle decodes a Proxmox `tag-style` string and returns just the
// color map entries.
//
// Format:
//
//	shape=full,ordering=config,case-sensitive=0,color-map=tagA:112233:ffffff;tagB:445566
//
// Only the color-map section is interpreted. Entries are separated by `;`; each
// entry is `tagname:bg[:text]` with both colors in 6-char hex (no '#').
func parseTagStyle(tagStyle string) map[string]TagColor {
	return splitTagStyle(tagStyle).colors
}

// splitTagStyle parses a tag-style value into a tagStyleBuilder that preserves
// unknown extras so callers can serialize the value back unchanged apart from
// the color map.
func splitTagStyle(tagStyle string) *tagStyleBuilder {
	b := &tagStyleBuilder{colors: map[string]TagColor{}}
	if strings.TrimSpace(tagStyle) == "" {
		return b
	}
	for _, part := range strings.Split(tagStyle, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "color-map=") {
			rawMap := strings.TrimPrefix(trimmed, "color-map=")
			for _, entry := range strings.Split(rawMap, ";") {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				fields := strings.Split(entry, ":")
				if len(fields) < 2 {
					continue
				}
				name := strings.TrimSpace(fields[0])
				bg := strings.TrimSpace(fields[1])
				if name == "" || bg == "" {
					continue
				}
				tc := TagColor{Background: normalizeHex(bg)}
				if len(fields) >= 3 {
					tc.Text = normalizeHex(strings.TrimSpace(fields[2]))
				}
				b.colors[name] = tc
			}
			continue
		}
		b.extras = append(b.extras, trimmed)
	}
	return b
}

// serialize rebuilds a tag-style string from the preserved extras plus the
// current color map. Keys are sorted to produce stable output.
func (b *tagStyleBuilder) serialize() string {
	parts := make([]string, 0, len(b.extras)+1)
	parts = append(parts, b.extras...)
	if len(b.colors) > 0 {
		names := make([]string, 0, len(b.colors))
		for name := range b.colors {
			names = append(names, name)
		}
		sort.Strings(names)
		entries := make([]string, 0, len(names))
		for _, name := range names {
			tc := b.colors[name]
			entry := name + ":" + tc.Background
			if tc.Text != "" {
				entry += ":" + tc.Text
			}
			entries = append(entries, entry)
		}
		parts = append(parts, "color-map="+strings.Join(entries, ";"))
	}
	return strings.Join(parts, ",")
}

// normalizeHex strips a leading '#' and lowercases a hex color string.
// It does not validate the length; callers should validate before calling this.
func normalizeHex(hex string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	return strings.ToLower(hex)
}
