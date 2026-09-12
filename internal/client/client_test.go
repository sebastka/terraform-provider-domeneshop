package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recorder captures what the client actually sent.
type recorder struct {
	method string
	path   string
	query  string
	body   string
	auth   string
	agent  string
}

// newTestClient spins up a server that records the request and replays a
// canned response, and returns a Client pointed at it.
func newTestClient(t *testing.T, status int, response string) (*Client, *recorder) {
	t.Helper()

	rec := &recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		rec.body = string(body)
		rec.auth = r.Header.Get("Authorization")
		rec.agent = r.Header.Get("User-Agent")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)

	return New("token", "secret", server.URL, "test-agent"), rec
}

func TestNewFallsBackToTheProductionEndpoint(t *testing.T) {
	if got := New("t", "s", "", "ua").baseURL; got != DefaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", got, DefaultBaseURL)
	}
}

func TestNewTrimsATrailingSlash(t *testing.T) {
	if got := New("t", "s", "https://example.test/v0/", "ua").baseURL; got != "https://example.test/v0" {
		t.Fatalf("baseURL = %q", got)
	}
}

func TestRequestsCarryBasicAuthAndUserAgent(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, `[]`)

	if _, err := c.ListDomains(context.Background(), ""); err != nil {
		t.Fatalf("ListDomains: %v", err)
	}

	// "token:secret" base64-encoded.
	if want := "Basic dG9rZW46c2VjcmV0"; rec.auth != want {
		t.Errorf("Authorization = %q, want %q", rec.auth, want)
	}
	if rec.agent != "test-agent" {
		t.Errorf("User-Agent = %q", rec.agent)
	}
}

func TestListDomainsSendsTheFilter(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, `[]`)

	if _, err := c.ListDomains(context.Background(), "example"); err != nil {
		t.Fatalf("ListDomains: %v", err)
	}

	if rec.path != "/domains" || rec.query != "domain=example" {
		t.Errorf("got %s?%s", rec.path, rec.query)
	}
}

func TestListDomainsOmitsAnEmptyFilter(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, `[]`)

	if _, err := c.ListDomains(context.Background(), ""); err != nil {
		t.Fatalf("ListDomains: %v", err)
	}

	if rec.query != "" {
		t.Errorf("query = %q, want empty", rec.query)
	}
}

func TestGetDomainDecodesNestedServices(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `{
		"id": 1, "domain": "example.com", "status": "active", "renew": true,
		"nameservers": ["ns1.hyp.net", "ns2.hyp.net"],
		"services": {"registrar": true, "dns": true, "email": false, "webhotel": "websmall"}
	}`)

	domain, err := c.GetDomain(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}

	if domain.Domain != "example.com" || !domain.Services.DNS || domain.Services.Webhotel != "websmall" {
		t.Errorf("unexpected domain: %+v", domain)
	}
	if len(domain.Nameservers) != 2 {
		t.Errorf("nameservers = %v", domain.Nameservers)
	}
}

func TestFindDomainByNameMatchesExactlyNotBySubstring(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `[
		{"id": 1, "domain": "myexample.com"},
		{"id": 2, "domain": "example.com"}
	]`)

	domain, err := c.FindDomainByName(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("FindDomainByName: %v", err)
	}
	if domain.ID != 2 {
		t.Errorf("id = %d, want 2", domain.ID)
	}
}

func TestFindDomainByNameReturnsNotFoundWhenOnlySubstringsMatch(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `[{"id": 1, "domain": "myexample.com"}]`)

	_, err := c.FindDomainByName(context.Background(), "example.com")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want a 404 APIError", err)
	}
}

func TestCreateDNSRecordOmitsExtrasThatDoNotApply(t *testing.T) {
	c, rec := newTestClient(t, http.StatusCreated, `{"id": 55}`)

	id, err := c.CreateDNSRecord(context.Background(), 3, DNSRecord{
		Host: "www", Type: RecordTypeA, Data: "203.0.113.10", TTL: ptr(int64(300)),
	})
	if err != nil {
		t.Fatalf("CreateDNSRecord: %v", err)
	}
	if id != 55 {
		t.Errorf("id = %d, want 55", id)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(rec.body), &sent); err != nil {
		t.Fatalf("decoding sent body: %v", err)
	}
	for _, key := range []string{"priority", "weight", "port", "usage", "selector", "dtype", "id"} {
		if _, present := sent[key]; present {
			t.Errorf("body carried %q, which does not apply to an A record: %s", key, rec.body)
		}
	}
	if sent["ttl"] != float64(300) {
		t.Errorf("ttl = %v", sent["ttl"])
	}
}

func TestCreateDNSRecordSendsMxPriority(t *testing.T) {
	c, rec := newTestClient(t, http.StatusCreated, `{"id": 1}`)

	_, err := c.CreateDNSRecord(context.Background(), 3, DNSRecord{
		Host: "@", Type: RecordTypeMX, Data: "mx.example.com", Priority: ptr(int64(10)),
	})
	if err != nil {
		t.Fatalf("CreateDNSRecord: %v", err)
	}

	if !strings.Contains(rec.body, `"priority":10`) {
		t.Errorf("body = %s", rec.body)
	}
}

func TestUpdateDNSRecordNeverSendsTheReadOnlyID(t *testing.T) {
	c, rec := newTestClient(t, http.StatusNoContent, ``)

	err := c.UpdateDNSRecord(context.Background(), 3, 9, DNSRecord{
		ID: 9, Host: "@", Type: RecordTypeA, Data: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("UpdateDNSRecord: %v", err)
	}

	if rec.method != http.MethodPut || rec.path != "/domains/3/dns/9" {
		t.Errorf("got %s %s", rec.method, rec.path)
	}
	if strings.Contains(rec.body, `"id"`) {
		t.Errorf("body carried the read-only id: %s", rec.body)
	}
}

func TestDeleteDNSRecordIssuesADelete(t *testing.T) {
	c, rec := newTestClient(t, http.StatusNoContent, ``)

	if err := c.DeleteDNSRecord(context.Background(), 3, 9); err != nil {
		t.Fatalf("DeleteDNSRecord: %v", err)
	}
	if rec.method != http.MethodDelete || rec.path != "/domains/3/dns/9" {
		t.Errorf("got %s %s", rec.method, rec.path)
	}
}

func TestForwardsListKeepsTheRequiredTrailingSlash(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, `[]`)

	if _, err := c.ListForwards(context.Background(), 5); err != nil {
		t.Fatalf("ListForwards: %v", err)
	}
	if rec.path != "/domains/5/forwards/" {
		t.Errorf("path = %q, want a trailing slash", rec.path)
	}
}

func TestForwardPathEscapesTheApexHost(t *testing.T) {
	c, rec := newTestClient(t, http.StatusOK, `{"host": "@", "url": "https://example.com"}`)

	if _, err := c.GetForward(context.Background(), 5, "@"); err != nil {
		t.Fatalf("GetForward: %v", err)
	}
	// http strips the escaping again server-side; what matters is that it arrives
	// as the apex host rather than being read as a URL delimiter.
	if rec.path != "/domains/5/forwards/@" {
		t.Errorf("path = %q", rec.path)
	}
}

func TestAPIErrorCarriesTheServersExplanation(t *testing.T) {
	c, _ := newTestClient(t, http.StatusBadRequest, `{"help": "ttl must be a multiple of 60", "code": "invalid_ttl"}`)

	_, err := c.ListDomains(context.Background(), "")

	// errors.As rather than a type assertion: the client wraps errors in places,
	// and IsNotFound relies on unwrapping working, so the test should too.
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 400 || apiErr.Help != "ttl must be a multiple of 60" || apiErr.Code != "invalid_ttl" {
		t.Errorf("unexpected error: %+v", apiErr)
	}
	if !strings.Contains(apiErr.Error(), "ttl must be a multiple of 60") {
		t.Errorf("message = %q", apiErr.Error())
	}
}

func TestAPIErrorWithoutAKnownKeyStillReads(t *testing.T) {
	c, _ := newTestClient(t, http.StatusInternalServerError, `upstream exploded`)

	_, err := c.ListDomains(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "Domeneshop API request failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestIsNotFoundOnlyMatches404(t *testing.T) {
	if IsNotFound(&APIError{StatusCode: 500}) {
		t.Error("a 500 should not count as not-found")
	}
	if !IsNotFound(&APIError{StatusCode: 404}) {
		t.Error("a 404 should count as not-found")
	}
	if IsNotFound(io.EOF) {
		t.Error("a non-API error should not count as not-found")
	}
}

func TestCredentialsNeverAppearInErrors(t *testing.T) {
	c, _ := newTestClient(t, http.StatusUnauthorized, `{"help": "unauthorized"}`)

	_, err := c.ListDomains(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token") {
		t.Errorf("error leaked credentials: %v", err)
	}
}

func ptr(v int64) *int64 { return &v }
