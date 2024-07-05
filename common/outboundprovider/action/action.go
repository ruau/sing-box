package action

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var actionCreatorMap = make(map[string]func([]byte) (providerAction, error))

type ProviderActionGroupContext struct {
	outbounds        []*option.Outbound
	outboundMap      map[string]*option.Outbound
	groupOutbounds   []*option.Outbound
	groupOutboundMap map[string]*option.Outbound
}

func (c *ProviderActionGroupContext) Outbounds() []*option.Outbound {
	return c.outbounds
}

func (c *ProviderActionGroupContext) GroupOutbounds() []*option.Outbound {
	return c.groupOutbounds
}

type ProviderActionGroup struct {
	actions []providerAction
}

type providerAction interface {
	execute(ctx context.Context, router adapter.Router, logger log.ContextLogger, groupContext *ProviderActionGroupContext) (bool, error) // bool: continue(true)|break(false)
}

func NewProviderActionGroup(options []option.ProviderOutboundActionOptions) (*ProviderActionGroup, error) {
	actions := make([]providerAction, 0, len(options))
	for i, opt := range options {
		creator, ok := actionCreatorMap[opt.Operate]
		if !ok {
			return nil, E.New("action[", i, "]: unknown operate [", opt.Operate, "]")
		}
		action, err := creator(opt.RawMessage)
		if err != nil {
			return nil, E.Cause(err, "action[", i, "]")
		}
		actions = append(actions, action)
	}
	return &ProviderActionGroup{actions: actions}, nil
}

func (g *ProviderActionGroup) Execute(ctx context.Context, router adapter.Router, logger log.ContextLogger, outbounds []*option.Outbound) (*ProviderActionGroupContext, error) {
	groupContext := &ProviderActionGroupContext{
		outbounds:        outbounds,
		outboundMap:      make(map[string]*option.Outbound),
		groupOutbounds:   make([]*option.Outbound, 0),
		groupOutboundMap: make(map[string]*option.Outbound),
	}
	for i, action := range g.actions {
		isContinue, err := action.execute(ctx, router, logger, groupContext)
		if err != nil {
			return nil, E.Cause(err, "action [", i, "]")
		}
		if !isContinue {
			break
		}
	}
	return groupContext, nil
}
