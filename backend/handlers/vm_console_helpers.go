package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"pvmss/logger"
	"pvmss/proxmox"
)

var (
	errProxmoxUnauthorized = errors.New("proxmox unauthorized")
	errProxmoxForbidden    = errors.New("proxmox forbidden")
)

type consoleAccessData struct {
	Ticket       string
	Port         int
	Host         string
	Hostname     string
	Scheme       string
	Node         string
	ConsoleURL   string
	WebsocketURL string
}

func shouldSkipProxmoxTLSVerify() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")), "false")
}

func buildConsoleAccess(ctx context.Context, apiURL, node string, vmid int, pveAuthCookie, csrfToken, username string) (*consoleAccessData, error) {
	log := logger.Get().With().
		Str("function", "buildConsoleAccess").
		Str("username", username).
		Str("node", node).
		Int("vmid", vmid).
		Str("api_url", apiURL).
		Logger()

	log.Info().Msg("Building console access for Proxmox VNC")

	if apiURL == "" {
		log.Error().Msg("Proxmox API URL is empty")
		return nil, errors.New("proxmox api url is empty")
	}

	if node == "" {
		log.Error().Msg("Proxmox node is required but not provided")
		return nil, errors.New("proxmox node is required")
	}

	if pveAuthCookie == "" {
		log.Warn().Msg("PVE auth cookie is empty - unauthorized")
		return nil, errProxmoxUnauthorized
	}

	// Mask sensitive data in logs
	maskedCookie := pveAuthCookie
	if len(maskedCookie) > 8 {
		maskedCookie = maskedCookie[:4] + "..." + maskedCookie[len(maskedCookie)-4:]
	}

	log.Info().
		Str("masked_cookie", maskedCookie).
		Bool("has_csrf_token", csrfToken != "").
		Msg("Authentication credentials available")

	log.Info().Bool("skip_tls_verify", shouldSkipProxmoxTLSVerify()).Msg("Creating Proxmox client")

	client, err := proxmox.NewClientCookieAuth(apiURL, shouldSkipProxmoxTLSVerify())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client")
		return nil, fmt.Errorf("failed to create proxmox client: %w", err)
	}

	client.PVEAuthCookie = pveAuthCookie
	client.CSRFPreventionToken = csrfToken

	log.Info().Msg("Calling Proxmox GetVNCProxy API")

	vncResp, err := client.GetVNCProxy(ctx, node, vmid)
	if err != nil {
		log.Error().
			Err(err).
			Str("proxmox_error", err.Error()).
			Msg("Proxmox GetVNCProxy API call failed")
		errMsg := err.Error()
		if strings.Contains(errMsg, "401") {
			return nil, errProxmoxUnauthorized
		}
		if strings.Contains(errMsg, "403") {
			return nil, errProxmoxForbidden
		}
		return nil, err
	}

	var vncData map[string]any
	if data, ok := vncResp["data"].(map[string]any); ok {
		vncData = data
	} else {
		vncData = vncResp
	}

	// Extract ticket from VNC data
	ticket, ok := vncData["ticket"].(string)
	if !ok || ticket == "" {
		log.Error().Interface("vnc_data", vncData).Msg("VNC proxy response missing ticket")
		return nil, errors.New("vnc proxy response missing ticket")
	}

	// Extract port from VNC data
	port, err := parsePortFromVNCData(vncData)
	if err != nil {
		log.Error().Err(err).Interface("vnc_data", vncData).Msg("Failed to parse port from VNC data")
		return nil, err
	}

	// Get additional info for logging
	cert, _ := vncData["cert"].(string)
	user, _ := vncData["user"].(string)

	log.Info().
		Str("ticket_prefix", ticket[:min(8, len(ticket))]+"...").
		Int("port", port).
		Str("cert", cert).
		Str("user", user).
		Msg("Successfully received VNC proxy response from Proxmox")

	parsedURL, err := neturl.Parse(apiURL)
	if err != nil {
		log.Error().Err(err).Str("api_url", apiURL).Msg("Failed to parse Proxmox API URL")
		return nil, fmt.Errorf("invalid proxmox api url: %w", err)
	}

	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}

	hostWithPort := parsedURL.Host
	if hostWithPort == "" {
		hostWithPort = parsedURL.Hostname()
	}

	websocketHost := hostWithPort

	// Build local noVNC URL with WebSocket connection parameters using encoded query values
	values := neturl.Values{}
	values.Set("vmid", strconv.Itoa(vmid))
	values.Set("node", node)
	values.Set("host", hostWithPort)
	values.Set("port", strconv.Itoa(port))
	values.Set("ticket", ticket)
	values.Set("scheme", scheme)

	wsScheme := "wss"
	if strings.EqualFold(scheme, "http") {
		wsScheme = "ws"
	}
	values.Set("ws_scheme", wsScheme)

	consoleURL := "/vm/console-proxy?" + values.Encode()

	websocketURL := fmt.Sprintf("%s://%s/api2/json/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=%s",
		wsScheme, websocketHost, node, vmid, port, neturl.QueryEscape(ticket),
	)

	hostname := parsedURL.Hostname()
	if hostname == "" {
		hostname = hostWithPort
	}

	return &consoleAccessData{
		Ticket:       ticket,
		Port:         port,
		Host:         hostWithPort,
		Hostname:     hostname,
		Scheme:       scheme,
		Node:         node,
		ConsoleURL:   consoleURL,
		WebsocketURL: websocketURL,
	}, nil
}

func parsePortFromVNCData(vncData map[string]any) (int, error) {
	switch v := vncData["port"].(type) {
	case float64:
		return int(v), nil
	case string:
		if v == "" {
			return 0, errors.New("vnc proxy response missing port")
		}
		port, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("invalid port value %q: %w", v, err)
		}
		return port, nil
	default:
		return 0, errors.New("vnc proxy response missing port")
	}
}

func setProxmoxAuthCookies(w http.ResponseWriter, authCookie, csrfToken, proxmoxHost string) {
	if authCookie == "" {
		return
	}

	cookieDomain := strings.TrimSpace(os.Getenv("PROXMOX_COOKIE_DOMAIN"))
	proxmoxDomain := strings.TrimSpace(proxmoxHost)
	maxAge := int((2 * time.Hour).Seconds())
	// When PROXMOX_VERIFY_SSL is explicitly disabled ("false"), we assume a dev environment without HTTPS
	// and avoid setting the Secure flag so browsers will accept the cookie.
	verifySSL := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL"))
	isSecure := !strings.EqualFold(verifySSL, "false")

	writeCookie := func(name, value, domain string) {
		if value == "" {
			return
		}
		cookie := &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			HttpOnly: false,
			Secure:   isSecure,
			SameSite: http.SameSiteNoneMode,
			MaxAge:   maxAge,
		}
		if domain != "" {
			cookie.Domain = domain
		}
		http.SetCookie(w, cookie)
	}

	// Always set cookies scoped to the current application origin (no domain) for compatibility with internal proxies.
	writeCookie("PVEAuthCookie", authCookie, "")
	writeCookie("CSRFPreventionToken", csrfToken, "")

	// Determine the domain for Proxmox host cookies.
	if cookieDomain == "" {
		cookieDomain = proxmoxDomain
	}

	// Normalize domain (strip port/IPv6 brackets) when available.
	if cookieDomain != "" {
		if host, _, err := net.SplitHostPort(cookieDomain); err == nil {
			cookieDomain = host
		} else {
			cookieDomain = strings.Trim(cookieDomain, "[]")
		}
	}

	if cookieDomain != "" {
		writeCookie("PVEAuthCookie", authCookie, cookieDomain)
		writeCookie("CSRFPreventionToken", csrfToken, cookieDomain)
	}
}
