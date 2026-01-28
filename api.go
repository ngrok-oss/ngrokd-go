package ngrokd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultAPIURL  = "https://api.ngrok.com"
	apiVersion     = "2"
)

type apiClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func newAPIClient(apiKey string) *apiClient {
	return &apiClient{
		baseURL: defaultAPIURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type apiEndpoint struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Proto string `json:"proto"`
	Port  int    `json:"port,omitempty"`
}

type operatorCreateRequest struct {
	Description     string                 `json:"description,omitempty"`
	Metadata        string                 `json:"metadata,omitempty"`
	EnabledFeatures []string               `json:"enabled_features,omitempty"`
	Region          string                 `json:"region,omitempty"`
	Binding         *operatorBindingCreate `json:"binding,omitempty"`
}

type operatorBindingCreate struct {
	EndpointSelectors []string `json:"endpoint_selectors,omitempty"`
	CSR               string   `json:"csr,omitempty"`
}

type operatorResponse struct {
	ID      string           `json:"id"`
	Binding *operatorBinding `json:"binding,omitempty"`
}

type operatorBinding struct {
	Cert            operatorCert `json:"cert,omitempty"`
	IngressEndpoint string       `json:"ingress_endpoint,omitempty"`
}

type operatorCert struct {
	Cert      string `json:"cert"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
}

func (c *apiClient) ListBoundEndpoints(ctx context.Context, operatorID string) ([]apiEndpoint, error) {
	url := fmt.Sprintf("%s/kubernetes_operators/%s/bound_endpoints", c.baseURL, operatorID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Ngrok-Version", apiVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Endpoints []apiEndpoint `json:"endpoints"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Validate endpoints exist by checking against /endpoints API
	validEndpoints, err := c.getValidKubernetesEndpoints(ctx)
	if err != nil {
		// If validation fails, return unfiltered (best effort)
		return result.Endpoints, nil
	}

	// Filter to only include endpoints that actually exist
	filtered := make([]apiEndpoint, 0, len(result.Endpoints))
	for _, ep := range result.Endpoints {
		if validEndpoints[ep.ID] {
			filtered = append(filtered, ep)
		}
	}

	return filtered, nil
}

// getValidKubernetesEndpoints fetches all endpoints with kubernetes binding from /endpoints API
func (c *apiClient) getValidKubernetesEndpoints(ctx context.Context) (map[string]bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/endpoints", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Ngrok-Version", apiVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Endpoints []struct {
			ID       string   `json:"id"`
			Bindings []string `json:"bindings"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Build map of valid private endpoint IDs
	valid := make(map[string]bool)
	for _, ep := range result.Endpoints {
		for _, binding := range ep.Bindings {
			if binding == "kubernetes" {
				valid[ep.ID] = true
				break
			}
		}
	}

	return valid, nil
}

func (c *apiClient) CreateOperator(ctx context.Context, req *operatorCreateRequest) (*operatorResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/kubernetes_operators", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Ngrok-Version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var operator operatorResponse
	if err := json.Unmarshal(respBody, &operator); err != nil {
		return nil, err
	}

	return &operator, nil
}
