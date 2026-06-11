package plugin

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
	PluginVersion     = "0.5.0"
	PluginDescription = "Skills tools for executing predefined agent skill definitions"
)

// SkillsDriver implements plugins.Driver for the skills plugin.
type SkillsDriver struct {
	plugins.UnimplementedDriver
	log    hclog.Logger
	config *SkillsToolsConfig
	tools  *SkillToolsPlugin
}

// NewSkillsDriver creates a new skills driver that supports tools plugin type.
func NewSkillsDriver(log hclog.Logger) plugins.Driver {
	return &SkillsDriver{
		log:    log.Named(PluginName),
		config: NewDefaultConfig(),
	}
}

func (d *SkillsDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      PluginAuthor,
		Version:     PluginVersion,
		Description: PluginDescription,
	}
}

func (d *SkillsDriver) GetPluginHealth(_ context.Context) (*plugins.PluginHealth, error) {
	if d.config.Path == "" {
		return &plugins.PluginHealth{
			Status:  plugins.StatusUnhealthy,
			Code:    plugins.HealthCodeConfigInvalid,
			Message: "skill path undefined",
			Action:  "Set path in the skills plugin config block.",
		}, nil
	}

	info, err := os.Stat(d.config.Path)
	if err != nil {
		return &plugins.PluginHealth{
			Status:  plugins.StatusUnhealthy,
			Code:    plugins.HealthCodeConfigInvalid,
			Message: fmt.Sprintf("skills path %q not accessible: %v", d.config.Path, err),
			Action:  "Ensure the skills directory exists and is readable.",
		}, nil
	}

	if !info.IsDir() {
		return &plugins.PluginHealth{
			Status:  plugins.StatusUnhealthy,
			Code:    plugins.HealthCodeConfigInvalid,
			Message: fmt.Sprintf("skills path %q is not a directory", d.config.Path),
			Action:  "Set path to a directory containing skill definition files.",
		}, nil
	}

	return &plugins.PluginHealth{
		Status:  plugins.StatusHealthy,
		Code:    plugins.HealthCodeOK,
		Message: "skills directory accessible",
	}, nil
}

func (d *SkillsDriver) GetCapabilities(_ context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *SkillsDriver) OpenDriver(_ context.Context) error {
	d.log.Info("Creating skills tools plugin")

	tools, err := NewSkillToolsPlugin(d)
	if err != nil {
		return fmt.Errorf("failed to create tools plugin: %w", err)
	}

	d.tools = tools
	return d.tools.scanSkills()
}

func (d *SkillsDriver) CloseDriver(_ context.Context) error {
	return nil
}

func (d *SkillsDriver) ConfigDriver(_ context.Context, config plugins.PluginConfig) error {
	if err := mapstructure.Decode(config.ConfigMap, d.config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}
	return nil
}

func (d *SkillsDriver) GetToolsPlugin(_ context.Context) (plugins.ToolsPlugin, error) {
	if d.tools == nil {
		return nil, fmt.Errorf("driver not opened")
	}
	return d.tools, nil
}
