package ngrokd

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Endpoint represents a private endpoint in ngrok.
type Endpoint struct {
	ID  string
	URL *url.URL
}

// Hostname returns the hostname from the endpoint URL.
func (e Endpoint) Hostname() string {
	return e.URL.Hostname()
}

// parseAddress parses an address string into hostname and port.
func parseAddress(address string) (hostname string, port int, err error) {
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

	if idx := strings.LastIndex(address, ":"); idx != -1 {
		hostname = address[:idx]
		portStr := address[idx+1:]
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
		return hostname, port, nil
	}

	// Just hostname, default to 80
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

	// Deduplicate by URL
	seen := make(map[string]bool)
	endpoints := make([]Endpoint, 0, len(apiEndpoints))
	for _, ep := range apiEndpoints {
		if seen[ep.URL] {
			continue
		}
		seen[ep.URL] = true

		u, err := url.Parse(ep.URL)
		if err != nil {
			continue
		}

		endpoints = append(endpoints, Endpoint{
			ID:  ep.ID,
			URL: u,
		})
	}

	return endpoints, nil
}
