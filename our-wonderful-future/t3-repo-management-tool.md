# T3: Repo Management Tool

A **T3** (declarative-tool-tool-tool) for managing GitHub repositories. Covers creation/prep (POST) and reading (GET). GitHub is the initial provider; the design should remain provider-agnostic at the schema level so other providers (GitLab, Bitbucket, etc.) can be added later.

---

## Operations

### POST — Create / Initial Prep

**Modes:**

| Mode | Description |
|------|-------------|
| `empty` | Create a new repo with no content |
| `static-template` | Create a repo from a GitHub template repo |
| `scaffold` | Create a repo and push basic scaffolding files (README, .gitignore, LICENSE, etc.) |
| `smart-template` | Create a repo and populate it using the smart templating engine (see [smart templating repo](#smart-templating)) |

---

### GET — Read

**Operations:**

| Operation | Description |
|-----------|-------------|
| `list` | Return the full set of repos for an owner (user or org) |
| `get` | Return details for a single named repo |

---

## Example Schema

### Create Request

```yaml
operation: create
provider: github
mode: scaffold           # empty | static-template | scaffold | smart-template
owner: je-sidestuff
name: my-new-repo
description: "A new repo"
visibility: private      # public | private | internal
options:
  auto_init: false       # only used for mode: empty
  template_repo: ""      # only used for mode: static-template (owner/repo)
  scaffold:              # only used for mode: scaffold
    readme: true
    gitignore: go
    license: MIT
  smart_template: ""     # only used for mode: smart-template (path or ref to template manifest)
```

### Create Response

```yaml
status: created
provider: github
owner: je-sidestuff
name: my-new-repo
full_name: je-sidestuff/my-new-repo
url: https://github.com/je-sidestuff/my-new-repo
clone_url: https://github.com/je-sidestuff/my-new-repo.git
visibility: private
created_at: "2026-03-27T17:52:00Z"
default_branch: main
```

### List Request

```yaml
operation: list
provider: github
owner: je-sidestuff
filters:
  visibility: all        # all | public | private
  type: all              # all | owner | member | forks | sources
  sort: updated          # created | updated | pushed | full_name
  direction: desc
  per_page: 30
  page: 1
```

### List Response

```yaml
status: ok
provider: github
owner: je-sidestuff
total_count: 42
page: 1
repos:
  - name: AI-sandboxing
    full_name: je-sidestuff/AI-sandboxing
    url: https://github.com/je-sidestuff/AI-sandboxing
    description: "AI agent sandbox"
    visibility: public
    default_branch: main
    updated_at: "2026-03-27T17:00:00Z"
    fork: false
  # ...
```

### Get Request

```yaml
operation: get
provider: github
owner: je-sidestuff
name: AI-sandboxing
```

### Get Response

```yaml
status: ok
provider: github
name: AI-sandboxing
full_name: je-sidestuff/AI-sandboxing
url: https://github.com/je-sidestuff/AI-sandboxing
clone_url: https://github.com/je-sidestuff/AI-sandboxing.git
description: "AI agent sandbox"
visibility: public
default_branch: main
language: Go
fork: false
archived: false
created_at: "2025-01-01T00:00:00Z"
updated_at: "2026-03-27T17:00:00Z"
pushed_at: "2026-03-27T17:00:00Z"
topics:
  - ai
  - agents
size_kb: 1024
open_issues: 0
```

---

## Example Manifests

### Manifest: Empty Repo

```yaml
# manifests/create-empty.yaml
operation: create
provider: github
mode: empty
owner: je-sidestuff
name: scratch-pad
description: "Scratch pad"
visibility: private
```

### Manifest: Repo from Static Template

```yaml
# manifests/create-from-template.yaml
operation: create
provider: github
mode: static-template
owner: je-sidestuff
name: new-go-service
description: "New Go microservice"
visibility: private
options:
  template_repo: je-sidestuff/go-service-template
```

### Manifest: Scaffolded Repo

```yaml
# manifests/create-scaffolded.yaml
operation: create
provider: github
mode: scaffold
owner: je-sidestuff
name: new-project
description: "New project with standard scaffolding"
visibility: private
options:
  scaffold:
    readme: true
    gitignore: go
    license: MIT
```

### Manifest: Smart-Templated Repo

```yaml
# manifests/create-smart-template.yaml
operation: create
provider: github
mode: smart-template
owner: je-sidestuff
name: new-ai-tool
description: "New AI tool bootstrapped via smart template"
visibility: private
options:
  smart_template: je-sidestuff/smart-templates/ai-tool-template.yaml
  # Template variables passed through to the smart templating engine
  template_vars:
    tool_name: new-ai-tool
    author: je-sidestuff
    language: python
```

### Template Manifest (Smart Templating)

The smart template manifest is consumed by the existing smart templating engine (see the smart templating repo). A repo management T3 would accept a reference to one of these and delegate rendering to that engine.

```yaml
# Example: ai-tool-template.yaml (lives in the smart templates repo)
name: ai-tool-template
description: "Scaffolds a new AI tool repo"
vars:
  - name: tool_name
    description: "Snake-case name for the tool"
    required: true
  - name: author
    required: true
  - name: language
    default: python
files:
  - path: README.md
    content: |
      # {{ tool_name }}
      By {{ author }}
  - path: main.{{ "py" if language == "python" else "go" }}
    content: |
      # {{ tool_name }} entrypoint
  - path: .gitignore
    content: |
      __pycache__/
      *.pyc
      .env
```

---

## Smart Templating

The smart templating functionality delegates to the existing smart templating repo (`je-sidestuff/...`). The repo management T3 does not own template rendering logic — it accepts a reference to a smart template manifest, passes through `template_vars`, and calls into the smart templating engine to materialise files before pushing them to the newly created repo.

This keeps the repo management T3 focused on repo lifecycle and avoids duplicating templating logic.

---

## T3 Schema (tool-tool registration)

```yaml
name: repo-management
description: "Create and read GitHub repositories"
inputs:
  manifest:
    type: string
    description: "Path to the operation manifest file (YAML or JSON)"
    required: true
outputs:
  result:
    type: string
    description: "Path to the output file written by the tool"
entrypoint:
  command: python
  args: ["repo_management/main.py", "--manifest", "{{ manifest }}"]
```

---

## Future Brainstorming / Open Questions

### Additional Operations
- **PUT / PATCH** — update repo settings (description, visibility, topics, branch protections, webhooks)
- **DELETE** — archive or delete a repo
- **PATCH branches** — create/protect/delete branches
- **POST collaborators** — add/remove team members or collaborators

### Provider Expansion
- GitLab, Bitbucket, Azure DevOps, Gitea
- Provider-specific fields should live in an `options.provider_overrides` block rather than polluting the top-level schema

### Smart Templating Integration Depth
- Should the T3 call the smart templating tool as a subprocess, import it as a library, or dispatch it as a separate T3 execution?
- What does the version/ref story look like for smart templates — should `smart_template` accept a git ref or a registry reference?

### Registry / Catalog
- Should created repos be automatically registered somewhere (e.g., an internal repo catalog T3)?
- How does this compose with a potential "org management" T3?

### Auth / Credential Handling
- GitHub token passed via env var (`GITHUB_TOKEN`) vs. a secrets manager reference
- Support for GitHub App auth (installation tokens) in addition to PATs

### Idempotency
- What should happen if the repo already exists? Options: error, skip, update.
- A `if_exists` field (`error` | `skip` | `update`) could handle this.

### Dry Run
- A `dry_run: true` flag that previews what would be created/changed without making API calls.

### Pagination / Streaming for List
- For orgs with many repos, list responses may need cursor-based pagination rather than page/per_page.
