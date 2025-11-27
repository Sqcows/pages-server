package pages_server

import (
	"testing"
)

// TestNewCertificateManager tests the NewCertificateManager function.
func TestNewCertificateManager(t *testing.T) {
	cm, err := NewCertificateManager("https://acme-staging-v02.api.letsencrypt.org/directory", "admin@example.com")

	if err != nil {
		t.Fatalf("NewCertificateManager failed: %v", err)
	}

	if cm == nil {
		t.Fatal("NewCertificateManager returned nil")
	}

	if cm.endpoint != "https://acme-staging-v02.api.letsencrypt.org/directory" {
		t.Errorf("Expected endpoint to be set correctly")
	}

	if cm.email != "admin@example.com" {
		t.Errorf("Expected email to be set correctly")
	}

	if cm.certs == nil {
		t.Error("Certificate map should be initialized")
	}
}

// TestNewCertificateManagerMissingEndpoint tests NewCertificateManager with missing endpoint.
func TestNewCertificateManagerMissingEndpoint(t *testing.T) {
	_, err := NewCertificateManager("", "admin@example.com")

	if err == nil {
		t.Fatal("Expected error for missing endpoint")
	}

	if err.Error() != "ACME endpoint is required" {
		t.Errorf("Expected 'ACME endpoint is required' error, got %q", err.Error())
	}
}

// TestNewCertificateManagerMissingEmail tests NewCertificateManager with missing email.
func TestNewCertificateManagerMissingEmail(t *testing.T) {
	_, err := NewCertificateManager("https://acme-staging-v02.api.letsencrypt.org/directory", "")

	if err == nil {
		t.Fatal("Expected error for missing email")
	}

	if err.Error() != "email is required for ACME registration" {
		t.Errorf("Expected 'email is required for ACME registration' error, got %q", err.Error())
	}
}

// TestHasCertificate tests the HasCertificate method.
func TestHasCertificate(t *testing.T) {
	cm, _ := NewCertificateManager("https://acme-staging-v02.api.letsencrypt.org/directory", "admin@example.com")

	domain := "example.com"

	// Should not have certificate initially
	if cm.HasCertificate(domain) {
		t.Error("Expected HasCertificate to return false for non-existent domain")
	}

	// Store a nil certificate (placeholder for test)
	cm.StoreCertificate(domain, nil)

	// Should have certificate now
	if !cm.HasCertificate(domain) {
		t.Error("Expected HasCertificate to return true after storing certificate")
	}
}

// TestDeleteCertificate tests the DeleteCertificate method.
func TestDeleteCertificate(t *testing.T) {
	cm, _ := NewCertificateManager("https://acme-staging-v02.api.letsencrypt.org/directory", "admin@example.com")

	domain := "example.com"

	// Store a certificate
	cm.StoreCertificate(domain, nil)

	// Verify it exists
	if !cm.HasCertificate(domain) {
		t.Error("Expected certificate to exist after storing")
	}

	// Delete it
	cm.DeleteCertificate(domain)

	// Verify it's gone
	if cm.HasCertificate(domain) {
		t.Error("Expected certificate to be deleted")
	}
}

// TestListDomains tests the ListDomains method.
func TestListDomains(t *testing.T) {
	cm, _ := NewCertificateManager("https://acme-staging-v02.api.letsencrypt.org/directory", "admin@example.com")

	// Initially empty
	domains := cm.ListDomains()
	if len(domains) != 0 {
		t.Errorf("Expected 0 domains, got %d", len(domains))
	}

	// Add some certificates
	cm.StoreCertificate("example.com", nil)
	cm.StoreCertificate("test.com", nil)
	cm.StoreCertificate("demo.com", nil)

	// List should have 3 domains
	domains = cm.ListDomains()
	if len(domains) != 3 {
		t.Errorf("Expected 3 domains, got %d", len(domains))
	}

	// Verify domains are in the list
	domainMap := make(map[string]bool)
	for _, d := range domains {
		domainMap[d] = true
	}

	if !domainMap["example.com"] {
		t.Error("Expected example.com in domain list")
	}
	if !domainMap["test.com"] {
		t.Error("Expected test.com in domain list")
	}
	if !domainMap["demo.com"] {
		t.Error("Expected demo.com in domain list")
	}
}

// TestCertificateManagerConcurrency tests concurrent access to the certificate manager.
func TestCertificateManagerConcurrency(t *testing.T) {
	cm, _ := NewCertificateManager("https://acme-staging-v02.api.letsencrypt.org/directory", "admin@example.com")

	done := make(chan bool)

	// Concurrent stores
	for i := 0; i < 100; i++ {
		go func(n int) {
			domain := string(rune('a'+(n%26))) + ".example.com"
			cm.StoreCertificate(domain, nil)
			done <- true
		}(i)
	}

	// Wait for all stores
	for i := 0; i < 100; i++ {
		<-done
	}

	// Concurrent checks
	for i := 0; i < 100; i++ {
		go func(n int) {
			domain := string(rune('a'+(n%26))) + ".example.com"
			cm.HasCertificate(domain)
			done <- true
		}(i)
	}

	// Wait for all checks
	for i := 0; i < 100; i++ {
		<-done
	}
}
