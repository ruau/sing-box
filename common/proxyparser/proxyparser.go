package proxyparser

import (
	"github.com/sagernet/sing-box/common/proxyparser/clash"
	"github.com/sagernet/sing-box/common/proxyparser/raw"
	"github.com/sagernet/sing-box/common/proxyparser/singbox"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func ParseOutbound(content []byte) ([]option.Outbound, error) {
	var (
		outbounds        []option.Outbound
		err1, err2, err3 error
	)
	outbounds, err1 = singbox.ParseSingboxConfig(content)
	if err1 == nil {
		return outbounds, nil
	}
	outbounds, err2 = clash.ParseClashConfig(content)
	if err2 == nil {
		return outbounds, nil
	}
	outbounds, err3 = raw.ParseRawConfig(content)
	if err3 == nil {
		return outbounds, nil
	}
	return nil, E.New("parse config failed: sing-box: ", err1, " | clash: ", err2, " | raw: ", err3)
}
