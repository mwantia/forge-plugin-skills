package plugin

import (
	"github.com/mwantia/forge-plugin-skills/internal/skills"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

func init() {
	plugins.Register(skills.PluginName, skills.PluginDescription, skills.NewSkillsDriver)
}
