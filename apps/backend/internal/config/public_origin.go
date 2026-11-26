package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// normalizePublicOrigin permits an HTTPS authority only. Share tokens are appended by the browser.
func normalizePublicOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	invalid := errors.New("publicOrigin must be an HTTPS origin with a reachable hostname and optional port")
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || strings.ContainsAny(value, "?#%\\") || u.Path != "" && u.Path != "/" {
		return "", invalid
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(u.Host, ":") {
		return "", invalid
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
			return "", invalid
		}
	} else {
		labels := strings.Split(host, ".")
		if len(host) > 253 || len(labels) < 2 {
			return "", invalid
		}
		for _, label := range labels {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", invalid
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
					return "", invalid
				}
			}
		}
		// Numeric final labels may be interpreted as legacy IPv4 by browsers.
		if _, err := strconv.ParseUint(labels[len(labels)-1], 0, 64); err == nil {
			return "", invalid
		}
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", invalid
		}
		if n != 443 {
			return "https://" + net.JoinHostPort(host, strconv.Itoa(n)), nil
		}
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "https://" + host, nil
}
