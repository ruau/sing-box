package ruleprovider

import (
	"encoding/json"
	"os"
	"time"

	"github.com/sagernet/sing-box/ruleprovider/clash"
)

type Cache struct {
	LastUpdate time.Time           `json:"last_update,omitempty"`
	Behavior   string              `json:"behavior,omitempty"`
	Format     string              `json:"format,omitempty"`
	RuleSet    *clash.ClashRuleSet `json:"rule_set,omitempty"`
	RuleCount  uint64              `json:"rule_count,omitempty"`
}

type _Cache Cache

func (c *Cache) IsNil() bool {
	return c.RuleSet == nil
}

func (c *Cache) WriteToFile(path string) error {
	raw, err := json.Marshal((*_Cache)(c))
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func (c *Cache) ReadFromFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, (*_Cache)(c))
}

type Format string

const (
	FormatYAML Format = "yaml"
	FormatText Format = "text"
)

type Behavior string

const (
	BehaviorDomain    Behavior = "domain"
	BehaviorIPCIDR    Behavior = "ipcidr"
	BehaviorClassical Behavior = "classical"
)
