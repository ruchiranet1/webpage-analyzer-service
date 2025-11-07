package ssrf

import (
	"net"
)

var privateCIDRs = []*net.IPNet{
	mustParseCIDR("127.0.0.0/8"),
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
}

func mustParseCIDR(s string) *net.IPNet {
	_, cidr, _ := net.ParseCIDR(s)
	return cidr
}

func IsPublicHost(host string) (bool, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() {
			continue
		}
		for _, cidr := range privateCIDRs {
			if cidr.Contains(ip) {
				return false, nil
			}
		}
	}
	return true, nil
}
