//go:build linux
// +build linux

package wireguard

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/exp/slog"
	"golang.org/x/sys/unix"
)

const (
	BypassPref = 90
	BypassFile = "/data/adb/netclient/bypass.json"
)

// BypassConfig defines the home SSIDs and subnets to bypass WireGuard when connected to home Wi-Fi.
type BypassConfig struct {
	HomeSSIDs     []string `json:"home_ssids"`
	BypassSubnets []string `json:"bypass_subnets"`
}

var bypassMu sync.Mutex

// LoadBypassConfig loads bypass configuration from disk.
func LoadBypassConfig() (*BypassConfig, error) {
	data, err := os.ReadFile(BypassFile)
	if err != nil {
		return nil, err
	}
	var cfg BypassConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveBypassConfig writes bypass configuration to disk.
func SaveBypassConfig(cfg *BypassConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(BypassFile, data, 0644)
}

// GetActiveSSID detects the currently connected Wi-Fi SSID on Android.
func GetActiveSSID() string {
	// Method 1: cmd wifi status
	if out, err := exec.Command("cmd", "wifi", "status").Output(); err == nil {
		s := string(out)
		if idx := strings.Index(s, "connected to \""); idx != -1 {
			rest := s[idx+len("connected to \""):]
			if end := strings.Index(rest, "\""); end != -1 {
				return rest[:end]
			}
		}
		if idx := strings.Index(s, "SSID: \""); idx != -1 {
			rest := s[idx+len("SSID: \""):]
			if end := strings.Index(rest, "\""); end != -1 {
				return rest[:end]
			}
		}
	}

	// Method 2: dumpsys wifi
	if out, err := exec.Command("dumpsys", "wifi").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "mWifiInfo") && strings.Contains(line, "SSID: ") {
				idx := strings.Index(line, "SSID: ")
				rest := line[idx+len("SSID: "):]
				rest = strings.Trim(strings.Split(rest, ",")[0], "\" ")
				if rest != "" && rest != "<unknown ssid>" && rest != "0x" {
					return rest
				}
			}
		}
	}

	// Method 3: iw dev wlan0 link
	if out, err := exec.Command("iw", "dev", "wlan0", "link").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "SSID: ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "SSID: "))
			}
		}
	}

	return ""
}

// EvaluateBypassRules evaluates current Wi-Fi SSID and applies or removes pref 90 bypass rules.
func EvaluateBypassRules() {
	bypassMu.Lock()
	defer bypassMu.Unlock()

	cfg, err := LoadBypassConfig()
	if err != nil || cfg == nil || len(cfg.HomeSSIDs) == 0 || len(cfg.BypassSubnets) == 0 {
		FlushBypassRules()
		return
	}

	currentSSID := GetActiveSSID()
	isHome := false
	if currentSSID != "" {
		for _, ssid := range cfg.HomeSSIDs {
			if strings.EqualFold(strings.TrimSpace(ssid), currentSSID) {
				isHome = true
				break
			}
		}
	}

	wlanLink, _ := netlink.LinkByName("wlan0")

	if isHome {
		slog.Info("connected to home Wi-Fi, applying LAN bypass rules", "ssid", currentSSID)
		for _, subnet := range cfg.BypassSubnets {
			_, ipnet, err := net.ParseCIDR(strings.TrimSpace(subnet))
			if err != nil {
				continue
			}
			rule := netlink.NewRule()
			rule.Dst = ipnet
			rule.Table = unix.RT_TABLE_MAIN
			rule.Priority = BypassPref
			if wlanLink != nil {
				rule.OifName = "wlan0"
			}
			_ = netlink.RuleDel(rule)
			if err := netlink.RuleAdd(rule); err != nil && !os.IsExist(err) {
				slog.Warn("failed to add bypass rule", "subnet", subnet, "error", err)
			}
		}
	} else {
		FlushBypassRules()
	}
}

// FlushBypassRules flushes all pref 90 bypass rules.
func FlushBypassRules() {
	rules, err := netlink.RuleList(netlink.FAMILY_ALL)
	if err == nil {
		for _, rule := range rules {
			if rule.Priority == BypassPref {
				_ = netlink.RuleDel(&rule)
			}
		}
	}
}

// StartBypassMonitor starts an event-driven Netlink listener for network state changes (0% idle CPU).
func StartBypassMonitor(ctx context.Context) {
	// Initial evaluation at startup
	EvaluateBypassRules()

	linkCh := make(chan netlink.LinkUpdate)
	linkDone := make(chan struct{})
	if err := netlink.LinkSubscribe(linkCh, linkDone); err != nil {
		slog.Warn("failed to subscribe to netlink link updates", "error", err)
		return
	}
	defer close(linkDone)

	addrCh := make(chan netlink.AddrUpdate)
	addrDone := make(chan struct{})
	if err := netlink.AddrSubscribe(addrCh, addrDone); err != nil {
		slog.Warn("failed to subscribe to netlink addr updates", "error", err)
		return
	}
	defer close(addrDone)

	slog.Info("started event-driven Wi-Fi bypass monitor")

	for {
		select {
		case <-ctx.Done():
			FlushBypassRules()
			return
		case linkUpdate := <-linkCh:
			if linkUpdate.Link != nil && linkUpdate.Link.Attrs() != nil && strings.HasPrefix(linkUpdate.Link.Attrs().Name, "wlan") {
				EvaluateBypassRules()
			}
		case addrUpdate := <-addrCh:
			if link, err := netlink.LinkByIndex(addrUpdate.LinkIndex); err == nil && link != nil && strings.HasPrefix(link.Attrs().Name, "wlan") {
				EvaluateBypassRules()
			}
		}
	}
}
