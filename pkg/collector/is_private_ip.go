package collector

import (
	"fmt"
	"net"
)

var loopbackIPBlocks []*net.IPNet
var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		privateIPBlocks = append(privateIPBlocks, parseCIDR(cidr))
	}
	for _, cidr := range []string{
		"127.0.0.0/8", // IPv4 loopback
		"::1/128",     // IPv6 loopback
	} {
		loopbackIPBlocks = append(loopbackIPBlocks, parseCIDR(cidr))
	}
}

func parseCIDR(cidr string) *net.IPNet {
	_, block, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Errorf("parse error on %q: %v", cidr, err))
	}
	return block
}

func isLoopbackIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	for _, block := range loopbackIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}
