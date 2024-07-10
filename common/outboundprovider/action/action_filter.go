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
	newOutbounds := make([]*option.Outbound, 0, len(groupContext.outbounds))
	for _, outbound := range groupContext.outbounds {
		if !a.filter.MatchOutboundOptions(outbound) {
			newOutbounds = append(newOutbounds, outbound)
			continue
		}
		delete(groupContext.outboundMap, outbound.Tag)
		removeOutboundTags[outbound.Tag] = struct{}{}
		logger.Debug("action[filter]: tag: [", outbound.Tag, "]")
	}
	groupContext.outbounds = make([]*option.Outbound, 0, len(newOutbounds))
	groupContext.outbounds = append(groupContext.outbounds, newOutbounds...)
	for _, outbound := range groupContext.groupOutbounds {
		switch outbound.Type {
		case C.TypeSelector:
			newOutbounds := make([]string, 0, len(outbound.SelectorOptions.Outbounds))
			for _, out := range outbound.SelectorOptions.Outbounds {
				_, ok := removeOutboundTags[out]
				if !ok {
					newOutbounds = append(newOutbounds, out)
				}
			}
			outbound.SelectorOptions.Outbounds = make([]string, 0, len(newOutbounds))
			outbound.SelectorOptions.Outbounds = append(outbound.SelectorOptions.Outbounds, newOutbounds...)
			if outbound.SelectorOptions.Default != "" {
				_, ok := removeOutboundTags[outbound.SelectorOptions.Default]
				if ok {
					outbound.SelectorOptions.Default = ""
				}
			}
		case C.TypeURLTest:
			newOutbounds := make([]string, 0, len(outbound.URLTestOptions.Outbounds))
			for _, out := range outbound.URLTestOptions.Outbounds {
				_, ok := removeOutboundTags[out]
				if !ok {
					newOutbounds = append(newOutbounds, out)
				}
			}
			outbound.URLTestOptions.Outbounds = make([]string, 0, len(newOutbounds))
			outbound.URLTestOptions.Outbounds = append(outbound.URLTestOptions.Outbounds, newOutbounds...)
		}
	}
	return true, nil
}
