package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

// Note: for proxy protocols, outbound creates early connections by default.

type Outbound interface {
	Type() string
	Tag() string
	Network() []string
	Dependencies() []string
	N.Dialer
	NewConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

type OutboundProvider interface {
	OutboundGroup
	Update(ctx context.Context) error
	ProviderInfo() *OutboundProviderInfo
	Outbound(tag string) (Outbound, bool)
	BasicOutbounds() []Outbound
	GroupOutbounds() []OutboundGroup
}
