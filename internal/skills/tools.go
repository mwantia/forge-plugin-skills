package skills

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/errors"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

type SkillToolsPlugin struct {
	plugins.UnimplementedToolsPlugin

	driver *SkillsDriver
	path   string
	skills map[string]*Skill
}

func NewSkillToolsPlugin(driver *SkillsDriver) (*SkillToolsPlugin, error) {
	path := driver.config.Path
	if path == "" {
		path = "./skills"
	}

	return &SkillToolsPlugin{
		driver: driver,
		path:   path,
		skills: make(map[string]*Skill),
	}, nil
}

func (p *SkillToolsPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *SkillToolsPlugin) scanSkills() error {
	p.driver.log.Info("Searching SKILL tools", "path", p.path)
	err := filepath.WalkDir(p.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "SKILL.md" {
			return nil
		}

		skill, err := p.parseSkillFile(path)
		if err != nil {
			p.driver.log.Warn("Failed to parse skill file", "path", path, "error", err)
			return nil // Continue scanning other files
		}

		p.skills[skill.Name] = skill
		p.driver.log.Trace("Loaded skill", "name", skill.Name, "path", path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk skills directory: %w", err)
	}

	return nil
}

func (p *SkillToolsPlugin) parseSkillFile(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	skill := &Skill{
		Path:       path,
		Content:    string(content),
		Tags:       make([]string, 0),
		Parameters: make(map[string]Parameter),
	}

	// Extract skill name from parent directory
	dir := filepath.Dir(path)
	skill.Name = filepath.Base(dir)
	// Parse frontmatter and content
	frontmatter, body, err := parseFrontmatter(string(content))
	if err == nil && frontmatter != nil {
		// Use frontmatter values
		if name, ok := frontmatter["name"].(string); ok && name != "" {
			skill.Name = name
		}
		if desc, ok := frontmatter["description"].(string); ok {
			skill.Description = desc
		}
		if readonly, ok := frontmatter["readonly"].(string); ok {
			skill.Readonly = strings.ToLower(readonly) == "true"
		}
		if destructive, ok := frontmatter["destructive"].(string); ok {
			skill.Destructive = strings.ToLower(destructive) == "true"
		}
		if idempotent, ok := frontmatter["idempotent"].(string); ok {
			skill.Idempotent = strings.ToLower(idempotent) == "true"
		}
		if version, ok := frontmatter["version"].(string); ok {
			skill.Version = version
		}
		if deprecated, ok := frontmatter["deprecated"].(string); ok {
			skill.Deprecated = strings.ToLower(deprecated) == "true"
		}
		if msg, ok := frontmatter["deprecation_message"].(string); ok {
			skill.DeprecationMessage = msg
		}
		if tagsRaw, ok := frontmatter["tags"].(string); ok && tagsRaw != "" {
			for tag := range strings.SplitSeq(tagsRaw, ",") {
				if t := strings.TrimSpace(tag); t != "" {
					skill.Tags = append(skill.Tags, t)
				}
			}
		}
		if params, ok := frontmatter["parameters"].(map[string]any); ok {
			for name, paramDef := range params {
				if param, ok := paramDef.(map[string]any); ok {
					p := Parameter{}
					if typ, ok := param["type"].(string); ok {
						p.Type = typ
					} else {
						p.Type = "string" // default type
					}
					if description, ok := param["description"].(string); ok {
						p.Description = description
					}
					if required, ok := param["required"].(bool); ok {
						p.Required = required
					}
					if enumRaw, ok := param["enum"].(string); ok {
						p.Enum = make([]string, 0)
						for enum := range strings.SplitSeq(enumRaw, ",") {
							if e := strings.TrimSpace(enum); e != "" {
								p.Enum = append(p.Enum, e)
							}
						}
					}
					if format, ok := param["format"].(string); ok {
						p.Format = format
					}

					skill.Parameters[name] = p
				}
			}
		}
	}

	if skill.Name == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	// Use body as description if not set in frontmatter
	if skill.Description == "" {
		// Use first paragraph or line
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) > 0 {
			// Find first non-empty, non-heading line
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					skill.Description = line
					break
				}
			}
		}
	}

	return skill, nil
}

// parseFrontmatter extracts YAML-like frontmatter from markdown content.
func parseFrontmatter(content string) (map[string]any, string, error) {
	// Check for frontmatter delimiter
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, content, fmt.Errorf("no frontmatter found")
	}

	// Find closing delimiter
	_, after, ok := strings.Cut(content, "---")
	if !ok {
		return nil, content, fmt.Errorf("no opening delimiter")
	}

	remaining := after // skip opening ---
	if strings.HasPrefix(remaining, "\r\n") {
		remaining = remaining[2:]
	} else if strings.HasPrefix(remaining, "\n") {
		remaining = remaining[1:]
	}

	delimEnd := strings.Index(remaining, "---\n")
	if delimEnd == -1 {
		delimEnd = strings.Index(remaining, "---\r\n")
		if delimEnd == -1 {
			return nil, content, fmt.Errorf("no closing delimiter")
		}
	}

	frontmatterText := strings.TrimSpace(remaining[:delimEnd])
	body := strings.TrimSpace(remaining[delimEnd+3:])

	// Simple YAML-like parsing
	frontmatter := make(map[string]any)
	lines := strings.SplitSeq(frontmatterText, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle key: value
		before, after, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key := strings.TrimSpace(before)
		value := strings.TrimSpace(after)

		// Simple value parsing (remove quotes)
		value = strings.Trim(value, "\"'")
		frontmatter[key] = value
	}

	return frontmatter, body, nil
}

func (p *SkillToolsPlugin) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	tools := make([]plugins.ToolDefinition, 0, len(p.skills))
	for name, skill := range p.skills {
		def := skillToToolDefinition(name, skill)
		if !skillMatchesFilter(def, filter) {
			continue
		}
		p.driver.log.Debug("Tool definition", "name", name, "description", skill.Description, "tags", skill.Tags)
		tools = append(tools, def)
	}

	return &plugins.ListToolsResponse{
		Tools: tools,
	}, nil
}

func (p *SkillToolsPlugin) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	skill, ok := p.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}

	def := skillToToolDefinition(name, skill)
	return &def, nil
}

func (p *SkillToolsPlugin) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	skill, ok := p.skills[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("skill %q not found", req.Tool)},
		}, nil
	}

	var errs []string
	for name, param := range skill.Parameters {
		if !param.Required {
			continue
		}
		v, present := req.Arguments[name]
		if !present || v == nil {
			errs = append(errs, fmt.Sprintf("%q is required", name))
			continue
		}
		if param.Type == "string" {
			if _, ok := v.(string); !ok {
				errs = append(errs, fmt.Sprintf("%q must be a string", name))
			}
		}
	}

	return &plugins.ValidateResponse{
		Valid:  len(errs) == 0,
		Errors: errs,
	}, nil
}

// skillToToolDefinition converts a Skill to a plugins.ToolDefinition.
func skillToToolDefinition(name string, skill *Skill) plugins.ToolDefinition {
	properties := make(map[string]plugins.ToolProperty)
	required := make([]string, 0)

	for name, param := range skill.Parameters {
		prop := plugins.ToolProperty{
			Type:        param.Type,
			Description: param.Description,
			Enum:        param.Enum,
			Format:      param.Format,
		}
		properties[name] = prop

		if param.Required {
			required = append(required, name)
		}
	}

	return plugins.ToolDefinition{
		Name:        name,
		Description: skill.Description,
		Parameters: plugins.ToolParameters{
			Type:       "object",
			Required:   required,
			Properties: properties,
		},
		Tags:               skill.Tags,
		Version:            skill.Version,
		Deprecated:         skill.Deprecated,
		DeprecationMessage: skill.DeprecationMessage,
		Annotations: plugins.ToolAnnotations{
			ReadOnly:    skill.Readonly,
			Destructive: skill.Destructive,
			Idempotent:  skill.Idempotent,
			CostHint:    "cheap",
		},
	}
}

// skillMatchesFilter reports whether def satisfies the given filter.
func skillMatchesFilter(def plugins.ToolDefinition, f plugins.ListToolsFilter) bool {
	if def.Deprecated && !f.Deprecated {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(def.Name, f.Prefix) {
		return false
	}
	if len(f.Tags) > 0 {
		for _, want := range f.Tags {
			for _, have := range def.Tags {
				if have == want {
					goto tagMatched
				}
			}
		}
		return false
	tagMatched:
	}
	return true
}

func (p *SkillToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	skill, ok := p.skills[req.Tool]
	if !ok {
		return nil, errors.ErrSkillNotFound
	}

	// Return the skill content for execution
	// The actual execution logic would be implemented by the agent/LLM
	result := map[string]any{
		"skill":     skill.Name,
		"content":   skill.Content,
		"path":      skill.Path,
		"arguments": req.Arguments,
		"executed":  true,
		"message":   fmt.Sprintf("Skill '%s' executed successfully", skill.Name),
	}

	return &plugins.ExecuteResponse{
		Result:  result,
		IsError: false,
	}, nil
}
