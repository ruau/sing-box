package filter

import (
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type outboundFilterRule interface {
	matchOutboundOptions(outbound *option.Outbound) bool
	matchOutbound(outbound adapter.Outbound) bool
}

type OutboundFilter struct {
	logical string
	rules   []outboundFilterRule
}

func NewOutboundFilter(options option.OutboundFilterOptions) (*OutboundFilter, error) {
	switch options.Logical {
	case "or", "and":
	case "":
		options.Logical = "or"
	default:
		return nil, E.New("invalid logical [", options.Logical, "]")
	}
	if len(options.Rules) == 0 {
		return nil, E.New("missing rules")
	}
	filter := &OutboundFilter{
		logical: options.Logical,
		rules:   make([]outboundFilterRule, 0, len(options.Rules)),
	}
	for i, rule := range options.Rules {
		var r outboundFilterRule
		idx := 0
		if strings.HasPrefix(rule, "!") {
			idx += 1
		}
		if strings.HasPrefix(rule[idx:], "tag:") {
			r = tagFilter(rule[4+idx:])
		} else if strings.HasPrefix(rule[idx:], "type:") {
			r = typeFilter(rule[5+idx:])
		} else {
			re, err := regexp.Compile(rule[idx:])
			if err != nil {
				return nil, E.New("rule [", i, "]: invalid regex rule [", rule, "]")
			}
			r = (*tagRegexFilter)(re)
		}
		if idx == 1 {
			r = &invertFilter{r}
		}
		filter.rules = append(filter.rules, r)
	}
	return filter, nil
}

func (f *OutboundFilter) MatchOutboundOptions(outbound *option.Outbound) bool {
	if f.logical == "or" {
		for _, rule := range f.rules {
			if rule.matchOutboundOptions(outbound) {
				return true
			}
		}
		return false
	} else if f.logical == "and" {
		for _, rule := range f.rules {
			if !rule.matchOutboundOptions(outbound) {
				return false
			}
		}
		return true
	} else {
		panic("unreachable")
	}
}

func (f *OutboundFilter) MatchOutbound(outbound adapter.Outbound) bool {
	if f.logical == "or" {
		for _, rule := range f.rules {
			if rule.matchOutbound(outbound) {
				return true
			}
		}
		return false
	} else if f.logical == "and" {
		for _, rule := range f.rules {
			if !rule.matchOutbound(outbound) {
				return false
			}
		}
		return true
	} else {
		panic("unreachable")
	}
}

type invertFilter struct {
	inner outboundFilterRule
}

func (i *invertFilter) matchOutboundOptions(outbound *option.Outbound) bool {
	return !i.inner.matchOutboundOptions(outbound)
}

func (t *invertFilter) matchOutbound(outbound adapter.Outbound) bool {
	return !t.inner.matchOutbound(outbound)
}

type tagFilter string

func (t tagFilter) matchOutboundOptions(outbound *option.Outbound) bool {
	return strings.Contains(outbound.Tag, string(t))
}

func (t tagFilter) matchOutbound(outbound adapter.Outbound) bool {
	return strings.Contains(outbound.Tag(), string(t))
}

type typeFilter string

func (t typeFilter) matchOutboundOptions(outbound *option.Outbound) bool {
	return outbound.Type == string(t)
}

func (t typeFilter) matchOutbound(outbound adapter.Outbound) bool {
	return outbound.Type() == string(t)
}

type tagRegexFilter regexp.Regexp

func (r *tagRegexFilter) matchOutboundOptions(outbound *option.Outbound) bool {
	return (*regexp.Regexp)(r).MatchString(outbound.Tag)
}

func (r *tagRegexFilter) matchOutbound(outbound adapter.Outbound) bool {
	return (*regexp.Regexp)(r).MatchString(outbound.Tag())
}
