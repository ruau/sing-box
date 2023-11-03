package simpledns

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/miekg/dns"
)

// tcp://1.1.1.1
// tcp://1.1.1.1:53
// tcp://[2606:4700:4700::1111]
// tcp://[2606:4700:4700::1111]:53
// udp://1.1.1.1
// udp://1.1.1.1:53
// udp://[2606:4700:4700::1111]
// udp://[2606:4700:4700::1111]:53
// tls://1.1.1.1
// tls://1.1.1.1:853
// tls://[2606:4700:4700::1111]
// tls://[2606:4700:4700::1111]:853
// tls://1.1.1.1/?sni=cloudflare-dns.com
// tls://1.1.1.1:853/?sni=cloudflare-dns.com
// tls://[2606:4700:4700::1111]/?sni=cloudflare-dns.com
// tls://[2606:4700:4700::1111]:853/?sni=cloudflare-dns.com
// https://1.1.1.1
// https://1.1.1.1:443/dns-query
// https://[2606:4700:4700::1111]
// https://[2606:4700:4700::1111]:443
// https://1.1.1.1/dns-query?sni=cloudflare-dns.com
// https://1.1.1.1:443/dns-query?sni=cloudflare-dns.com
// https://[2606:4700:4700::1111]/dns-query?sni=cloudflare-dns.com
// https://[2606:4700:4700::1111]:443/dns-query?sni=cloudflare-dns.com
// 1.1.1.1 => udp://1.1.1.1:53
// 1.1.1.1:53 => udp://1.1.1.1:53
// [2606:4700:4700::1111] => udp://[2606:4700:4700::1111]:53
// [2606:4700:4700::1111]:53 => udp://[2606:4700:4700::1111]:53

func DNSLookup(ctx context.Context, dialer N.Dialer, addr string, queryDomain string, a, aaaa bool) ([]netip.Addr, error) {
	var f func(...*dns.Msg) ([][]netip.Addr, error)
	switch {
	case strings.HasPrefix(addr, "tcp://"):
		ipPort, err := netip.ParseAddrPort(addr[6:])
		if err != nil {
			ip, err := netip.ParseAddr(addr[6:])
			if err != nil {
				return nil, fmt.Errorf("invalid addr: %s", addr)
			}
			ipPort = netip.AddrPortFrom(ip, 53)
		}
		f = func(msgs ...*dns.Msg) ([][]netip.Addr, error) {
			return dnsLookupTCPOrUDPOrTLS(ctx, dialer, "tcp", nil, ipPort.String(), msgs...)
		}
	case strings.HasPrefix(addr, "udp://"):
		ipPort, err := netip.ParseAddrPort(addr[6:])
		if err != nil {
			ip, err := netip.ParseAddr(addr[6:])
			if err != nil {
				return nil, fmt.Errorf("invalid addr: %s", addr)
			}
			ipPort = netip.AddrPortFrom(ip, 53)
		}
		f = func(msgs ...*dns.Msg) ([][]netip.Addr, error) {
			return dnsLookupTCPOrUDPOrTLS(ctx, dialer, "udp", nil, ipPort.String(), msgs...)
		}
	case strings.HasPrefix(addr, "tls://"):
		u, err := url.Parse(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid addr: %s", addr)
		}
		ipPort, err := netip.ParseAddrPort(u.Host)
		if err != nil {
			ip, err := netip.ParseAddr(u.Host)
			if err != nil {
				return nil, fmt.Errorf("invalid addr: %s", addr)
			}
			ipPort = netip.AddrPortFrom(ip, 853)
		}
		tlsConfig := &tls.Config{}
		query := u.Query()
		sni := query.Get("sni")
		if sni != "" {
			tlsConfig.ServerName = sni
		} else {
			tlsConfig.ServerName = u.Hostname()
		}
		u.RawQuery = ""
		f = func(msgs ...*dns.Msg) ([][]netip.Addr, error) {
			return dnsLookupTCPOrUDPOrTLS(ctx, dialer, "tcp", tlsConfig, ipPort.String(), msgs...)
		}
	case strings.HasPrefix(addr, "https://"):
		u, err := url.Parse(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid addr: %s", addr)
		}
		ipPort, err := netip.ParseAddrPort(u.Host)
		if err != nil {
			ip, err := netip.ParseAddr(u.Host)
			if err != nil {
				return nil, fmt.Errorf("invalid addr: %s", addr)
			}
			ipPort = netip.AddrPortFrom(ip, 443)
		}
		if u.Path == "" {
			u.Path = "/dns-query"
		}
		tlsConfig := &tls.Config{}
		query := u.Query()
		sni := query.Get("sni")
		if sni != "" {
			tlsConfig.ServerName = sni
		} else {
			tlsConfig.ServerName = u.Hostname()
		}
		u.RawQuery = query.Encode()
		f = func(msgs ...*dns.Msg) ([][]netip.Addr, error) {
			return dnsLookupHTTPS(ctx, dialer, tlsConfig, ipPort.String(), u.String(), msgs...)
		}
	default:
		ipPort, err := netip.ParseAddrPort(addr)
		if err != nil {
			ip, err := netip.ParseAddr(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid addr: %s", addr)
			}
			ipPort = netip.AddrPortFrom(ip, 53)
		}
		f = func(msgs ...*dns.Msg) ([][]netip.Addr, error) {
			return dnsLookupTCPOrUDPOrTLS(ctx, dialer, "udp", nil, ipPort.String(), msgs...)
		}
	}
	var msgs []*dns.Msg
	if a {
		msg := &dns.Msg{}
		msg.SetQuestion(dns.Fqdn(queryDomain), dns.TypeA)
		msgs = append(msgs, msg)
	}
	if aaaa {
		msg := &dns.Msg{}
		msg.SetQuestion(dns.Fqdn(queryDomain), dns.TypeAAAA)
		msgs = append(msgs, msg)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no query")
	}

	ipss, err := f(msgs...)
	if err != nil {
		return nil, err
	}

	var ips []netip.Addr
	for _, s := range ipss {
		ips = append(ips, s...)
	}

	return ips, nil
}

func dnsLookupTCPOrUDPOrTLS(ctx context.Context, dialer N.Dialer, network string, tlsConfig *tls.Config, addr string, msgs ...*dns.Msg) ([][]netip.Addr, error) {
	conn, err := dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		tlsConn := tls.Client(conn, tlsConfig)
		err = tlsConn.HandshakeContext(ctx)
		if err != nil {
			return nil, err
		}
		conn = tlsConn
	}

	dnsConn := &dns.Conn{Conn: conn}
	defer dnsConn.Close()

	var ipss [][]netip.Addr

	for _, msg := range msgs {
		err = dnsConn.WriteMsg(msg)
		if err != nil {
			return nil, err
		}

		respMsg, err := dnsConn.ReadMsg()
		if err != nil {
			return nil, err
		}

		var ips []netip.Addr
		for _, answer := range respMsg.Answer {
			switch answer.Header().Rrtype {
			case dns.TypeA:
				a := answer.(*dns.A)
				ip, ok := netip.AddrFromSlice(a.A)
				if ok {
					ips = append(ips, ip)
				}
			case dns.TypeAAAA:
				a := answer.(*dns.AAAA)
				ip, ok := netip.AddrFromSlice(a.AAAA)
				if ok {
					ips = append(ips, ip)
				}
			}
		}

		ipss = append(ipss, ips)
	}

	return ipss, nil
}

func dnsLookupHTTPS(ctx context.Context, dialer N.Dialer, tlsConfig *tls.Config, addr, url string, msgs ...*dns.Msg) ([][]netip.Addr, error) {
	client := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   tlsConfig,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}

	buffer := bytes.NewBuffer(nil)

	var ipss [][]netip.Addr

	for _, msg := range msgs {
		rawMsg, err := msg.Pack()
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawMsg))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")

		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(buffer, resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}

		var respMsg dns.Msg
		err = respMsg.Unpack(buffer.Bytes())
		if err != nil {
			resp.Body.Close()
			return nil, err
		}

		resp.Body.Close()
		buffer.Reset()

		var ips []netip.Addr
		for _, answer := range respMsg.Answer {
			switch answer.Header().Rrtype {
			case dns.TypeA:
				a := answer.(*dns.A)
				ip, ok := netip.AddrFromSlice(a.A)
				if ok {
					ips = append(ips, ip)
				}
			case dns.TypeAAAA:
				a := answer.(*dns.AAAA)
				ip, ok := netip.AddrFromSlice(a.AAAA)
				if ok {
					ips = append(ips, ip)
				}
			}
		}

		ipss = append(ipss, ips)
	}

	return ipss, nil
}
