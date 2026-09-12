package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Record types the API supports.
const (
	RecordTypeA     = "A"
	RecordTypeAAAA  = "AAAA"
	RecordTypeCNAME = "CNAME"
	RecordTypeMX    = "MX"
	RecordTypeSRV   = "SRV"
	RecordTypeTLSA  = "TLSA"
	RecordTypeTXT   = "TXT"
)

// RecordTypes lists every supported record type, for schema validation.
var RecordTypes = []string{
	RecordTypeA, RecordTypeAAAA, RecordTypeCNAME,
	RecordTypeMX, RecordTypeSRV, RecordTypeTLSA, RecordTypeTXT,
}

// DNSRecord is the union of every record shape the API accepts. The extras are
// pointers so that `omitempty` can leave out the ones a given type does not use
// — sending priority=0 on an A record is rejected.
type DNSRecord struct {
	ID       int64  `json:"id,omitempty"`
	Host     string `json:"host"`
	TTL      *int64 `json:"ttl,omitempty"`
	Type     string `json:"type"`
	Data     string `json:"data"`
	Priority *int64 `json:"priority,omitempty"`
	Weight   *int64 `json:"weight,omitempty"`
	Port     *int64 `json:"port,omitempty"`
	Usage    *int64 `json:"usage,omitempty"`
	Selector *int64 `json:"selector,omitempty"`
	DType    *int64 `json:"dtype,omitempty"`
}

func dnsPath(domainID int64) string {
	return "/domains/" + strconv.FormatInt(domainID, 10) + "/dns"
}

func dnsRecordPath(domainID, recordID int64) string {
	return dnsPath(domainID) + "/" + strconv.FormatInt(recordID, 10)
}

// ListDNSRecords returns a domain's records, optionally narrowed by host and type.
func (c *Client) ListDNSRecords(ctx context.Context, domainID int64, host, recordType string) ([]DNSRecord, error) {
	query := url.Values{}
	if host != "" {
		query.Set("host", host)
	}
	if recordType != "" {
		query.Set("type", recordType)
	}

	var records []DNSRecord
	if err := c.do(ctx, http.MethodGet, dnsPath(domainID), query, nil, &records); err != nil {
		return nil, err
	}

	return records, nil
}

// GetDNSRecord fetches one record.
func (c *Client) GetDNSRecord(ctx context.Context, domainID, recordID int64) (*DNSRecord, error) {
	var record DNSRecord
	if err := c.do(ctx, http.MethodGet, dnsRecordPath(domainID, recordID), nil, nil, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// CreateDNSRecord creates a record and returns its new id.
func (c *Client) CreateDNSRecord(ctx context.Context, domainID int64, record DNSRecord) (int64, error) {
	// The id is read-only and lives in the URL, never the body.
	record.ID = 0

	var created struct {
		ID int64 `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, dnsPath(domainID), nil, record, &created); err != nil {
		return 0, err
	}

	return created.ID, nil
}

// UpdateDNSRecord replaces a record. The API's PUT is a full replacement, so
// record must carry every field its type requires.
func (c *Client) UpdateDNSRecord(ctx context.Context, domainID, recordID int64, record DNSRecord) error {
	record.ID = 0

	return c.do(ctx, http.MethodPut, dnsRecordPath(domainID, recordID), nil, record, nil)
}

// DeleteDNSRecord removes a record.
func (c *Client) DeleteDNSRecord(ctx context.Context, domainID, recordID int64) error {
	return c.do(ctx, http.MethodDelete, dnsRecordPath(domainID, recordID), nil, nil, nil)
}
