package action

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/outboundprovider/filter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

const actionGroupOperate = "group"

func init() {
	actionCreatorMap[actionGroupOperate] = newActionGroup
}

type actionGroupOptions struct {
	Options *option.Outbound `json:"options"`
	option.OutboundFilterOptions
}

type actionGroup struct {
	filter  *filter.OutboundFilter
	options option.Outbound
}

func newActionGroup(b []byte) (providerAction, error) {
	var options actionGroupOptions
	err := json.Unmarshal(b, &options)
	if err != nil {
		return nil, err
	}
	if options.Options == nil {
		return nil, E.New("missing options")
	}
	if options.Options.Tag == "" {
		return nil, E.New("missing tag")
	}
	switch options.Options.Type {
	case C.TypeSelector, C.TypeURLTest:
	case "":
		return nil, E.New("missing type")
	default:
		return nil, E.New("invalid type [", options.Options.Type, "]")
	}
	a := &actionGroup{}
	f, err := filter.NewOutboundFilter(options.OutboundFilterOptions)
	if err != nil {
		return nil, err
	}
	a.filter = f
	a.options = *options.Options
	return a, nil
}

func (a *actionGroup) execute(_ context.Context, _ adapter.Router, logger log.ContextLogger, groupContext *ProviderActionGroupContext) (bool, error) {
	outbounds := make([]string, 0)
	for _, outbound := range groupContext.outbounds {
		if a.filter != nil {
			if !a.filter.MatchOutboundOptions(outbound) {
				continue
			}
		}
		outbounds = append(outbounds, outbound.Tag)
	}
	logger.Debug("action[group]: ", a.options.Tag, " outbounds: [", strings.Join(outbounds, ", "), "]")
	newGroupOutbound := a.options
	switch newGroupOutbound.Type {
	case C.TypeSelector:
		outs := make([]string, 0, len(newGroupOutbound.SelectorOptions.Outbounds)+len(outbounds))
		copy(outs, newGroupOutbound.SelectorOptions.Outbounds)
		outs = append(outs, outbounds...)
		newGroupOutbound.SelectorOptions.Outbounds = outs
	case C.TypeURLTest:
		outs := make([]string, 0, len(newGroupOutbound.URLTestOptions.Outbounds)+len(outbounds))
		copy(outs, newGroupOutbound.URLTestOptions.Outbounds)
		outs = append(outs, outbounds...)
		newGroupOutbound.URLTestOptions.Outbounds = outs
	}
	groupContext.groupOutbounds = append(groupContext.groupOutbounds, &newGroupOutbound)
	groupContext.groupOutboundMap[newGroupOutbound.Tag] = &newGroupOutbound
	return true, nil
}
