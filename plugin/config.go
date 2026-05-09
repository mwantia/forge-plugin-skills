package plugin

import (
	"strconv"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/sandbox"
)

// SkillCapability gates which skill tools are advertised and callable.
type SkillCapability string

const (
	CapActivate SkillCapability = "act"
	CapList     SkillCapability = "list"
	CapRead     SkillCapability = "read"
	CapExec     SkillCapability = "exec"
)

//	config {
//	    path         = "./skills"
//	    capabilities = ["act", "list", "read", "exec"]
//
//	    exec {
//	        allowed_environment = ["HOME", "PATH", "TZ", "LANG"]
//	        runtime_timeout     = "60s"
//	        max_output          = "32kb"
//
//	        profile {
//	            name        = "/usr/local/bin/node"
//	            extensions  = ["js", "mjs"]
//	            arguments   = []
//	            environment = []
//	        }
//	    }
//	}
type SkillsToolsConfig struct {
	Path string            `mapstructure:"path"`
	Caps []SkillCapability `mapstructure:"capabilities"`
	Exec SkillsExecConfig  `mapstructure:"exec"`
}

type SkillsExecConfig struct {
	AllowedEnvironment []string                  `mapstructure:"allowed_environment"`
	RuntimeTimeout     string                    `mapstructure:"runtime_timeout"`
	MaxOutput          string                    `mapstructure:"max_output"`
	Profiles           []SkillsExecProfileConfig `mapstructure:"profile"`
}

// SkillsExecProfileConfig mirrors sandbox.ExecProfile for HCL config decoding.
// Entries overwrite the preset profile for any shared extension.
type SkillsExecProfileConfig struct {
	Name        string   `mapstructure:"name"`
	Extensions  []string `mapstructure:"extensions"`
	Arguments   []string `mapstructure:"arguments"`
	Environment []string `mapstructure:"environment"`
}

func NewDefaultConfig() *SkillsToolsConfig {
	return &SkillsToolsConfig{
		Path: "./skills",
		Caps: []SkillCapability{
			CapActivate,
			CapList,
			CapRead,
			CapExec,
		},
		Exec: SkillsExecConfig{
			AllowedEnvironment: []string{"HOME", "PATH", "TZ", "LANG"},
			RuntimeTimeout:     "60s",
			MaxOutput:          "32kb",
		},
	}
}

// applyProfiles registers any config-defined profiles on box, overwriting preset
// entries for matching extensions.
func (c *SkillsToolsConfig) applyProfiles(box *sandbox.TinySandbox) error {
	for _, p := range c.Exec.Profiles {
		if err := box.RegisterProfile(sandbox.ExecProfile{
			Name:        p.Name,
			Extensions:  p.Extensions,
			Arguments:   p.Arguments,
			Environment: p.Environment,
		}); err != nil {
			return err
		}
	}
	return nil
}

// sandboxConfig builds a sandbox.Config from the exec section.
func (c *SkillsToolsConfig) sandboxConfig() sandbox.Config {
	cfg := sandbox.Config{
		Caps:               []sandbox.Capability{sandbox.CapRead, sandbox.CapList, sandbox.CapExec},
		MaxOutputBytes:     parseBytes(c.Exec.MaxOutput),
		InheritEnvironment: c.Exec.AllowedEnvironment,
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 32 * 1024
	}

	d, err := time.ParseDuration(c.Exec.RuntimeTimeout)
	if err == nil && d > 0 {
		cfg.RuntimeTimeout = d
	} else {
		cfg.RuntimeTimeout = 60 * time.Second
	}

	return cfg
}

// parseBytes converts a human-readable byte size ("32kb", "2mb", "1gb") to an
// integer. A bare number is returned as-is. Returns 0 on parse failure.
func parseBytes(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	suffixes := []struct {
		suffix string
		factor int
	}{
		{"gb", 1 << 30},
		{"mb", 1 << 20},
		{"kb", 1 << 10},
	}
	for _, m := range suffixes {
		if strings.HasSuffix(s, m.suffix) {
			n, err := strconv.Atoi(strings.TrimSuffix(s, m.suffix))
			if err == nil && n > 0 {
				return n * m.factor
			}
			return 0
		}
	}
	n, _ := strconv.Atoi(s)
	return n
}
