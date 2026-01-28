package ngrokd

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Endpoint represents a kubernetes-bound endpoint in ngrok.
// URL format: [http|tcp|tls]://name.namespace[:port]
// Hostnames must be exactly two parts separated by a dot (e.g., app.example).
type Endpoint struct {
	ID       string
	Hostname string
	Proto    string // "http", "tcp", or "tls"
	Port     int    // required for tcp/tls, optional for http (defaults to 80)
	URL      string
}

// parseAddress parses an address string into hostname and port.
// Kubernetes-bound endpoint URL format: [http|tcp|tls]://name.namespace[:port]
// Supports formats:
//   - http://app.example
//   - http://app.example:8080
//   - tcp://app.example:443
//   - tls://app.example:443
func parseAddress(address string) (hostname string, port int, err error) {
	// Check if it's a URL
	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return "", 0, fmt.Errorf("invalid URL: %w", err)
		}

		hostname = u.Hostname()
		portStr := u.Port()

		if portStr != "" {
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
				return "", 0, fmt.Errorf("invalid port: %w", err)
			}
		} else {
			// Default ports by scheme
			switch u.Scheme {
			case "http":
				port = 80
			case "tcp", "tls":
				return "", 0, fmt.Errorf("%s scheme requires explicit port", u.Scheme)
			default:
				port = 80
			}
		}
		return hostname, port, nil
	}

	// Check for host:port format
	if idx := strings.LastIndex(address, ":"); idx != -1 {
		hostname = address[:idx]
		portStr := address[idx+1:]
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
		return hostname, port, nil
	}

	// Just hostname, default to 80 (http)
	return address, 80, nil
}

// discoverEndpoints fetches bound endpoints from ngrok API
func (d *Dialer) discoverEndpoints(ctx context.Context) ([]Endpoint, error) {
	if d.operatorID == "" {
		return nil, fmt.Errorf("operator ID not set")
	}

	apiEndpoints, err := d.apiClient.ListBoundEndpoints(ctx, d.operatorID)
	if err != nil {
		return nil, err
	}

	// Deduplicate by URL (ngrok API may return stale duplicates)
	seen := make(map[string]bool)
	endpoints := make([]Endpoint, 0, len(apiEndpoints))
	for _, ep := range apiEndpoints {
		if seen[ep.URL] {
			continue
		}
		seen[ep.URL] = true
		
		hostname, port := extractHostPort(ep.URL)
		endpoints = append(endpoints, Endpoint{
			ID:       ep.ID,
			Hostname: hostname,
			Proto:    ep.Proto,
			Port:     port,
			URL:      ep.URL,
		})
	}

	return endpoints, nil
}

// extractHostPort extracts hostname and port from an endpoint URL
func extractHostPort(endpointURL string) (hostname string, port int) {
	u, err := url.Parse(endpointURL)
	if err != nil {
		return endpointURL, 443
	}

	hostname = u.Hostname()
	portStr := u.Port()

	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	} else {
		switch u.Scheme {
		case "https", "tls":
			port = 443
		case "http":
			port = 80
		default:
			port = 443
		}
	}

	return hostname, port
}
