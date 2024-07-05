package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/outboundprovider/filter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

var (
	_ adapter.Outbound      = (*Selector)(nil)
	_ adapter.OutboundGroup = (*Selector)(nil)
)

type Selector struct {
	myOutboundAdapter
	ctx                          context.Context
	tags                         []string
	providers                    []providerOutbound
	defaultTag                   string
	outbounds                    map[string]adapter.Outbound
	outboundTags                 []string
	selected                     adapter.Outbound
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
}

func NewSelector(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SelectorOutboundOptions) (*Selector, error) {
	outbound := &Selector{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeSelector,
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: options.Outbounds,
		},
		ctx:                          ctx,
		tags:                         options.Outbounds,
		defaultTag:                   options.Default,
		outbounds:                    make(map[string]adapter.Outbound),
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: options.InterruptExistConnections,
	}
	if len(options.Providers) > 0 {
		outbound.providers = make([]providerOutbound, 0, len(options.Providers))
		for i, provider := range options.Providers {
			if provider.Tag == "" {
				return nil, E.New("missing provider tag[", i, "]")
			}
			f, err := filter.NewOutboundFilter(provider.OutboundFilterOptions)
			if err != nil {
				return nil, E.Cause(err, "parse filter[", i, "]")
			}
			outbound.providers = append(outbound.providers, providerOutbound{
				providerTag: provider.Tag,
				filter:      f,
			})
		}
	}
	if len(outbound.tags) == 0 && len(outbound.providers) == 0 {
		return nil, E.New("missing tags and providers")
	}
	return outbound, nil
}

func (s *Selector) Dependencies() []string {
	dependencies := make([]string, 0, len(s.tags)+len(s.providers))
	dependencies = append(dependencies, s.dependencies...)
	for _, provider := range s.providers {
		dependencies = append(dependencies, provider.providerTag)
	}
	return dependencies
}

func (s *Selector) Network() []string {
	if s.selected == nil {
		return []string{N.NetworkTCP, N.NetworkUDP}
	}
	return s.selected.Network()
}

func (s *Selector) Start() error {
	outboundTags := make([]string, 0, len(s.tags))
	for i, tag := range s.tags {
		detour, loaded := s.router.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		s.outbounds[tag] = detour
		outboundTags = append(outboundTags, tag)
	}

	for i, p := range s.providers {
		provider, loaded := s.router.OutboundProvider(p.providerTag)
		if !loaded {
			return E.New("outbound provider[", i, "] provider not found: ", p.providerTag)
		}
		for _, outbound := range provider.BasicOutbounds() {
			if p.filter.MatchOutbound(outbound) {
				_, loaded := s.outbounds[outbound.Tag()]
				if loaded {
					return E.New("duplicate outbound tag: ", outbound.Tag())
				}
				s.outbounds[outbound.Tag()] = outbound
				outboundTags = append(outboundTags, outbound.Tag())
			}
		}
		for _, outbound := range provider.GroupOutbounds() {
			if p.filter.MatchOutbound(outbound) {
				_, loaded := s.outbounds[outbound.Tag()]
				if loaded {
					return E.New("duplicate outbound tag: ", outbound.Tag())
				}
				s.outbounds[outbound.Tag()] = outbound
				outboundTags = append(outboundTags, outbound.Tag())
			}
		}
	}
	if len(s.outbounds) == 0 {
		return E.New("missing outbounds")
	}
	s.outboundTags = make([]string, len(s.outbounds))
	copy(s.outboundTags, outboundTags)

	if s.tag != "" {
		cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
		if cacheFile != nil {
			selected := cacheFile.LoadSelected(s.tag)
			if selected != "" {
				detour, loaded := s.outbounds[selected]
				if loaded {
					s.selected = detour
					return nil
				}
			}
		}
	}

	if s.defaultTag != "" {
		detour, loaded := s.outbounds[s.defaultTag]
		if !loaded {
			return E.New("default outbound not found: ", s.defaultTag)
		}
		s.selected = detour
		return nil
	}

	s.selected = s.outbounds[s.tags[0]]
	return nil
}

func (s *Selector) Now() string {
	return s.selected.Tag()
}

func (s *Selector) All() []string {
	return s.outboundTags
}

func (s *Selector) SelectOutbound(tag string) bool {
	detour, loaded := s.outbounds[tag]
	if !loaded {
		return false
	}
	if s.selected == detour {
		return true
	}
	s.selected = detour
	if s.tag != "" {
		cacheFile := service.FromContext[adapter.CacheFile](s.ctx)
		if cacheFile != nil {
			err := cacheFile.StoreSelected(s.tag, tag)
			if err != nil {
				s.logger.Error("store selected: ", err)
			}
		}
	}
	s.interruptGroup.Interrupt(s.interruptExternalConnections)
	return true
}

func (s *Selector) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	conn, err := s.selected.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	return s.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (s *Selector) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := s.selected.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}
	return s.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (s *Selector) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	return s.selected.NewConnection(ctx, conn, metadata)
}

func (s *Selector) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	return s.selected.NewPacketConnection(ctx, conn, metadata)
}

func RealTag(detour adapter.Outbound) string {
	if group, isGroup := detour.(adapter.OutboundGroup); isGroup {
		return group.Now()
	}
	return detour.Tag()
}

type providerOutbound struct {
	providerTag string
	filter      *filter.OutboundFilter
}
