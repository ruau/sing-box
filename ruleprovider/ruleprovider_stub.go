//go:build !with_ruleprovider

package ruleprovider

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewRuleProvider(ctx context.Context, logger log.ContextLogger, tag string, options option.RuleProvider) (adapter.RuleProvider, error) {
	return nil, E.New(`RuleProvider is not included in this build, rebuild with -tags with_ruleprovider`)
}
