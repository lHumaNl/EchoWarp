package views

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

func TestApplyDiscoveredServer(t *testing.T) {
	fields := []SetupField{
		NewTextField("server_address", "Server address", "", true),
		NewNumberField("port", "Port", 4415, 1, 65535),
		NewPasswordField("password", "Password", ""),
	}
	advFields := []SetupField{
		NewToggleField("mode", "Mode", []string{"normal (server → client)", "reverse (client → server)"}, 0),
		NewToggleField("tls", "TLS", []string{"off", "on", "insecure"}, 0),
	}

	server := discovery.ServiceInfo{
		Name:     "TestServer",
		Port:     8080,
		AddrIPv4: []net.IP{net.ParseIP("192.168.1.50")},
		TLS:      true,
		AuthReq:  true,
		Mode:     "reverse",
	}

	fields, advFields = ApplyDiscoveredServer(fields, advFields, server)

	assert.Equal(t, "192.168.1.50", fields[0].Value)
	assert.Equal(t, SourceAuto, fields[0].Source)
	assert.Equal(t, "8080", fields[1].Value)
	assert.Equal(t, SourceAuto, fields[1].Source)
	assert.Equal(t, "(required)", fields[2].Hint)
	assert.Equal(t, "reverse (client → server)", advFields[0].Value)
	assert.Equal(t, "on", advFields[1].Value)
}

func TestDiscoveryAddressHint(t *testing.T) {
	assert.Contains(t, DiscoveryAddressHint(true, 0, ""), "scanning")
	assert.Contains(t, DiscoveryAddressHint(false, 1, "MyPC"), "discovered: MyPC")
	assert.Contains(t, DiscoveryAddressHint(false, 3, ""), "choose from 3")
	assert.Equal(t, "", DiscoveryAddressHint(false, 0, ""))
}
