package engine

const TerminalCommandExitMarker = "__GITGYM_COMMAND_EXIT__"

type TerminalClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type TerminalServerMessage struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Cols     uint16 `json:"cols,omitempty"`
	Rows     uint16 `json:"rows,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Message  string `json:"message,omitempty"`
}
