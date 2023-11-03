package clash

import (
	"fmt"
	"strings"

	"github.com/sagernet/sing-box/log"

	"gopkg.in/yaml.v3"
)

func ParseIPCIDR(logger log.ContextLogger, raw []byte, isText bool) (*ClashRuleSet, error) {
	if !isText {
		var yamlRule YamlRule
		err := yaml.Unmarshal(raw, &yamlRule)
		if err != nil {
			return nil, err
		}
		if len(yamlRule.Payload) == 0 {
			return nil, fmt.Errorf("no rules found")
		}
		set := NewClashRuleSet()
		for _, rule := range yamlRule.Payload {
			err = set.InsertIPCIDRRule(rule)
			if err != nil {
				logger.Warn("invalid rule: ", rule)
			}
		}
		return set, nil
	} else {
		lines := strings.Split(string(raw), "\n")
		if len(lines) == 0 {
			return nil, fmt.Errorf("no rules found")
		}
		set := NewClashRuleSet()
		var err error
		var h bool
		for _, line := range lines {
			line = strings.TrimSuffix(line, "\r")
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "//") {
				continue
			}
			err = set.InsertIPCIDRRule(line)
			if err != nil {
				logger.Warn("invalid rule: ", line)
			} else {
				h = true
			}
		}
		if !h {
			return nil, fmt.Errorf("no rules found")
		}
		return set, nil
	}
}
