package action

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/outboundprovider/filter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
)

const actionFilterOperate = "filter"

func init() {
	actionCreatorMap[actionFilterOperate] = newActionFilter
}

type actionFilterOptions struct {
	option.OutboundFilterOptions
}

type actionFilter struct {
	filter *filter.OutboundFilter
}

func newActionFilter(b []byte) (providerAction, error) {
	var options actionFilterOptions
	err := json.Unmarshal(b, &options)
	if err != nil {
		return nil, err
	}
	a := &actionFilter{}
	f, err := filter.NewOutboundFilter(options.OutboundFilterOptions)
	if err != nil {
		return nil, err
	}
	a.filter = f
	return a, nil
}

func (a *actionFilter) execute(_ context.Context, _ adapter.Router, logger log.ContextLogger, groupContext *ProviderActionGroupContext) (bool, error) {
	removeOutboundTags := make(map[string]struct{})
	for i, outbound := range groupContext.outbounds {
		if !a.filter.MatchOutboundOptions(outbound) {
			continue
		}
		groupContext.outbounds = append(groupContext.outbounds[:i], groupContext.outbounds[i+1:]...)
		delete(groupContext.outboundMap, outbound.Tag)
		removeOutboundTags[outbound.Tag] = struct{}{}
		logger.Debug("action[filter]: tag: [", outbound.Tag, "]")
	}
	for _, outbound := range groupContext.groupOutbounds {
		switch outbound.Type {
		case C.TypeSelector:
			for i, out := range outbound.SelectorOptions.Outbounds {
				_, ok := removeOutboundTags[out]
				if ok {
					outbound.SelectorOptions.Outbounds = append(outbound.SelectorOptions.Outbounds[:i], outbound.SelectorOptions.Outbounds[i+1:]...)
				}
			}
			if outbound.SelectorOptions.Default != "" {
				_, ok := removeOutboundTags[outbound.SelectorOptions.Default]
				if ok {
					outbound.SelectorOptions.Default = ""
				}
			}
		case C.TypeURLTest:
			for i, out := range outbound.URLTestOptions.Outbounds {
				_, ok := removeOutboundTags[out]
				if ok {
					outbound.URLTestOptions.Outbounds = append(outbound.URLTestOptions.Outbounds[:i], outbound.URLTestOptions.Outbounds[i+1:]...)
				}
			}
		}
	}
	return true, nil
}
