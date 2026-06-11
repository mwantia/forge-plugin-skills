package plugin

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge-sdk/pkg/sandbox"
)

// standardSkillPaths returns the standard skill directories in priority order
// (lowest to highest). The configured path is appended by the caller last.
func standardSkillPaths() []string {
	var paths []string
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths,
			filepath.Join(home, ".agents", "skills"),
			filepath.Join(home, ".forge", "skills"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".agents", "skills"))
	}
	return paths
}

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

// System emits the tier-1 catalog — name + description for every loaded skill.
// Returns "" when no skills are present so the assembler produces no block.
// Called once at session creation; the result is frozen in the immutable system message.
func (p *SkillToolsPlugin) System(_ context.Context) (string, error) {
	if len(p.skills) == 0 {
		return "", nil
	}

	names := sortedSkillNames(p.skills)

	var sb strings.Builder
	sb.WriteString("The following skills are available. When the user's task matches a skill's description, call skill__activate with the skill name to load its full instructions before proceeding.\n\n")
	sb.WriteString("<available_skills>\n")
	for _, name := range names {
		skill := p.skills[name]
		fmt.Fprintf(&sb, "  <skill>\n    <name>%s</name>\n    <description>%s</description>\n  </skill>\n",
			skill.Name, skill.Description)
	}
	sb.WriteString("</available_skills>")

	return sb.String(), nil
}

// scanSkills discovers SKILL.md files across all configured paths.
// Later paths take priority on name collision (configured path is highest priority).
func (p *SkillToolsPlugin) scanSkills() error {
	paths := []string{}
	if p.driver.config.ScanStandardPaths {
		paths = append(paths, standardSkillPaths()...)
	}
	paths = append(paths, p.path)

	for _, dir := range paths {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		p.driver.log.Debug("Scanning skills directory", "path", dir)
		if err := filepath.WalkDir(dir, p.scanDirectory); err != nil {
			p.driver.log.Warn("Error scanning skills directory", "path", dir, "error", err)
		}
	}

	p.driver.log.Info("Skills loaded", "count", len(p.skills))
	return nil
}

func (p *SkillToolsPlugin) scanDirectory(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	if d.IsDir() {
		name := d.Name()
		if name == ".git" || name == "node_modules" {
			return filepath.SkipDir
		}
		return nil
	}
	if d.Name() != "SKILL.md" {
		return nil
	}

	skill, err := parseSkillFile(path)
	if err != nil {
		p.driver.log.Warn("Failed to parse skill file", "path", path, "error", err)
		return nil
	}

	if existing, ok := p.skills[skill.Name]; ok {
		p.driver.log.Debug("Skill shadowed", "name", skill.Name, "kept", skill.Dir, "shadowed", existing.Dir)
	}

	p.skills[skill.Name] = skill
	p.driver.log.Trace("Loaded skill", "name", skill.Name, "path", path)
	return nil
}

func parseSkillFile(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	if absDir, err := filepath.Abs(dir); err == nil {
		dir = absDir
	}

	skill := &Skill{
		Dir:      dir,
		Name:     filepath.Base(dir),
		Metadata: make(map[string]string),
	}

	frontmatter, body, err := parseFrontmatter(string(content))
	if err != nil {
		// No frontmatter — use directory name as skill name, body as description
		skill.Body = strings.TrimSpace(string(content))
		if skill.Name == "" {
			return nil, fmt.Errorf("skill name is required")
		}
		return skill, nil
	}

	skill.Body = body

	if name, ok := frontmatter["name"].(string); ok && name != "" {
		skill.Name = name
	}
	if desc, ok := frontmatter["description"].(string); ok {
		skill.Description = desc
	}
	if license, ok := frontmatter["license"].(string); ok {
		skill.License = license
	}
	if compat, ok := frontmatter["compatibility"].(string); ok {
		skill.Compatibility = compat
	}
	if meta, ok := frontmatter["metadata"].(map[string]string); ok {
		skill.Metadata = meta
	}
	if allowedTools, ok := frontmatter["allowed-tools"].(string); ok && allowedTools != "" {
		skill.AllowedTools = strings.Fields(allowedTools)
	}

	if skill.Name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	if skill.Description == "" {
		// Derive from first non-heading line of body
		for line := range strings.SplitSeq(strings.TrimSpace(body), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				skill.Description = line
				break
			}
		}
	}

	return skill, nil
}

// parseFrontmatter extracts YAML frontmatter between --- delimiters.
// Handles simple scalar values and one level of nested key-value maps (e.g. metadata:).
func parseFrontmatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, content, fmt.Errorf("no frontmatter found")
	}

	_, after, ok := strings.Cut(content, "---")
	if !ok {
		return nil, content, fmt.Errorf("no opening delimiter")
	}

	remaining := after
	if strings.HasPrefix(remaining, "\r\n") {
		remaining = remaining[2:]
	} else if strings.HasPrefix(remaining, "\n") {
		remaining = remaining[1:]
	}

	delimEnd := strings.Index(remaining, "\n---\n")
	if delimEnd == -1 {
		delimEnd = strings.Index(remaining, "\n---\r\n")
		if delimEnd == -1 {
			return nil, content, fmt.Errorf("no closing delimiter")
		}
	}

	frontmatterText := remaining[:delimEnd]
	body := strings.TrimSpace(remaining[delimEnd+5:]) // skip \n---\n

	frontmatter := make(map[string]any)
	var blockKey string
	var blockMap map[string]string

	for line := range strings.SplitSeq(frontmatterText, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if blockKey != "" {
				trimmed := strings.TrimSpace(line)
				if k, v, ok := strings.Cut(trimmed, ":"); ok {
					blockMap[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), "\"'")
				}
			}
			continue
		}

		// Non-indented line — flush any active nested block
		if blockKey != "" {
			frontmatter[blockKey] = blockMap
			blockKey = ""
			blockMap = nil
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		k, v, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}

		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)

		if value == "" {
			blockKey = key
			blockMap = make(map[string]string)
		} else {
			frontmatter[key] = strings.Trim(value, "\"'")
		}
	}

	if blockKey != "" {
		frontmatter[blockKey] = blockMap
	}

	return frontmatter, body, nil
}

// --- ToolsPlugin interface ---

func (p *SkillToolsPlugin) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	tools := p.loadToolDefinitions()

	filtered := tools[:0]
	for _, t := range tools {
		if plugins.MatchesToolsFilter(t, filter) {
			filtered = append(filtered, t)
		}
	}

	return &plugins.ListToolsResponse{
		Tools: filtered,
	}, nil
}

func (p *SkillToolsPlugin) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	for _, t := range p.loadToolDefinitions() {
		if t.Name == name {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("tool %q not found", name)
}

func (p *SkillToolsPlugin) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	for _, def := range p.loadToolDefinitions() {
		if def.Name == req.Tool {
			return plugins.ValidateAgainstDefinition(def, req), nil
		}
	}
	return &plugins.ValidateResponse{
		Valid:  false,
		Errors: []string{fmt.Sprintf("unknown tool %q", req.Tool)},
	}, nil
}

func (p *SkillToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch req.Tool {
	case "activate":
		return p.executeActivate(ctx, req)
	case "read_file":
		return p.executeReadFile(ctx, req)
	case "execute_script":
		return p.executeScript(ctx, req)
	case "list_files":
		return p.executeListFiles(ctx, req)
	default:
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("unknown tool %q", req.Tool),
			IsError: true,
		}, nil
	}
}

// --- Execute handlers ---

func (p *SkillToolsPlugin) executeActivate(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	name := req.Args.Get("name").StringOr("")
	skill, ok := p.skills[name]
	if !ok {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("skill %q not found", name),
			IsError: true,
		}, nil
	}

	resources := p.listSkillResources(ctx, skill)

	var sb strings.Builder
	fmt.Fprintf(&sb, "<skill_content name=%q>\n", skill.Name)
	sb.WriteString(skill.Body)
	fmt.Fprintf(&sb, "\n\nSkill directory: %s\n", skill.Dir)
	if len(resources) > 0 {
		sb.WriteString("\n<skill_resources>\n")
		for _, r := range resources {
			fmt.Fprintf(&sb, "  <file>%s</file>\n", r)
		}
		sb.WriteString("</skill_resources>\n")
	}
	sb.WriteString("</skill_content>")

	return &plugins.ExecuteResponse{Result: sb.String()}, nil
}

// readFileAllowedDirs are the only subdirectories read_file may access.
var readFileAllowedDirs = []string{"scripts", "references", "assets"}

func (p *SkillToolsPlugin) executeReadFile(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	skillName := req.Args.Get("skill").StringOr("")
	relPath := req.Args.Get("path").StringOr("")

	skill, ok := p.skills[skillName]
	if !ok {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("skill %q not found", skillName), IsError: true}, nil
	}

	clean := filepath.Clean(relPath)

	if clean == "SKILL.md" {
		return &plugins.ExecuteResponse{Result: "SKILL.md is not readable via read_file; use skill__activate to load skill instructions", IsError: true}, nil
	}
	if !allowedSubdir(clean, readFileAllowedDirs) {
		return &plugins.ExecuteResponse{Result: "path must be under scripts/, references/, or assets/", IsError: true}, nil
	}

	box, err := sandbox.NewTinySandbox(skill.Dir, sandbox.Config{
		Caps: []sandbox.Capability{sandbox.CapRead},
	})
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to open sandbox: %v", err), IsError: true}, nil
	}
	defer box.Close()

	content, err := box.ReadFile(ctx, clean)
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to read file: %v", err), IsError: true}, nil
	}

	return &plugins.ExecuteResponse{Result: string(content)}, nil
}

func (p *SkillToolsPlugin) executeListFiles(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	skillName := req.Args.Get("skill").StringOr("")
	subDir := req.Args.Get("directory").StringOr("")
	if subDir == "" {
		subDir = "."
	}

	skill, ok := p.skills[skillName]
	if !ok {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("skill %q not found", skillName), IsError: true}, nil
	}

	box, err := sandbox.NewTinySandbox(skill.Dir, sandbox.Config{
		Caps: []sandbox.Capability{sandbox.CapList},
	})
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to open sandbox: %v", err), IsError: true}, nil
	}
	defer box.Close()

	var files []string
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		entries, err := box.ReadDir(ctx, dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			entryRel := e.Name
			if rel != "" && rel != "." {
				entryRel = rel + "/" + e.Name
			}
			if e.IsDir {
				_ = walk(dir+"/"+e.Name, entryRel)
			} else if entryRel != "SKILL.md" {
				files = append(files, entryRel)
			}
		}
		return nil
	}
	_ = walk(subDir, subDir)

	return &plugins.ExecuteResponse{Result: files}, nil
}

func (p *SkillToolsPlugin) executeScript(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	skillName := req.Args.Get("skill").StringOr("")
	scriptRel := req.Args.Get("script").StringOr("")

	skill, ok := p.skills[skillName]
	if !ok {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("skill %q not found", skillName), IsError: true}, nil
	}

	clean := filepath.Clean(scriptRel)
	if !allowedSubdir(clean, []string{"scripts"}) {
		return &plugins.ExecuteResponse{Result: "script must be under scripts/ directory", IsError: true}, nil
	}

	var cmdArgs []string
	if args, ok := req.Args.Get("args").Raw().([]any); ok {
		for i, a := range args {
			s, ok := a.(string)
			if !ok {
				continue
			}
			if err := validateScriptArg(s); err != nil {
				return &plugins.ExecuteResponse{
					Result:  fmt.Sprintf("args[%d]: %v", i, err),
					IsError: true,
				}, nil
			}
			cmdArgs = append(cmdArgs, s)
		}
	}

	var extraEnv []string
	if envMap, ok := req.Args.Get("env").Raw().(map[string]any); ok {
		for k, v := range envMap {
			if s, ok := v.(string); ok {
				extraEnv = append(extraEnv, k+"="+s)
			}
		}
	}

	box, err := sandbox.NewTinySandbox(skill.Dir, p.driver.config.sandboxConfig())
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to open sandbox: %v", err), IsError: true}, nil
	}
	defer box.Close()

	if err := p.driver.config.applyProfiles(box); err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("failed to apply exec profiles: %v", err), IsError: true}, nil
	}

	result, err := box.ExecProfile(ctx, sandbox.ExecRequest{
		Path:        clean,
		Args:        cmdArgs,
		Environment: extraEnv,
	})
	if err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("exec error: %v", err), IsError: true}, nil
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
			"stderr":    result.Stderr,
			"truncated": result.Truncated,
		},
		IsError: result.ExitCode != 0,
	}, nil
}

// --- Helpers ---

func (p *SkillToolsPlugin) listSkillResources(ctx context.Context, skill *Skill) []string {
	box, err := sandbox.NewTinySandbox(skill.Dir, sandbox.Config{
		Caps: []sandbox.Capability{sandbox.CapList},
	})
	if err != nil {
		return nil
	}
	defer box.Close()

	var files []string
	var walk func(dir, prefix string)
	walk = func(dir, prefix string) {
		entries, err := box.ReadDir(ctx, dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			rel := prefix + e.Name
			if e.IsDir {
				walk(dir+"/"+e.Name, rel+"/")
			} else {
				files = append(files, rel)
			}
		}
	}
	for _, subdir := range []string{"scripts", "references", "assets"} {
		walk(subdir, subdir+"/")
	}
	return files
}

// validateScriptArg rejects args that are absolute paths or traverse outside
// the working directory. Scripts run with the skill dir as CWD, so any arg
// that resolves above it would escape the boundary.
func validateScriptArg(arg string) error {
	if filepath.IsAbs(arg) {
		return fmt.Errorf("absolute paths are not allowed (%q)", arg)
	}
	if clean := filepath.Clean(arg); strings.HasPrefix(clean, "..") {
		return fmt.Errorf("path traversal is not allowed (%q)", arg)
	}
	return nil
}

// allowedSubdir reports whether clean (already filepath.Clean'd) is directly
// under one of the named subdirectories. Bare directory names and traversal
// attempts both return false.
func allowedSubdir(clean string, dirs []string) bool {
	sep := string(filepath.Separator)
	for _, dir := range dirs {
		if strings.HasPrefix(clean, dir+sep) {
			return true
		}
	}
	return false
}

func sortedSkillNames(skills map[string]*Skill) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
