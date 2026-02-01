package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/khaar-ai/BotNet/pkg/types"
)

// DNSService handles DNS-based node discovery
type DNSService struct {
	domain   string
	nodeID   string
	manifest *types.NodeManifest
	client   *http.Client
}

// NewDNS creates a new DNS discovery service
func NewDNS(domain, nodeID string) *DNSService {
	return &DNSService{
		domain: domain,
		nodeID: nodeID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PublishNodeRecord publishes DNS TXT record for this node
func (d *DNSService) PublishNodeRecord() error {
	// For development, we'll log what would be published
	// In production, this would interface with DNS provider API
	record := fmt.Sprintf("v=1 endpoint=https://%s type=node capabilities=messaging,agent_hosting", d.nodeID)
	txtRecord := fmt.Sprintf("_botnet.%s", d.domain)
	
	log.Printf("DNS: Would publish TXT record %s = %s", txtRecord, record)
	
	// TODO: Implement actual DNS provider integration
	// Examples: AWS Route53, Cloudflare API, etc.
	
	return nil
}

// DiscoverNodes discovers peer nodes via DNS TXT records
func (d *DNSService) DiscoverNodes(domains []string) ([]*types.NodeManifest, error) {
	var nodes []*types.NodeManifest
	
	for _, domain := range domains {
		log.Printf("DNS: Discovering nodes for domain: %s", domain)
		
		// 1. Query TXT record for _botnet.<domain>
		records, err := d.queryTXT(fmt.Sprintf("_botnet.%s", domain))
		if err != nil {
			log.Printf("DNS: Failed to query TXT record for %s: %v", domain, err)
			continue
		}
		
		// 2. Parse endpoint from TXT record
		endpoint := d.parseEndpoint(records)
		if endpoint == "" {
			log.Printf("DNS: No valid endpoint found in TXT records for %s", domain)
			continue
		}
		
		// 3. Fetch /.well-known/botnet-node.json from discovered endpoint
		manifest, err := d.fetchManifest(endpoint)
		if err != nil {
			log.Printf("DNS: Failed to fetch manifest from %s: %v", endpoint, err)
			continue
		}
		
		log.Printf("DNS: Successfully discovered node: %s", manifest.NodeID)
		nodes = append(nodes, manifest)
	}
	
	log.Printf("DNS: Discovery completed. Found %d nodes", len(nodes))
	return nodes, nil
}

// queryTXT queries DNS TXT records for a given hostname
func (d *DNSService) queryTXT(hostname string) ([]string, error) {
	records, err := net.LookupTXT(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup TXT records for %s: %w", hostname, err)
	}
	
	log.Printf("DNS: Found %d TXT records for %s", len(records), hostname)
	return records, nil
}

// parseEndpoint extracts the endpoint from TXT records
func (d *DNSService) parseEndpoint(records []string) string {
	for _, record := range records {
		// Look for BotNet TXT records: "v=1 endpoint=https://... type=node ..."
		if strings.HasPrefix(record, "v=1") && strings.Contains(record, "endpoint=") {
			parts := strings.Fields(record)
			for _, part := range parts {
				if strings.HasPrefix(part, "endpoint=") {
					endpoint := strings.TrimPrefix(part, "endpoint=")
					log.Printf("DNS: Extracted endpoint: %s", endpoint)
					return endpoint
				}
			}
		}
	}
	
	return ""
}

// fetchManifest fetches the node manifest from a discovered endpoint
func (d *DNSService) fetchManifest(endpoint string) (*types.NodeManifest, error) {
	manifestURL := endpoint + "/.well-known/botnet-node.json"
	
	log.Printf("DNS: Fetching manifest from: %s", manifestURL)
	
	resp, err := d.client.Get(manifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest endpoint returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest response: %w", err)
	}
	
	var manifest types.NodeManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}
	
	// Basic validation
	if manifest.NodeID == "" || manifest.Version == "" {
		return nil, fmt.Errorf("invalid manifest: missing required fields")
	}
	
	log.Printf("DNS: Successfully fetched manifest for node: %s (version %s)", manifest.NodeID, manifest.Version)
	return &manifest, nil
}

// SetManifest sets the manifest that this node will serve
func (d *DNSService) SetManifest(manifest *types.NodeManifest) {
	d.manifest = manifest
}

// GetManifest returns the current node manifest
func (d *DNSService) GetManifest() *types.NodeManifest {
	return d.manifest
}