//go:build !linux
// +build !linux

package ncutils

func init() {
	// No-op for non-linux platforms
}

func GetSystemDNSServers() []string {
	return []string{"8.8.8.8", "1.1.1.1", "8.8.4.4"}
}

func GetHostname() string {
	name, err := os.Hostname()
	if err == nil && name != "" {
		return name
	}
	return "netclient"
}
