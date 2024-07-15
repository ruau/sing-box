package raw

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type ShadowsocksR struct {
	options *option.Outbound
}

func (p *ShadowsocksR) Tag() string {
	return p.options.Tag
}

func (p *ShadowsocksR) ParseLink(link string) error {
	sLink := strings.TrimPrefix(link, "ssr://")
	l, err := base64Decode(sLink)
	if err != nil {
		return fmt.Errorf("invalid ssr link: %s, err: %s", link, err)
	}
	ll := strings.SplitN(string(l), "/", 2)
	if len(ll) != 2 {
		return fmt.Errorf("invalid ssr link: %s", link)
	}
	info := strings.SplitN(ll[0], ":", 6)
	if len(info) != 6 {
		return fmt.Errorf("invalid ssr link: %s", link)
	}
	port, err := strconv.ParseUint(info[1], 10, 16)
	if err != nil {
		return fmt.Errorf("invalid ssr link: %s, err: %s", link, err)
	}
	password, err := base64Decode(info[5])
	if err != nil {
		return fmt.Errorf("invalid ssr link: %s, err: %s", link, err)
	}
	params, err := url.ParseQuery(strings.TrimPrefix(ll[1], "?"))
	if err != nil {
		return fmt.Errorf("invalid ssr link: %s, err: %s", link, err)
	}
	//
	options := &option.Outbound{
		Tag:  params.Get("remarks"),
		Type: C.TypeShadowsocksR,
		ShadowsocksROptions: option.ShadowsocksROutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     info[0],
				ServerPort: uint16(port),
			},
			Method:        info[3],
			Password:      string(password),
			Protocol:      info[2],
			Obfs:          info[4],
			ProtocolParam: params.Get("protoparam"),
			ObfsParam:     params.Get("obfsparam"),
		},
	}
	if params.Has("tfo") || params.Has("tcp-fast-open") || params.Has("tcp-fast-open") {
		options.ShadowsocksOptions.TCPFastOpen = true
	}
	if options.Tag != "" {
		options.Tag = net.JoinHostPort(options.ShadowsocksROptions.Server, strconv.Itoa(int(options.ShadowsocksROptions.ServerPort)))
	}
	p.options = options
	return nil
}

func (p *ShadowsocksR) Options() *option.Outbound {
	return p.options
}
