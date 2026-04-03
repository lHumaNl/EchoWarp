package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

func skipIfNoNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS test in short mode")
	}
	if os.Getenv("ECHOWARP_NETWORK_TESTS") == "" {
		t.Skip("Skipping mDNS network test (set ECHOWARP_NETWORK_TESTS=1 to enable)")
	}
}

func randomServiceName() string {
	return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

func TestPublisher_Publish_ServiceVisible(t *testing.T) {
	skipIfNoNetwork(t)

	serviceName := randomServiceName()
	port := 4415
	txt := map[string]string{
		"version": "2.0.0",
		"name":    "Test Server",
	}

	publisher, err := NewPublisher(serviceName, port, txt)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisherReady := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(publisherReady)
		if err := publisher.Publish(ctx); err != nil {
			t.Errorf("Publish failed: %v", err)
		}
	}()

	select {
	case <-publisherReady:
	case <-time.After(2 * time.Second):
		t.Fatal("Publisher did not start in time")
	}

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		t.Fatalf("Failed to create resolver: %v", err)
	}

	found := false
	for attempt := 0; attempt < 3 && !found; attempt++ {
		if attempt > 0 {
			t.Logf("Retry attempt %d", attempt+1)
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				t.Fatal("Context canceled")
			}
		}

		entries := make(chan *zeroconf.ServiceEntry)
		discoverCtx, discoverCancel := context.WithTimeout(context.Background(), 3*time.Second)

		go func() {
			if err := resolver.Browse(discoverCtx, serviceType, "local.", entries); err != nil {
				t.Errorf("Browse failed: %v", err)
			}
			discoverCancel()
		}()

		for entry := range entries {
			if entry.Instance == serviceName {
				found = true
				if entry.Port != port {
					t.Errorf("Expected port %d, got %d", port, entry.Port)
				}
				break
			}
		}
	}

	if !found {
		t.Error("Service was not discovered after 3 attempts")
	}

	cancel()
	wg.Wait()
}

func TestPublisher_Publish_TXTRecords_Correct(t *testing.T) {
	skipIfNoNetwork(t)

	serviceName := randomServiceName()
	port := 4416
	txt := map[string]string{
		"version": "2.0.0",
		"name":    "Gaming PC",
		"auth":    "true",
		"tls":     "true",
		"clients": "1/5",
	}

	publisher, err := NewPublisher(serviceName, port, txt)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisherReady := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(publisherReady)
		if err := publisher.Publish(ctx); err != nil {
			t.Errorf("Publish failed: %v", err)
		}
	}()

	select {
	case <-publisherReady:
	case <-time.After(2 * time.Second):
		t.Fatal("Publisher did not start in time")
	}

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		t.Fatalf("Failed to create resolver: %v", err)
	}

	found := false
	for attempt := 0; attempt < 3 && !found; attempt++ {
		if attempt > 0 {
			t.Logf("Retry attempt %d", attempt+1)
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				t.Fatal("Context canceled")
			}
		}

		entries := make(chan *zeroconf.ServiceEntry)
		discoverCtx, discoverCancel := context.WithTimeout(context.Background(), 3*time.Second)

		go func() {
			if err := resolver.Browse(discoverCtx, serviceType, "local.", entries); err != nil {
				t.Errorf("Browse failed: %v", err)
			}
			discoverCancel()
		}()

		for entry := range entries {
			if entry.Instance == serviceName {
				found = true
				txtMap := make(map[string]string)
				for _, txtRecord := range entry.Text {
					txtMap[txtRecord] = ""
				}

				for k := range txt {
					if _, ok := txtMap[k+"="+txt[k]]; !ok {
						t.Errorf("Missing TXT record: %s=%s", k, txt[k])
					}
				}
				break
			}
		}
	}

	if !found {
		t.Error("Service was not discovered after 3 attempts")
	}

	cancel()
	wg.Wait()
}

func TestDiscoverer_Discover_FindsPublishedService(t *testing.T) {
	skipIfNoNetwork(t)

	serviceName := randomServiceName()
	port := 4417
	txt := map[string]string{
		"version": "2.0.0",
		"name":    "Test Server",
		"auth":    "true",
		"tls":     "false",
		"clients": "2/10",
	}

	publisher, err := NewPublisher(serviceName, port, txt)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}

	pubCtx, pubCancel := context.WithCancel(context.Background())
	defer pubCancel()

	publisherReady := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(publisherReady)
		if err := publisher.Publish(pubCtx); err != nil {
			t.Errorf("Publish failed: %v", err)
		}
	}()

	select {
	case <-publisherReady:
	case <-time.After(2 * time.Second):
		t.Fatal("Publisher did not start in time")
	}

	discoverer := NewDiscoverer()
	discoverCtx := context.Background()

	found := false
	for attempt := 0; attempt < 3 && !found; attempt++ {
		if attempt > 0 {
			t.Logf("Retry attempt %d", attempt+1)
			select {
			case <-time.After(500 * time.Millisecond):
			case <-pubCtx.Done():
				t.Fatal("Context canceled")
			}
		}

		results, err := discoverer.Discover(discoverCtx, 3*time.Second)
		if err != nil {
			t.Fatalf("Discover failed: %v", err)
		}

		for info := range results {
			if info.Name == serviceName {
				found = true
				if info.Port != port {
					t.Errorf("Expected port %d, got %d", port, info.Port)
				}
				if info.Version != txt["version"] {
					t.Errorf("Expected version %s, got %s", txt["version"], info.Version)
				}
				if info.AuthReq != true {
					t.Errorf("Expected AuthReq true, got %v", info.AuthReq)
				}
				if info.TLS != false {
					t.Errorf("Expected TLS false, got %v", info.TLS)
				}
				if info.Clients != txt["clients"] {
					t.Errorf("Expected clients %s, got %s", txt["clients"], info.Clients)
				}
				break
			}
		}
	}

	if !found {
		t.Error("Service was not discovered after 3 attempts")
	}

	pubCancel()
	wg.Wait()
}

func TestDiscoverer_Discover_Timeout_ReturnsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS test in short mode")
	}

	discoverer := NewDiscoverer()
	ctx := context.Background()

	results, err := discoverer.Discover(ctx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	receivedAny := false
	for range results {
		receivedAny = true
	}

	if receivedAny {
		t.Log("Warning: Received unexpected services (this is OK in a real network)")
	}
}

func TestPublisher_ContextCancel_StopsPublishing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mDNS test in short mode")
	}

	serviceName := randomServiceName()
	port := 4418
	txt := map[string]string{"version": "2.0.0"}

	publisher, err := NewPublisher(serviceName, port, txt)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	publisherStarted := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(publisherStarted)
		if err := publisher.Publish(ctx); err != nil {
			t.Errorf("Publish failed: %v", err)
		}
	}()

	select {
	case <-publisherStarted:
		cancel()
	case <-time.After(200 * time.Millisecond):
		cancel()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Publisher did not stop after context cancellation")
	}
}

func TestNewPublisher_InvalidParams(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		port        int
		txt         map[string]string
		wantErr     bool
	}{
		{
			name:        "empty name",
			serviceName: "",
			port:        4415,
			txt:         map[string]string{},
			wantErr:     true,
		},
		{
			name:        "invalid port zero",
			serviceName: "test",
			port:        0,
			txt:         map[string]string{},
			wantErr:     true,
		},
		{
			name:        "invalid port negative",
			serviceName: "test",
			port:        -1,
			txt:         map[string]string{},
			wantErr:     true,
		},
		{
			name:        "invalid port too high",
			serviceName: "test",
			port:        65536,
			txt:         map[string]string{},
			wantErr:     true,
		},
		{
			name:        "valid params",
			serviceName: "test",
			port:        4415,
			txt:         map[string]string{"version": "2.0.0"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPublisher(tt.serviceName, tt.port, tt.txt)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPublisher() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildTXTRecords(t *testing.T) {
	txt := BuildTXTRecords("2.0.0", "Gaming PC", true, false, 3, 10, "normal", "test-uuid-123")

	if txt["version"] != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", txt["version"])
	}
	if txt["name"] != "Gaming PC" {
		t.Errorf("Expected name 'Gaming PC', got %s", txt["name"])
	}
	if txt["auth"] != "true" {
		t.Errorf("Expected auth 'true', got %s", txt["auth"])
	}
	if txt["tls"] != "false" {
		t.Errorf("Expected tls 'false', got %s", txt["tls"])
	}
	if txt["clients"] != "3/10" {
		t.Errorf("Expected clients '3/10', got %s", txt["clients"])
	}
	if txt["mode"] != "normal" {
		t.Errorf("Expected mode 'normal', got %s", txt["mode"])
	}
}

func TestParseClientsField(t *testing.T) {
	tests := []struct {
		name     string
		clients  string
		wantCurr int
		wantMax  int
		wantErr  bool
	}{
		{
			name:     "valid format",
			clients:  "3/10",
			wantCurr: 3,
			wantMax:  10,
			wantErr:  false,
		},
		{
			name:     "zero clients",
			clients:  "0/5",
			wantCurr: 0,
			wantMax:  5,
			wantErr:  false,
		},
		{
			name:    "invalid format no slash",
			clients: "310",
			wantErr: true,
		},
		{
			name:    "invalid format too many slashes",
			clients: "3/10/5",
			wantErr: true,
		},
		{
			name:    "invalid current not a number",
			clients: "a/10",
			wantErr: true,
		},
		{
			name:    "invalid max not a number",
			clients: "3/b",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			curr, max, err := ParseClientsField(tt.clients)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseClientsField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if curr != tt.wantCurr {
					t.Errorf("Expected current %d, got %d", tt.wantCurr, curr)
				}
				if max != tt.wantMax {
					t.Errorf("Expected max %d, got %d", tt.wantMax, max)
				}
			}
		})
	}
}

func TestConvertEntry(t *testing.T) {
	entry := zeroconf.NewServiceEntry("Test Server", serviceType, "local.")
	entry.HostName = "test.local."
	entry.Port = 4415
	entry.AddrIPv4 = []net.IP{net.ParseIP("192.168.1.100")}
	entry.AddrIPv6 = []net.IP{net.ParseIP("::1")}
	entry.Text = []string{
		"version=2.0.0",
		"name=Test Server",
		"auth=true",
		"tls=false",
		"clients=5/10",
	}

	info := convertEntry(entry)

	if info.Name != "Test Server" {
		t.Errorf("Expected name 'Test Server', got '%s'", info.Name)
	}
	if info.Host != "test.local." {
		t.Errorf("Expected host 'test.local.', got '%s'", info.Host)
	}
	if info.Port != 4415 {
		t.Errorf("Expected port 4415, got %d", info.Port)
	}
	if info.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", info.Version)
	}
	if info.AuthReq != true {
		t.Errorf("Expected AuthReq true, got %v", info.AuthReq)
	}
	if info.TLS != false {
		t.Errorf("Expected TLS false, got %v", info.TLS)
	}
	if info.Clients != "5/10" {
		t.Errorf("Expected clients '5/10', got '%s'", info.Clients)
	}
	if len(info.AddrIPv4) != 1 || !info.AddrIPv4[0].Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("Expected IPv4 address 192.168.1.100, got %v", info.AddrIPv4)
	}
	if len(info.AddrIPv6) != 1 || !info.AddrIPv6[0].Equal(net.ParseIP("::1")) {
		t.Errorf("Expected IPv6 address ::1, got %v", info.AddrIPv6)
	}
}
