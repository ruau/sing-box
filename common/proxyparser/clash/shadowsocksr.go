package clash

import (
	"net"
	"strconv"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type ClashShadowsocksR struct {
	ClashProxyBasic `yaml:",inline"`
	//
	Cipher   string `yaml:"cipher"`
	Password string `yaml:"password"`
	//
	Obfs          string `yaml:"obfs"`
	ObfsParam     string `yaml:"obfs-param"`
	Protocol      string `yaml:"protocol"`
	ProtocolParam string `yaml:"protocol-param"`
	//
	TFO bool `yaml:"tfo,omitempty"`
	//
	UDP *bool `yaml:"udp"`
}

func (c *ClashShadowsocksR) Tag() string {
	if c.ClashProxyBasic.Name == "" {
		c.ClashProxyBasic.Name = net.JoinHostPort(c.ClashProxyBasic.Server, strconv.Itoa(int(c.ClashProxyBasic.ServerPort)))
	}
	return c.ClashProxyBasic.Name
}

func (c *ClashShadowsocksR) GenerateOptions() (*option.Outbound, error) {
	outboundOptions := &option.Outbound{
		Tag:  c.Tag(),
		Type: C.TypeShadowsocksR,
		ShadowsocksROptions: option.ShadowsocksROutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     c.ClashProxyBasic.Server,
				ServerPort: uint16(c.ClashProxyBasic.ServerPort),
			},
			Method:        c.Cipher,
			Password:      c.Password,
			Obfs:          c.Obfs,
			ObfsParam:     c.ObfsParam,
			Protocol:      c.Protocol,
			ProtocolParam: c.ProtocolParam,
		},
	}
	if c.UDP != nil && !*c.UDP {
		outboundOptions.ShadowsocksROptions.Network = option.NetworkList("tcp")
	}
	if c.TFO {
		outboundOptions.ShadowsocksROptions.TCPFastOpen = true
	}

	switch c.ClashProxyBasic.IPVersion {
	case "dual":
		outboundOptions.ShadowsocksROptions.DomainStrategy = 0
	case "ipv4":
		outboundOptions.ShadowsocksROptions.DomainStrategy = 3
	case "ipv6":
		outboundOptions.ShadowsocksROptions.DomainStrategy = 4
	case "ipv4-prefer":
		outboundOptions.ShadowsocksROptions.DomainStrategy = 1
	case "ipv6-prefer":
		outboundOptions.ShadowsocksROptions.DomainStrategy = 2
	}

	return outboundOptions, nil
}
