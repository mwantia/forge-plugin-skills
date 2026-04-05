package skills

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

const (
	PluginName        = "skills"
	PluginAuthor      = "forge"
	PluginVersion     = "0.1.0"
	PluginDescription = "Skills tools for executing predefined agent skill definitions"
)

// SkillsDriver implements plugins.Driver for the skills plugin.
type SkillsDriver struct {
	plugins.UnimplementedDriver
	log    hclog.Logger
	config *SkillsToolsConfig
	tools  *SkillToolsPlugin
}

type SkillsToolsConfig struct {
	Path string `mapstructure:"path"`
}

// NewSkillsDriver creates a new skills driver that supports tools plugin type.
func NewSkillsDriver(log hclog.Logger) plugins.Driver {
	return &SkillsDriver{
		log: log.Named(PluginName),
	}
}

// Lifecycle methods
func (d *SkillsDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      PluginAuthor,
		Version:     PluginVersion,
		Description: PluginDescription,
	}
}

func (d *SkillsDriver) ProbePlugin(ctx context.Context) (bool, error) {
	// Validate path exists
	info, err := os.Stat(d.tools.path)
	if err != nil {
		return false, fmt.Errorf("failed to access path '%s': %w", d.tools.path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("path '%s' is not a directory", d.tools.path)
	}

	return true, nil
}

func (d *SkillsDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *SkillsDriver) OpenDriver(ctx context.Context) error {
	d.log.Info("Creating skills tools plugin")
	var err error

	d.tools, err = NewSkillToolsPlugin(d)
	if err != nil {
		return fmt.Errorf("failed to create new tools plugin: %w", err)
	}

	return d.tools.scanSkills()
}

func (d *SkillsDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *SkillsDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	if err := mapstructure.Decode(config.ConfigMap, &d.config); err != nil {
		return fmt.Errorf("failed to decode config: %v", err)
	}

	return nil
}

func (d *SkillsDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return NewSkillToolsPlugin(d)
}
