// Package client is a small hand-written client for the Domeneshop API.
//
// It deliberately does not depend on the PHP SDK in this monorepo — a Terraform
// provider has to be a single static binary — but it targets the same API
// version (v0) and mirrors the same models.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production Domeneshop API endpoint.
const DefaultBaseURL = "https://api.domeneshop.no/v0"

// Client talks to the Domeneshop API using HTTP Basic auth, where the API
// token is the username and the secret is the password.
type Client struct {
	baseURL   string
	token     string
	secret    string
	userAgent string
	http      *http.Client
}

// New builds a Client. An empty baseURL falls back to DefaultBaseURL.
func New(token, secret, baseURL, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		secret:    secret,
		userAgent: userAgent,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError is any non-2xx response. The credentials are never included.
type APIError struct {
	StatusCode int
	Help       string
	Code       string
	Body       string
	Path       string
}

func (e *APIError) Error() string {
	reason := e.Help
	if reason == "" {
		reason = e.Code
	}
	if reason == "" {
		reason = "Domeneshop API request failed"
	}

	return fmt.Sprintf("[%d] %s (%s)", e.StatusCode, reason, e.Path)
}

// IsNotFound reports whether err is a 404 from the API. Terraform resources use
// it to drop deleted objects from state instead of failing the plan.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}

	return false
}

// do performs a request and, when out is non-nil, decodes the JSON body into it.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body for %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("building request for %s %s: %w", method, path, err)
	}

	req.SetBasicAuth(c.token, c.secret)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Deliberately wrap without the URL: it may carry query values, and the
		// message ends up in Terraform's output.
		return fmt.Errorf("HTTP transport error on %s %s: %w", method, path, err)
	}
	// The body is fully read below; a close error at that point tells us nothing
	// actionable and would mask the real result, so it is deliberately dropped.
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp.StatusCode, raw, method+" "+path)
	}

	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decoding response from %s %s: %w", method, path, err)
	}

	return nil
}

func newAPIError(status int, raw []byte, path string) *APIError {
	apiErr := &APIError{StatusCode: status, Body: string(raw), Path: path}

	// The API is inconsistent about which key carries the explanation.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err == nil {
		for _, key := range []string{"help", "error", "message", "detail"} {
			if value, ok := decoded[key].(string); ok && value != "" {
				apiErr.Help = value
				break
			}
		}
		if value, ok := decoded["code"]; ok {
			apiErr.Code = fmt.Sprintf("%v", value)
		}
	}

	return apiErr
}

// --- Domains ---------------------------------------------------------------

// DomainServices reports which Domeneshop services a domain has.
type DomainServices struct {
	Registrar bool   `json:"registrar"`
	DNS       bool   `json:"dns"`
	Email     bool   `json:"email"`
	Webhotel  string `json:"webhotel"`
}

// Domain is a domain in the account.
type Domain struct {
	ID             int64          `json:"id"`
	Domain         string         `json:"domain"`
	ExpiryDate     string         `json:"expiry_date"`
	RegisteredDate string         `json:"registered_date"`
	Renew          bool           `json:"renew"`
	Registrant     string         `json:"registrant"`
	Status         string         `json:"status"`
	Nameservers    []string       `json:"nameservers"`
	Services       DomainServices `json:"services"`
}

// ListDomains returns the account's domains. A non-empty filter narrows the
// result to domains whose name contains it (a substring match, server-side).
func (c *Client) ListDomains(ctx context.Context, filter string) ([]Domain, error) {
	query := url.Values{}
	if filter != "" {
		query.Set("domain", filter)
	}

	var domains []Domain
	if err := c.do(ctx, http.MethodGet, "/domains", query, nil, &domains); err != nil {
		return nil, err
	}

	return domains, nil
}

// GetDomain fetches one domain by id.
func (c *Client) GetDomain(ctx context.Context, domainID int64) (*Domain, error) {
	var domain Domain
	path := "/domains/" + strconv.FormatInt(domainID, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// FindDomainByName returns the domain with exactly this name. The API only
// offers a substring filter, so this narrows server-side then matches exactly.
func (c *Client) FindDomainByName(ctx context.Context, name string) (*Domain, error) {
	domains, err := c.ListDomains(ctx, name)
	if err != nil {
		return nil, err
	}

	for i := range domains {
		if strings.EqualFold(domains[i].Domain, name) {
			return &domains[i], nil
		}
	}

	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Help:       fmt.Sprintf("no domain named %q in this account", name),
		Path:       "GET /domains",
	}
}
