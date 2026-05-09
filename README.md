# forge-plugin-skills

Tools plugin that discovers and exposes skill definitions from `SKILL.md` files on disk, making them available as agent-callable tools.

## Capabilities

| Capability | Supported |
|---|---|
| Tools | yes |
| Async execution | no |

## Configuration

```hcl
plugin "skills" "skills" {
  config {
    path         = "./skills"
    capabilities = ["act", "list", "read", "exec"]

    exec {
      allowed_environment = ["HOME", "PATH", "TZ", "LANG"]
      runtime_timeout     = "60s"
      max_output          = "32kb"

      # Optional: override or extend preset exec profiles.
      # name is the interpreter binary (path or name on PATH).
      # extensions are matched without a leading dot.
      profile {
        name       = "/usr/local/bin/node"
        extensions = ["js", "mjs"]
        arguments  = []
      }
    }
  }
}
```

### Config fields

| Field | Type | Default | Description |
|---|---|---|---|
| `path` | string | `./skills` | Directory scanned recursively for `SKILL.md` files |
| `capabilities` | list | all | Which tools to expose: `act`, `list`, `read`, `exec` |

### `exec {}` block

Controls sandbox behaviour for `execute_script`. Scripts run inside a capability-gated `os.Root` sandbox — path traversal and symlink escapes are rejected at the OS level.

| Field | Type | Default | Description |
|---|---|---|---|
| `allowed_environment` | list | `["HOME","PATH","TZ","LANG"]` | Host env vars passed into the script process |
| `runtime_timeout` | duration | `"60s"` | Maximum wall-clock time per script |
| `max_output` | size | `"32kb"` | Combined stdout/stderr cap; output is truncated if exceeded (`"32kb"`, `"2mb"`, …) |

### `profile {}` block (repeatable)

Overrides or extends the built-in interpreter presets. The interpreter is resolved by matching the script's file extension.

| Field | Type | Description |
|---|---|---|
| `name` | string | Interpreter binary — path or name on `PATH` (e.g. `"python3"`, `"/usr/local/bin/node"`) |
| `extensions` | list | File extensions handled by this profile, without a leading dot (e.g. `["js", "mjs"]`) |
| `arguments` | list | Flags prepended before the script path (e.g. `["--experimental-vm-modules"]`) |
| `environment` | list | Additional `KEY=VALUE` pairs set for every invocation of this profile |

Built-in presets:

| Profile | Extensions | Interpreter |
|---|---|---|
| bash | `sh` | `bash` |
| python3 | `py` | `python3` |
| node | `js` | `node` |

## Skill directory layout

```
skills/
├── get-weather/
│   ├── SKILL.md
│   └── scripts/
│       └── weather.sh
└── summarize/
    ├── SKILL.md
    └── references/
        └── REFERENCE.md
```

Each skill lives in its own subdirectory. The directory name is the skill name unless overridden in frontmatter. Scripts intended for execution must live under `scripts/`.

## SKILL.md format

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
| `parameters.<name>.type` | string | Parameter type (`string`, `array`, `object`) |
| `parameters.<name>.description` | string | Parameter description passed to the LLM |
| `parameters.<name>.required` | bool | Whether the parameter must be provided |
| `parameters.<name>.default` | any | Default value if not provided |

## Tools

All tools are namespaced as `skills__<name>` when registered with the agent.

### `activate`

Loads the full instructions for a skill. Returns the `SKILL.md` body wrapped in `<skill_content>` tags plus a `<skill_resources>` listing of bundled files.

| Parameter | Required | Description |
|---|---|---|
| `name` | yes | Skill name to activate |

Requires capability: `act`

### `read_file`

Reads a file bundled with a skill (e.g. a reference document or script source). Path is relative to the skill root.

| Parameter | Required | Description |
|---|---|---|
| `skill` | yes | Skill name |
| `path` | yes | Relative path from the skill root (e.g. `references/REFERENCE.md`) |

Requires capability: `read`

### `execute_script`

Executes a script from a skill's `scripts/` directory inside the sandbox. Returns `exit_code`, `stdout`, `stderr`, and a `truncated` flag.

| Parameter | Required | Description |
|---|---|---|
| `skill` | yes | Skill name |
| `script` | yes | Script path relative to the skill root (must be under `scripts/`) |
| `args` | no | Command-line arguments passed to the script |
| `env` | no | Additional `KEY=VALUE` environment variables |

Requires capability: `exec`. Requires user confirmation before execution.

### `list_files`

Lists files in a skill's directory or a named subdirectory.

| Parameter | Required | Description |
|---|---|---|
| `skill` | yes | Skill name |
| `directory` | no | Subdirectory relative to skill root (defaults to skill root) |

Requires capability: `list`
