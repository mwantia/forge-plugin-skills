package plugin

// Skill holds the parsed contents of a SKILL.md file.
type Skill struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  []string
	Body          string // markdown body after frontmatter
	Dir           string // absolute path to skill directory
}
