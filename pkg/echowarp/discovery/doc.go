// Package discovery provides LAN discovery functionality for EchoWarp using mDNS/DNS-SD.
// It allows EchoWarp instances to find each other on the local network without
// manual IP address configuration.
//
// The Publisher interface advertises an EchoWarp service on the LAN.
// The Discoverer interface searches for EchoWarp services on the LAN.
//
// Service type: _echowarp._tcp
//
// Note: mDNS tests are flaky in CI environments. Tests are skipped by default
// and can be enabled with ECHOWARP_NETWORK_TESTS=1 environment variable.
package discovery
