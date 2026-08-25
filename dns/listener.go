package dns

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/gravitl/netclient/ncutils"
	"github.com/gravitl/netmaker/logger"

	dnscache "github.com/gravitl/netclient/dns/cache"
	dnsconfig "github.com/gravitl/netclient/dns/config"
	"github.com/miekg/dns"
	"golang.org/x/exp/slog"
)

var dnsMutex = sync.Mutex{} // used to mutex functions of the DNS

type DNSServer struct {
	DnsServer []*dns.Server
	AddrList  []string
	AddrStr   string
}

var dnsServer *DNSServer

func init() {
	dnsServer = &DNSServer{}
	cacheManager = dnscache.NewManager()
}

const AndroidDNSListenerAddr = "127.0.0.1:5300"

func Init() error {
	configManager = &dnsconfig.NoopManager{}
	return nil
}

// GetInstance
func GetDNSServerInstance() *DNSServer {
	return dnsServer
}

// Start the DNS listener on 127.0.0.1:5300
func (dnsServer *DNSServer) Start() {
	dnsMutex.Lock()
	defer dnsMutex.Unlock()
	if dnsServer.AddrStr != "" {
		return
	}

	lIp := AndroidDNSListenerAddr
	dns.HandleFunc(".", handleDNSRequest)
	srv := &dns.Server{
		Net:       "udp",
		Addr:      lIp,
		UDPSize:   65535,
		ReusePort: true,
		ReuseAddr: true,
	}

	dnsServer.AddrStr = lIp
	dnsServer.AddrList = []string{lIp}
	dnsServer.DnsServer = []*dns.Server{srv}

	go func(dnsServer *DNSServer) {
		err := srv.ListenAndServe()
		if err != nil {
			slog.Error("error in starting dns server", "error", lIp, err.Error())
			dnsServer.dropListener(lIp)
			disableDNSDNAT()
		}
	}(dnsServer)

	enableDNSDNAT()

	slog.Info("DNS server listens on: ", "Info", dnsServer.AddrList)
}

// dropListener forgets a listener that failed to bind. It removes by address
// rather than by position because the listener that failed is not necessarily
// the one appended last, and the surviving addresses are what get published to
// the resolver.
func (dnsServer *DNSServer) dropListener(addr string) {
	dnsServer.AddrList = slices.DeleteFunc(dnsServer.AddrList, func(a string) bool { return a == addr })
	dnsServer.DnsServer = slices.DeleteFunc(dnsServer.DnsServer, func(s *dns.Server) bool { return s.Addr == addr })

	dnsServer.AddrStr = ""
	if len(dnsServer.AddrList) > 0 {
		dnsServer.AddrStr = dnsServer.AddrList[0]
	}
}

// Stop the DNS listener
func (dnsServer *DNSServer) Stop() {
	dnsMutex.Lock()
	defer dnsMutex.Unlock()
	if len(dnsServer.AddrList) == 0 || len(dnsServer.DnsServer) == 0 {
		return
	}

	disableDNSDNAT()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for _, v := range dnsServer.DnsServer {
		err := v.ShutdownContext(ctx)
		if err != nil {
			logger.Log(0, "error shutting down dns server:", err.Error())
		}
	}

	dnsServer.AddrStr = ""
	dnsServer.AddrList = []string{}
	dnsServer.DnsServer = []*dns.Server{}
}

func enableDNSDNAT() {
	_, _ = ncutils.RunCmd("iptables -t nat -D OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -m owner ! --uid-owner 0 -j DNAT --to-destination 127.0.0.1:5300", false)
	_, _ = ncutils.RunCmd("iptables -t nat -A OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -m owner ! --uid-owner 0 -j DNAT --to-destination 127.0.0.1:5300", false)
	_, _ = ncutils.RunCmd("ip6tables -t nat -D OUTPUT -p udp --dport 53 ! -d ::1 -m owner ! --uid-owner 0 -j DNAT --to-destination [::1]:5300", false)
	_, _ = ncutils.RunCmd("ip6tables -t nat -A OUTPUT -p udp --dport 53 ! -d ::1 -m owner ! --uid-owner 0 -j DNAT --to-destination [::1]:5300", false)
}

func disableDNSDNAT() {
	_, _ = ncutils.RunCmd("iptables -t nat -D OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -m owner ! --uid-owner 0 -j DNAT --to-destination 127.0.0.1:5300", false)
	_, _ = ncutils.RunCmd("iptables -t nat -D OUTPUT -p udp --dport 53 -m mark ! --mark 0x10067 ! -d 127.0.0.1 -j DNAT --to-destination 127.0.0.1:5300", false)
	_, _ = ncutils.RunCmd("iptables -t nat -D OUTPUT -p udp --dport 53 ! -d 127.0.0.1 -j DNAT --to-destination 127.0.0.1:5300", false)
	_, _ = ncutils.RunCmd("ip6tables -t nat -D OUTPUT -p udp --dport 53 ! -d ::1 -m owner ! --uid-owner 0 -j DNAT --to-destination [::1]:5300", false)
	_, _ = ncutils.RunCmd("ip6tables -t nat -D OUTPUT -p udp --dport 53 -m mark ! --mark 0x10067 ! -d ::1 -j DNAT --to-destination [::1]:5300", false)
	_, _ = ncutils.RunCmd("ip6tables -t nat -D OUTPUT -p udp --dport 53 ! -d ::1 -j DNAT --to-destination [::1]:5300", false)
}
