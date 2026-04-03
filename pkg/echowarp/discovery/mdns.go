// Package discovery provides mDNS service discovery and publishing for EchoWarp.
// It uses the grandcat/zeroconf library for multicast DNS operations.
package discovery

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_echowarp._tcp"

// MDNSPublisher implements Publisher using grandcat/zeroconf.
type MDNSPublisher struct {
	name string
	port int
	txt  map[string]string
}

// NewPublisher creates a new mDNS publisher for the EchoWarp service.
// name: service instance name; port: listening port; txt: TXT records.
func NewPublisher(name string, port int, txt map[string]string) (*MDNSPublisher, error) {
	if name == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port number: %d", port)
	}
	return &MDNSPublisher{
		name: name,
		port: port,
		txt:  txt,
	}, nil
}

// Publish advertises the service via mDNS. Blocks until context is canceled.
func (p *MDNSPublisher) Publish(ctx context.Context) error {
	txtRecords := make([]string, 0, len(p.txt))
	for k, v := range p.txt {
		txtRecords = append(txtRecords, fmt.Sprintf("%s=%s", k, v))
	}

	server, err := zeroconf.Register(
		p.name,
		serviceType,
		"local.",
		p.port,
		txtRecords,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register mDNS service: %w", err)
	}
	defer server.Shutdown()

	<-ctx.Done()
	return nil
}

// MDNSDiscoverer implements Discoverer using grandcat/zeroconf.
type MDNSDiscoverer struct{}

// NewDiscoverer creates a new mDNS service discoverer.
func NewDiscoverer() *MDNSDiscoverer {
	return &MDNSDiscoverer{}
}

// Discover browses for EchoWarp services and returns results via channel.
// The discovery runs for the specified timeout or until context is canceled.
func (d *MDNSDiscoverer) Discover(ctx context.Context, timeout time.Duration) (<-chan ServiceInfo, error) {
	results := make(chan ServiceInfo, 16)
	discoverCtx, cancel := context.WithTimeout(ctx, timeout)

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		cancel()
		close(results)
		return results, fmt.Errorf("failed to create mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 16)

	if err := resolver.Browse(discoverCtx, serviceType, "local.", entries); err != nil {
		cancel()
		close(results)
		return results, fmt.Errorf("failed to browse mDNS services: %w", err)
	}

	go func() {
		defer close(results)
		defer cancel()

		for entry := range entries {
			info := convertEntry(entry)
			select {
			case results <- info:
			case <-discoverCtx.Done():
				for range entries { //nolint:revive // drain channel before return
				}
				return
			}
		}
	}()

	return results, nil
}

// convertEntry transforms a zeroconf.ServiceEntry into ServiceInfo.
func convertEntry(entry *zeroconf.ServiceEntry) ServiceInfo {
	info := ServiceInfo{
		Name:     entry.Instance,
		Host:     entry.HostName,
		Port:     entry.Port,
		AddrIPv4: entry.AddrIPv4,
		AddrIPv6: entry.AddrIPv6,
	}

	for _, txt := range entry.Text {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "version":
			info.Version = value
		case "name":
			if value != "" {
				info.Name = value
			}
		case "auth":
			info.AuthReq = value == "true"
		case "tls":
			info.TLS = value == "true"
		case "clients":
			info.Clients = value
		case "mode":
			info.Mode = value
		case "server_id":
			info.ServerID = value
		}
	}

	return info
}

// BuildTXTRecords creates TXT records for mDNS advertisement.
func BuildTXTRecords(version, name string, authReq, tls bool, currentClients, maxClients int, mode, serverID string) map[string]string {
	m := map[string]string{
		"version": version,
		"name":    name,
		"auth":    strconv.FormatBool(authReq),
		"tls":     strconv.FormatBool(tls),
		"clients": fmt.Sprintf("%d/%d", currentClients, maxClients),
		"mode":    mode,
	}
	if serverID != "" {
		m["server_id"] = serverID
	}
	return m
}

// ParseClientsField parses the "current/max" clients field from TXT records.
func ParseClientsField(clients string) (current, maxVal int, err error) {
	parts := strings.Split(clients, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid clients format: %s", clients)
	}

	current, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid current clients count: %w", err)
	}

	maxVal, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid max clients count: %w", err)
	}

	return current, maxVal, nil
}
