package plugin

import "github.com/mwantia/forge-sdk/pkg/plugins"

func (p *SkillToolsPlugin) loadToolDefinitions() []plugins.ToolDefinition {
	tools := make([]plugins.ToolDefinition, 0, 4)

	if len(p.skills) > 0 {
		tools = append(tools, plugins.ToolDefinition{
			Name:        "activate",
			Description: "Load the full instructions for a skill. Call when the current task matches a skill's description.",
			Annotations: plugins.ToolAnnotations{
				ReadOnly:   true,
				Idempotent: true,
				CostHint:   plugins.ToolCostCheap,
				System: `
Call activate when the user's task matches a skill's description from the available_skills catalog. Returns the full SKILL.md body wrapped in <skill_content> tags plus a <skill_resources> listing of bundled files.
Follow the returned instructions exactly; do not summarize or skip steps. If the instructions reference files (scripts/*, references/*), load them with read_file before acting on them.
`,
			},
			Parameters: plugins.ToolParameters{
				Type:     "object",
				Required: []string{"name"},
				Properties: map[string]plugins.ToolProperty{
					"name": {Type: "string", Description: "The skill name to activate.", Enum: sortedSkillNames(p.skills)},
				},
			},
		})
	}

	tools = append(tools,
		plugins.ToolDefinition{
			Name:        "read_file",
			Description: "Read a file bundled with a skill. Use when a skill's instructions reference a file in its directory.",
			Annotations: plugins.ToolAnnotations{
				ReadOnly:   true,
				Idempotent: true,
				CostHint:   plugins.ToolCostCheap,
				System: `
Use to read a reference, asset, or script source that a skill's instructions point to. Path is relative to the skill root (e.g. 'references/REFERENCE.md', 'assets/template.json').
Do not read SKILL.md directly — use activate for that. Use list_files first if you are unsure which files exist in the skill directory.
`,
			},
			Parameters: plugins.ToolParameters{
				Type:     "object",
				Required: []string{"skill", "path"},
				Properties: map[string]plugins.ToolProperty{
					"skill": {Type: "string", Description: "The skill name."},
					"path":  {Type: "string", Description: "Relative path from the skill directory root (e.g. 'references/REFERENCE.md')."},
				},
			},
		},
		plugins.ToolDefinition{
			Name:        "execute_script",
			Description: "Execute a script bundled with a skill. Use when a skill's instructions direct you to run a script.",
			Annotations: plugins.ToolAnnotations{
				RequiresConfirmation: true,
				CostHint:             plugins.ToolCostModerate,
				System: `
Run scripts from a skill's scripts/ directory exactly as the skill's instructions direct. Scripts execute with the skill directory as CWD; use relative paths when passing file arguments.
Check exit_code, stdout, and stderr — a non-zero exit code means the script reported failure. If truncated=true the output was cut at the configured limit; work with what you have unless the missing section is critical.
Requires user confirmation before execution.
`,
			},
			Parameters: plugins.ToolParameters{
				Type:     "object",
				Required: []string{"skill", "script"},
				Properties: map[string]plugins.ToolProperty{
					"skill":  {Type: "string", Description: "The skill name."},
					"script": {Type: "string", Description: "Script path relative to the skill root (must be under scripts/)."},
					"args":   {Type: "array", Description: "Command-line arguments to pass to the script."},
					"env":    {Type: "object", Description: "Additional environment variables for the process."},
				},
			},
		},
		plugins.ToolDefinition{
			Name:        "list_files",
			Description: "List files available in a skill's directory. Use to discover what resources a skill bundles.",
			Annotations: plugins.ToolAnnotations{
				ReadOnly:   true,
				Idempotent: true,
				CostHint:   plugins.ToolCostCheap,
				System: `
Use to enumerate what a skill bundles before deciding which files to read or run. Prefer the <skill_resources> listing returned by activate when available — call list_files only when you need finer-grained browsing of a specific subdirectory.
`,
			},
			Parameters: plugins.ToolParameters{
				Type:     "object",
				Required: []string{"skill"},
				Properties: map[string]plugins.ToolProperty{
					"skill":     {Type: "string", Description: "The skill name."},
					"directory": {Type: "string", Description: "Subdirectory to list relative to skill root (e.g. 'references'). Defaults to skill root."},
				},
			},
		},
	)

	return tools
}
