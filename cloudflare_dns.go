package pages_server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CloudflareDNSManager manages DNS records in Cloudflare.
type CloudflareDNSManager struct {
	apiKey     string
	zoneID     string
	httpClient *http.Client
	mu         sync.RWMutex
	records    map[string]string // domain -> record ID
}

// NewCloudfllareDNSManager creates a new Cloudflare DNS manager.
func NewCloudfllareDNSManager(apiKey, zoneID string) *CloudflareDNSManager {
	return &CloudflareDNSManager{
		apiKey:  apiKey,
		zoneID:  zoneID,
		records: make(map[string]string),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DNSRecord represents a Cloudflare DNS record.
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// CloudflareResponse represents the Cloudflare API response.
type CloudflareResponse struct {
	Success bool          `json:"success"`
	Errors  []interface{} `json:"errors"`
	Result  interface{}   `json:"result"`
}

// doRequest performs an HTTP request to the Cloudflare API.
func (cdm *CloudflareDNSManager) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s%s", cdm.zoneID, path)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cdm.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cdm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// CreateDNSRecord creates a new DNS record in Cloudflare.
func (cdm *CloudflareDNSManager) CreateDNSRecord(ctx context.Context, domain, targetIP string) error {
	record := DNSRecord{
		Type:    "A",
		Name:    domain,
		Content: targetIP,
		TTL:     1,       // Auto TTL
		Proxied: true,    // Enable Cloudflare proxy
	}

	resp, err := cdm.doRequest(ctx, "POST", "/dns_records", record)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create DNS record: %d - %s", resp.StatusCode, string(body))
	}

	var cfResp CloudflareResponse
	if err := json.Unmarshal(body, &cfResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !cfResp.Success {
		return fmt.Errorf("Cloudflare API error: %v", cfResp.Errors)
	}

	// Store the record ID if we can extract it
	if resultMap, ok := cfResp.Result.(map[string]interface{}); ok {
		if id, ok := resultMap["id"].(string); ok {
			cdm.mu.Lock()
			cdm.records[domain] = id
			cdm.mu.Unlock()
		}
	}

	return nil
}

// UpdateDNSRecord updates an existing DNS record in Cloudflare.
func (cdm *CloudflareDNSManager) UpdateDNSRecord(ctx context.Context, domain, targetIP string) error {
	cdm.mu.RLock()
	recordID, exists := cdm.records[domain]
	cdm.mu.RUnlock()

	if !exists {
		// Try to find the record first
		recordID, err := cdm.findDNSRecord(ctx, domain)
		if err != nil {
			// Record doesn't exist, create it
			return cdm.CreateDNSRecord(ctx, domain, targetIP)
		}
		cdm.mu.Lock()
		cdm.records[domain] = recordID
		cdm.mu.Unlock()
	}

	record := DNSRecord{
		Type:    "A",
		Name:    domain,
		Content: targetIP,
		TTL:     1,
		Proxied: true,
	}

	path := fmt.Sprintf("/dns_records/%s", recordID)
	resp, err := cdm.doRequest(ctx, "PUT", path, record)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update DNS record: %d - %s", resp.StatusCode, string(body))
	}

	var cfResp CloudflareResponse
	if err := json.Unmarshal(body, &cfResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !cfResp.Success {
		return fmt.Errorf("Cloudflare API error: %v", cfResp.Errors)
	}

	return nil
}

// DeleteDNSRecord deletes a DNS record from Cloudflare.
func (cdm *CloudflareDNSManager) DeleteDNSRecord(ctx context.Context, domain string) error {
	cdm.mu.RLock()
	recordID, exists := cdm.records[domain]
	cdm.mu.RUnlock()

	if !exists {
		// Try to find the record first
		var err error
		recordID, err = cdm.findDNSRecord(ctx, domain)
		if err != nil {
			return err
		}
	}

	path := fmt.Sprintf("/dns_records/%s", recordID)
	resp, err := cdm.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete DNS record: %d - %s", resp.StatusCode, string(body))
	}

	cdm.mu.Lock()
	delete(cdm.records, domain)
	cdm.mu.Unlock()

	return nil
}

// findDNSRecord finds a DNS record ID by domain name.
func (cdm *CloudflareDNSManager) findDNSRecord(ctx context.Context, domain string) (string, error) {
	path := fmt.Sprintf("/dns_records?type=A&name=%s", domain)
	resp, err := cdm.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to find DNS record: %d - %s", resp.StatusCode, string(body))
	}

	var cfResp struct {
		Success bool        `json:"success"`
		Result  []DNSRecord `json:"result"`
	}

	if err := json.Unmarshal(body, &cfResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !cfResp.Success || len(cfResp.Result) == 0 {
		return "", fmt.Errorf("DNS record not found for domain: %s", domain)
	}

	return cfResp.Result[0].ID, nil
}

// HasDNSRecord checks if a DNS record exists for a domain.
func (cdm *CloudflareDNSManager) HasDNSRecord(ctx context.Context, domain string) bool {
	_, err := cdm.findDNSRecord(ctx, domain)
	return err == nil
}
