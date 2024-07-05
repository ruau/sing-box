//go:build !with_outbound_provider

package outbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewProvider(_ context.Context, _ adapter.Router, _ log.Factory, _ log.ContextLogger, _ string, _ option.ProviderOutboundOptions) (adapter.Outbound, error) {
	return nil, E.New(`OutboundProvider is not included in this build, rebuild with -tags with_outbound_provider`)
}
