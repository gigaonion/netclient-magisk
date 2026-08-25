//go:build linux
// +build linux

package ncutils

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func init() {
	initCustomResolver()
}

func initCustomResolver() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 4,
			}
			target := address
			if strings.HasPrefix(address, "127.0.0.1:53") || strings.HasPrefix(address, "[::1]:53") || strings.HasPrefix(address, "localhost:53") || !strings.Contains(address, ":") {
				target = getSystemDNSTarget()
			}
			conn, err := d.DialContext(ctx, network, target)
			if err != nil {
				conn, err = d.DialContext(ctx, network, getSystemDNSTarget())
			}
			return conn, err
		},
	}
}

func getSystemDNSTarget() string {
	servers := GetSystemDNSServers()
	if len(servers) > 0 {
		return net.JoinHostPort(servers[0], "53")
	}
	return "8.8.8.8:53"
}

// GetSystemDNSServers returns active Android DNS servers (e.g. net.dns1) followed by public fallback DNS.
func GetSystemDNSServers() []string {
	var servers []string
	for _, prop := range []string{"net.dns1", "net.dns2", "net.dns3"} {
		if out, err := exec.Command("getprop", prop).Output(); err == nil {
			dnsIP := strings.TrimSpace(string(out))
			if dnsIP != "" && net.ParseIP(dnsIP) != nil && dnsIP != "127.0.0.1" && dnsIP != "::1" {
				servers = append(servers, dnsIP)
			}
		}
	}
	// Fallback to Google / Cloudflare public DNS
	servers = append(servers, "8.8.8.8", "1.1.1.1", "8.8.4.4")
	return servers
}

// GetHostname returns the host name, falling back to Android device model properties if hostname is "localhost".
func GetHostname() string {
	name, _ := os.Hostname()
	if name != "" && name != "localhost" && name != "localhost.localdomain" {
		return name
	}
	for _, prop := range []string{"net.hostname", "ro.product.model", "ro.product.device", "ro.product.name"} {
		if out, err := exec.Command("getprop", prop).Output(); err == nil {
			val := strings.TrimSpace(string(out))
			if val != "" {
				val = strings.ReplaceAll(val, " ", "-")
				return val
			}
		}
	}
	if name != "" {
		return name
	}
	return "android-client"
}
