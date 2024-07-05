package action

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/outboundprovider/filter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
)

const actionDialerOperate = "dialer"

func init() {
	actionCreatorMap[actionDialerOperate] = newActionDialer
}

type actionDialerOptions struct {
	Delete option.Listable[string] `json:"delete,omitempty"`
	Add    map[string]any          `json:"add,omitempty"`
	option.OutboundFilterOptions
}

type actionDialer struct {
	filter *filter.OutboundFilter
	delete []string
	add    map[string]any
}

func newActionDialer(b []byte) (providerAction, error) {
	var options actionDialerOptions
	err := json.Unmarshal(b, &options)
	if err != nil {
		return nil, err
	}
	a := &actionDialer{}
	f, err := filter.NewOutboundFilter(options.OutboundFilterOptions)
	if err != nil {
		return nil, err
	}
	a.filter = f
	a.delete = options.Delete
	a.add = options.Add
	return a, nil
}

func (a *actionDialer) execute(_ context.Context, _ adapter.Router, logger log.ContextLogger, groupContext *ProviderActionGroupContext) (bool, error) {
	for _, outbound := range groupContext.outbounds {
		if a.filter != nil && !a.filter.MatchOutboundOptions(outbound) {
			continue
		}
		dialerOptions := getDialerOptions(outbound)
		if dialerOptions == nil {
			continue
		}
		if len(a.delete) > 0 {
			for _, key := range a.delete {
				removeKey(dialerOptions, key)
				logger.Debug("action[dialer]: tag: [", outbound.Tag, "], delete ", key)
			}
		}
		if a.add != nil && len(a.add) > 0 {
			for k, v := range a.add {
				addKey(dialerOptions, k, v)
				logger.Debug("action[dialer]: tag: [", outbound.Tag, "], set ", k, " -> ", fmt.Sprintf("%v", v))
			}
		}
	}
	return true, nil
}

func getDialerOptions(options *option.Outbound) *option.DialerOptions {
	var dialerOptions *option.DialerOptions
	switch options.Type {
	case C.TypeDirect:
		dialerOptions = &options.DirectOptions.DialerOptions
	case C.TypeSOCKS:
		dialerOptions = &options.SocksOptions.DialerOptions
	case C.TypeHTTP:
		dialerOptions = &options.HTTPOptions.DialerOptions
	case C.TypeShadowsocks:
		dialerOptions = &options.ShadowsocksOptions.DialerOptions
	case C.TypeVMess:
		dialerOptions = &options.VMessOptions.DialerOptions
	case C.TypeTrojan:
		dialerOptions = &options.TrojanOptions.DialerOptions
	case C.TypeWireGuard:
		dialerOptions = &options.WireGuardOptions.DialerOptions
	case C.TypeHysteria:
		dialerOptions = &options.HysteriaOptions.DialerOptions
	case C.TypeTor:
		dialerOptions = &options.TorOptions.DialerOptions
	case C.TypeSSH:
		dialerOptions = &options.SSHOptions.DialerOptions
	case C.TypeShadowTLS:
		dialerOptions = &options.ShadowTLSOptions.DialerOptions
	case C.TypeShadowsocksR:
		dialerOptions = &options.ShadowsocksROptions.DialerOptions
	case C.TypeVLESS:
		dialerOptions = &options.VLESSOptions.DialerOptions
	case C.TypeTUIC:
		dialerOptions = &options.TUICOptions.DialerOptions
	case C.TypeHysteria2:
		dialerOptions = &options.Hysteria2Options.DialerOptions
	}
	return dialerOptions
}

func removeKey(p interface{}, key string) {
	t := reflect.TypeOf(p)
	v := reflect.ValueOf(p)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	for k := 0; k < t.NumField(); k++ {
		var match bool
		if strings.Contains(t.Field(k).Tag.Get("json"), key) {
			match = true
		}
		if t.Field(k).Name == key {
			match = true
		}
		if match {
			v.Field(k).Set(reflect.Zero(t.Field(k).Type))
		}
	}
}

func addKey(p interface{}, key string, value any) {
	t := reflect.TypeOf(p)
	v := reflect.ValueOf(p)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	for k := 0; k < t.NumField(); k++ {
		var match bool
		if strings.Contains(t.Field(k).Tag.Get("json"), key) {
			match = true
		}
		if t.Field(k).Name == key {
			match = true
		}
		if match {
			v.Field(k).Set(reflect.ValueOf(value))
		}
	}
}
