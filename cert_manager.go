package pages_server

import (
	"crypto/tls"
	"fmt"
	"sync"
)

// CertificateManager handles Let's Encrypt certificate management.
// Note: For Yaegi compatibility and to avoid complex ACME protocol implementation,
// this is a simplified interface. In production with Traefik, certificates are
// typically managed by Traefik's built-in ACME resolver, not by the plugin itself.
type CertificateManager struct {
	endpoint string
	email    string
	mu       sync.RWMutex
	certs    map[string]*tls.Certificate
}

// NewCertificateManager creates a new certificate manager.
func NewCertificateManager(endpoint, email string) (*CertificateManager, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("ACME endpoint is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required for ACME registration")
	}

	return &CertificateManager{
		endpoint: endpoint,
		email:    email,
		certs:    make(map[string]*tls.Certificate),
	}, nil
}

// GetCertificate retrieves or creates a certificate for the given domain.
// Note: In a Traefik environment, certificate management is typically handled
// by Traefik's ACME resolver. This method serves as a placeholder interface.
func (cm *CertificateManager) GetCertificate(domain string) (*tls.Certificate, error) {
	cm.mu.RLock()
	cert, exists := cm.certs[domain]
	cm.mu.RUnlock()

	if exists {
		return cert, nil
	}

	// In a real implementation with go-acme/lego, we would:
	// 1. Create an ACME client
	// 2. Register with Let's Encrypt
	// 3. Solve the challenge (HTTP-01 or DNS-01)
	// 4. Obtain the certificate
	// 5. Store and return it

	// For Yaegi compatibility and Traefik integration, we rely on Traefik's
	// ACME resolver to handle certificates. This is documented in the README.
	return nil, fmt.Errorf("certificate management is handled by Traefik ACME resolver")
}

// StoreCertificate stores a certificate for a domain.
func (cm *CertificateManager) StoreCertificate(domain string, cert *tls.Certificate) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.certs[domain] = cert
}

// HasCertificate checks if a certificate exists for a domain.
func (cm *CertificateManager) HasCertificate(domain string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, exists := cm.certs[domain]
	return exists
}

// DeleteCertificate removes a certificate for a domain.
func (cm *CertificateManager) DeleteCertificate(domain string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.certs, domain)
}

// ListDomains returns all domains with stored certificates.
func (cm *CertificateManager) ListDomains() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	domains := make([]string, 0, len(cm.certs))
	for domain := range cm.certs {
		domains = append(domains, domain)
	}
	return domains
}
