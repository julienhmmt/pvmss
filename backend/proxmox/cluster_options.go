package proxmox

import (
	"context"
	"fmt"
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

// GetTagColorsResty returns the color map defined in the Proxmox datacenter
// options (`tag-style.color-map`). When no color is configured or the endpoint
// fails, an empty map is returned without an error being propagated so callers
// can fall back to a deterministic color.
func GetTagColorsResty(ctx context.Context, restyClient *RestyClient) (map[string]TagColor, error) {
	if restyClient == nil {
		return map[string]TagColor{}, fmt.Errorf("resty client is nil")
	}
	var resp Response[clusterOptionsResponse]
	if err := restyClient.Get(ctx, "/cluster/options", &resp); err != nil {
		logger.Get().Debug().Err(err).Msg("Failed to fetch cluster options (tag colors)")
		return map[string]TagColor{}, nil
	}
	return parseTagStyle(resp.Data.TagStyle), nil
}

// parseTagStyle decodes a Proxmox `tag-style` string.
//
// Format:
//
//	shape=full,ordering=config,case-sensitive=0,color-map=tagA:112233:ffffff;tagB:445566
//
// Only the color-map section is interpreted. Entries are separated by `;`; each
// entry is `tagname:bg[:text]` with both colors in 6-char hex (no '#').
func parseTagStyle(tagStyle string) map[string]TagColor {
	colors := make(map[string]TagColor)
	if tagStyle == "" {
		return colors
	}
	for _, part := range strings.Split(tagStyle, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "color-map=") {
			continue
		}
		rawMap := strings.TrimPrefix(part, "color-map=")
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
			colors[name] = tc
		}
	}
	return colors
}

// normalizeHex strips a leading '#' and lowercases a hex color string.
func normalizeHex(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	return strings.ToLower(hex)
}
