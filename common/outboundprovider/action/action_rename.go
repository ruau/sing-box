package action

import (
	"context"
	"fmt"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/outboundprovider/filter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

const actionRenameOperate = "rename"

func init() {
	actionCreatorMap[actionRenameOperate] = newActionRename
}

type actionRenameOptions struct {
	Format string `json:"format"`
	option.OutboundFilterOptions
}

type actionRename struct {
	filter *filter.OutboundFilter
	format string
}

func newActionRename(b []byte) (providerAction, error) {
	var options actionRenameOptions
	err := json.Unmarshal(b, &options)
	if err != nil {
		return nil, err
	}
	if options.Format == "" {
		return nil, E.New("missing format")
	}
	a := &actionRename{}
	f, err := filter.NewOutboundFilter(options.OutboundFilterOptions)
	if err != nil {
		return nil, err
	}
	a.filter = f
	a.format = options.Format
	return a, nil
}

func (a *actionRename) execute(_ context.Context, _ adapter.Router, logger log.ContextLogger, groupContext *ProviderActionGroupContext) (bool, error) {
	renameMap := make(map[string]string)
	for _, outbound := range groupContext.outbounds {
		if a.filter != nil {
			if !a.filter.MatchOutboundOptions(outbound) {
				continue
			}
		}
		newTag := fmt.Sprintf(a.format, outbound.Tag)
		logger.Debug("action[rename]: outbound: ", outbound.Tag, " -> ", newTag)
		renameMap[outbound.Tag] = newTag
		delete(groupContext.outboundMap, outbound.Tag)
		groupContext.outboundMap[newTag] = outbound
		outbound.Tag = newTag
	}
	for _, outbound := range groupContext.groupOutbounds {
		switch outbound.Type {
		case C.TypeSelector:
			for i, tag := range outbound.SelectorOptions.Outbounds {
				newTag, ok := renameMap[tag]
				if ok {
					outbound.SelectorOptions.Outbounds[i] = newTag
				}
			}
			if outbound.SelectorOptions.Default != "" {
				newTag, ok := renameMap[outbound.SelectorOptions.Default]
				if ok {
					outbound.SelectorOptions.Default = newTag
				}
			}
		case C.TypeURLTest:
			for i, tag := range outbound.URLTestOptions.Outbounds {
				newTag, ok := renameMap[tag]
				if ok {
					outbound.URLTestOptions.Outbounds[i] = newTag
				}
			}
		}
	}
	return true, nil
}
