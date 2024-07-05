package option

type RedirectInboundOptions struct {
	ListenOptions

	// Script
	Scripts Listable[ScriptOptions] `json:"scripts,omitempty"`
}

type TProxyInboundOptions struct {
	ListenOptions
	Network NetworkList `json:"network,omitempty"`

	// Script
	Scripts Listable[ScriptOptions] `json:"scripts,omitempty"`
}
