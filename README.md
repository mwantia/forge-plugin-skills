# forge-plugin-skills

Tools plugin that discovers and exposes skill definitions from `SKILL.md` files on disk, making them available as agent-callable tools.

## Capabilities

| Capability | Supported |
|---|---|
| Tools | yes |
| Async execution | no |

## Configuration

```hcl
plugin "skills" {
  path = "./skills"  # default
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `path` | string | `./skills` | Directory to scan recursively for `SKILL.md` files |

## Defining skills

Each skill lives in its own subdirectory as a `SKILL.md` file. The directory name becomes the skill name unless overridden in the frontmatter.

```
skills/
├── summarize/
│   └── SKILL.md
└── translate/
    └── SKILL.md
```

### SKILL.md format

```markdown
---
name: "summarize"
description: "Summarize a block of text into bullet points."
readonly: true
idempotent: true
tags: "text,summarization"
version: "1.0.0"
parameters:
  text:
    type: string
    description: The text to summarize.
    required: true
  max_bullets:
    type: string
    description: Maximum number of bullet points.
    required: false
    default: "5"
---
Summarize the following text into at most {{ max_bullets }} concise bullet points:

{{ text }}
```

### Frontmatter fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Tool name (defaults to directory name) |
| `description` | string | Shown to the LLM as the tool description |
| `readonly` | bool | Marks the tool as read-only |
| `destructive` | bool | Marks the tool as destructive |
| `idempotent` | bool | Marks the tool as idempotent |
| `tags` | string | Comma-separated tags |
| `version` | string | Semantic version |
| `deprecated` | bool | Hide from default listings |
| `deprecation_message` | string | Reason shown when deprecated |
| `parameters.<name>.type` | string | Parameter type (currently `string`) |
| `parameters.<name>.description` | string | Parameter description passed to the LLM |
| `parameters.<name>.required` | bool | Whether the parameter must be provided |
| `parameters.<name>.default` | any | Default value if not provided |

## Execution

When a skill is called, the plugin returns its full `SKILL.md` content along with the provided arguments. The agent uses the content as a prompt template to carry out the skill.
