package discovery

import (
	"context"
	"net"
	"time"
)

// ServiceInfo contains discovered mDNS service information.
type ServiceInfo struct {
	Name     string
	Host     string
	AddrIPv4 []net.IP
	AddrIPv6 []net.IP
	Port     int
	Version  string
	AuthReq  bool
	TLS      bool
	Clients  string
	Mode     string // "normal" or "reverse"
	ServerID string // Stable per-port UUID identifying this server across networks.
}

// Publisher defines the interface for mDNS service publishing.
type Publisher interface {
	// Publish advertises the service via mDNS until context is canceled.
	Publish(ctx context.Context) error
}

// Discoverer defines the interface for mDNS service discovery.
type Discoverer interface {
	// Discover browses for services and returns a channel of results.
	Discover(ctx context.Context, timeout time.Duration) (<-chan ServiceInfo, error)
}
