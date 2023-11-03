//go:build !with_proxyprovider

package proxyprovider

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewProxyProvider(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ProxyProvider) (adapter.ProxyProvider, error) {
	return nil, E.New(`ProxyProvider is not included in this build, rebuild with -tags with_proxyprovider`)
}
