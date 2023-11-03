package adapter

import (
	"time"

	"github.com/sagernet/sing-box/option"
)

type RuleProvider interface {
	Service
	Tag() string
	SetRouter(Router)
	FormatRule(*[]option.DNSRule, *[]option.Rule) ([]option.DNSRule, []option.Rule, error)
	GetRuleInfo() (string, string, uint64, error) // format, behavior, ruleCount, error
	LastUpdateTime() time.Time
	Update()
}
