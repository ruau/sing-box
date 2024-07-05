package option

type ScriptOptions struct {
	Command        string            `json:"command"`
	Args           Listable[string]  `json:"args,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	CurrentDir     string            `json:"current_dir,omitempty"`
	AcceptEvents   Listable[string]  `json:"accept_events,omitempty"`
	AsService      bool              `json:"as_service,omitempty"`
	StdoutLogLevel string            `json:"stdout_log_level,omitempty"`
	StderrLogLevel string            `json:"stderr_log_level,omitempty"`
}
