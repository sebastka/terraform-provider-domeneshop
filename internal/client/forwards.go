package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// HTTPForward is a subdomain that redirects to a URL. The host is the forward's
// identity: the API addresses forwards by host and rejects (412) an update that
// tries to change it.
type HTTPForward struct {
	Host  string `json:"host"`
	Frame bool   `json:"frame"`
	URL   string `json:"url"`
}

func forwardsPath(domainID int64) string {
	// The trailing slash is required — the API 404s without it.
	return "/domains/" + strconv.FormatInt(domainID, 10) + "/forwards/"
}

func forwardPath(domainID int64, host string) string {
	// `@` (the apex) must survive as %40 rather than being read as a delimiter.
	return forwardsPath(domainID) + url.PathEscape(host)
}

// ListForwards returns a domain's HTTP forwards.
func (c *Client) ListForwards(ctx context.Context, domainID int64) ([]HTTPForward, error) {
	var forwards []HTTPForward
	if err := c.do(ctx, http.MethodGet, forwardsPath(domainID), nil, nil, &forwards); err != nil {
		return nil, err
	}

	return forwards, nil
}

// GetForward fetches the forward on host (`@` for the apex).
func (c *Client) GetForward(ctx context.Context, domainID int64, host string) (*HTTPForward, error) {
	var forward HTTPForward
	if err := c.do(ctx, http.MethodGet, forwardPath(domainID, host), nil, nil, &forward); err != nil {
		return nil, err
	}

	return &forward, nil
}

// CreateForward creates a forward. It fails with 409 when the host already has
// a forward, or an A/AAAA/ANAME/CNAME record that would collide.
func (c *Client) CreateForward(ctx context.Context, domainID int64, forward HTTPForward) error {
	return c.do(ctx, http.MethodPost, forwardsPath(domainID), nil, forward, nil)
}

// UpdateForward changes where the forward on host points. forward.Host must
// equal host; the API rejects a rename with 412.
func (c *Client) UpdateForward(ctx context.Context, domainID int64, host string, forward HTTPForward) error {
	return c.do(ctx, http.MethodPut, forwardPath(domainID, host), nil, forward, nil)
}

// DeleteForward removes the forward on host.
func (c *Client) DeleteForward(ctx context.Context, domainID int64, host string) error {
	return c.do(ctx, http.MethodDelete, forwardPath(domainID, host), nil, nil, nil)
}
