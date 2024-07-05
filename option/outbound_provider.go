package option

import (
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

type ProviderOutboundOptions struct {
	URL             string                                  `json:"url"`
	CacheTag        string                                  `json:"cache_tag,omitempty"`
	UpdateInterval  Duration                                `json:"update_interval,omitempty"`
	RequestTimeout  Duration                                `json:"request_timeout,omitempty"`
	HTTP3           bool                                    `json:"http3,omitempty"`
	Headers         map[string]string                       `json:"headers,omitempty"`
	SelectorOptions SelectorOutboundOptions                 `json:"selector,omitempty"`
	Actions         Listable[ProviderOutboundActionOptions] `json:"actions,omitempty"`
	ProviderDialer  DialerOptions                           `json:"dialer,omitempty"`
}

type ProviderOutboundActionOptions struct {
	Operate    string          `json:"operate"`
	RawMessage json.RawMessage `json:"-"`
}

func (o *ProviderOutboundActionOptions) UnmarshalJSON(data []byte) error {
	var m map[string]any
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}

	operateField, ok := m["operate"]
	if !ok {
		return E.New("missing operate")
	}
	p, ok := operateField.(string)
	if !ok {
		return E.New("operate must be string")
	}
	o.Operate = p
	delete(m, "operate")
	rawMessage, _ := json.Marshal(m)
	o.RawMessage = json.RawMessage(rawMessage)

	return nil
}

func (o *ProviderOutboundActionOptions) MarshalJSON() ([]byte, error) {
	var m map[string]any
	err := json.Unmarshal(o.RawMessage, &m)
	if err != nil {
		return nil, err
	}
	m["operate"] = o.Operate
	return json.Marshal(m)
}

type OutboundFilterOptions struct {
	Rules   Listable[string] `json:"rules"`
	Logical string           `json:"logical"` // default: or
}

type ProviderGroupOutboundOptions struct {
	Tag string `json:"tag"`
	OutboundFilterOptions
}
