package clash

import (
	"fmt"

	"github.com/sagernet/sing-box/log"

	"gopkg.in/yaml.v3"
)

func ParseClassical(logger log.ContextLogger, raw []byte) (*ClashRuleSet, error) {
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
		err = set.InsertRule(rule)
		if err != nil {
			logger.Warn("invalid rule: ", rule)
		}
	}
	return set, nil
}
