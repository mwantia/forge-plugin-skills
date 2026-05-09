package skills

import (
	"github.com/mwantia/forge-plugin-skills/plugin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

func init() {
	plugins.Register(plugin.PluginName, plugin.PluginDescription, plugin.NewSkillsDriver)
}
