package views

import (
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
)

// ParamChange is an alias for probe.ParamChange so existing TUI code compiles unchanged.
type ParamChange = probe.ParamChange

// CompareServerParams delegates to probe.CompareServerParams.
func CompareServerParams(cfg config.Config, p *ProbeServerResult) []ParamChange {
	return probe.CompareServerParams(cfg, p)
}
