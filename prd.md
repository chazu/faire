# Git-backed Savvy Clone PRD

**Date:** 2026-01-31
**Source:** [chat.openai.com](https://chatgpt.com/c/697289b8-4d14-832c-82a7-0985174ab68b)

---

### **You**

Take a look at this cli tool https://github.com/getsavvyinc/savvy-cli
I want to write an alternative which has all the same features \_except\_ it uses a git repository to sync data, so there are no privacy concerns and it can be used freely without depending on their platform. When configuring the tool, users should claim a path in the git repository corresponding to their identity (like a username, i would pick chaz) or even a nested path with their team then their username (so platform/chaz) - and the tool should persist any changes made to that path. There are probably additional design questions which need to be worked out, but for now please research this savvy-cli tool and create a product requirements document for a clone tool written in golang using bubbletea and other charm.sh tools for menus where applicable

---

### **ChatGPT**

Below is a Product Requirements Document (PRD) for a Savvy CLI–compatible clone that swaps Savvy’s hosted backend for a Git repository as the sync + sharing mechanism, implemented in Go with Bubble Tea + other charm.sh TUIs.
I based the “feature parity” inventory on Savvy’s public README (workflows from record / record history, running workflows via run, placeholders, ask AI generation, sync for local copy + run --local) and what’s visible in the repo/release notes (export + extension work). Go Packages+2GitHub+2

PRD: “Git-backed Savvy-compatible CLI” (working name: git-savvy)
1) Summary
git-savvy is a terminal-first workflow/runbook tool compatible in spirit and UX with Savvy CLI, but with no proprietary SaaS dependency. All workflows and associated metadata live in a user-chosen Git repository. Sync, sharing, review, audit trails, and collaboration are achieved through standard Git operations (clone/pull/commit/push/PRs), removing privacy concerns and platform lock-in.
Savvy today focuses on creating workflows from commands (record, record history), running them (run), and parameterization (“placeholders”), plus AI-assisted generation (ask) and local sync (sync, run --local). Go Packages+1
git-savvy delivers the same user-facing capabilities but stores/syncs via Git instead of a hosted backend.

2) Goals & Non-goals
Goals

Feature parity with Savvy CLI user workflows:

Create workflows from terminal sessions (record) and shell history (record history). Go Packages+1

Run workflows interactively (run) with auto-advancing steps. Go Packages+1

Support placeholders like <param> with one-time prompting per parameter. GitHub

Maintain a local cache and an explicit sync mechanism. GitHub

AI-assisted generation (ask) remains available, but is opt-in and provider-agnostic. Go Packages+1

Git-first data model

Users “claim” a path (e.g., chaz/ or platform/chaz/) and all writes persist under that path.

Collaboration via PRs, code review, branch protections, CODEOWNERS, etc.

Great TUI/UX

Bubble Tea-driven flows for selecting history lines, searching workflows, running steps, resolving sync conflicts, editing placeholders.

Offline-first

Everything works locally if the repo is present; syncing is incremental and explicit.

Non-goals (initially)

Recreating Savvy’s hosted dashboard or link-based sharing UI (replaced by Git hosting + optional static export).

Shipping a browser extension in v1 (design hooks allowed; can be a v2+ add-on).

Guaranteeing exact command/flag compatibility byte-for-byte (we aim for “same features,” not necessarily identical CLI surface).

3) Target Users & Use Cases
Personas

SRE / Platform engineer: wants repeatable incident/ops workflows, reviewed and versioned like code.

Team lead: wants shared runbooks with audit trails and low friction contributions.

Security-conscious org: wants no external SaaS storing sensitive commands.

Core Use Cases

“I just did a gnarly fix; turn my steps into a runbook.”

“Pull a few commands from my shell history into a reusable workflow.”

“Run the workflow interactively; prompt me once for parameters; guide me step-by-step.”

“Sync team workflows from Git; submit changes as a PR.”

“Export workflow to Markdown for wiki/docs.”

4) Product Principles

Git is the source of truth

Local-first, explicit sync

Readable formats (YAML/JSON + optional Markdown)

Composable: workflows can be run non-interactively (CI) as well as via TUI

Safe by default: never auto-exec history selection; always show what will run

5) Functional Requirements (FR)
FR0: Repository & Identity Claiming (foundational)

FR0.1: init wizard:

Choose repo location:

clone from remote URL (HTTPS/SSH), or

point to an existing local repo.

Choose default branch (auto-detect).

Choose “identity path” (e.g., chaz, platform/chaz) used as the default write target.

Configure author info (name/email), optional signing, optional remote push.

FR0.2: Store config locally:

OS-native config dir (e.g., ~/.config/git-savvy/config.toml).

FR0.3: Validate identity path constraints:

no .., no absolute paths, no symlink escapes

optionally enforce path prefix rules for teams.

FR1: Data Model & Layout in Git

FR1.1: Canonical repo structure (suggested):

\`\`\`
perlCopy codeworkflows/
  <team>/<user>/          # claimed identity path (write-default)
    <workflow-slug>/
      workflow.yaml
      README.md           # optional generated markdown
      assets/             # optional attachments
shared/
  <workflow-slug>/...     # team-owned or curated workflows
index.json                # optional generated index for fast search
\`\`\`

FR1.2: Workflow schema (v1):

id, title, description

tags

created\_at, updated\_at

steps[]: each step includes:

name

command (string; shell)

shell (bash/zsh/sh/pwsh)

cwd (optional)

env (optional map)

continue\_on\_error (bool)

confirmation (bool / prompt text)

placeholders[] optional metadata (name, default, secret, validation)

FR1.3: Backward/forward compatibility:

versioned schema field, migration helpers.

FR2: Create Workflows From Commands
Savvy supports building workflows from provided commands, including recording a subshell and selecting from history. Go Packages+1

FR2.1: record

Launch an interactive subshell (user’s preferred shell).

Capture commands executed in that subshell session.

On exit, open a TUI editor:

select/deselect commands

rename steps

reorder steps

add descriptions

Save workflow under identity path and commit changes to Git.

FR2.2: record history

Load recent shell history.

TUI multi-select with preview and search.

Never execute selected commands during creation. GitHub

Convert to steps and save/commit.

FR2.3: Alias expansion / normalization

Savvy notes it expands aliases to improve portability. Go Packages+1

Provide best-effort alias/function expansion:

bash: alias + declare -f

zsh: alias + functions

Store both “original” and “expanded” representations if possible.

FR3: Run Workflows Interactively (TUI)
Savvy’s run searches and runs workflows, auto-filling the next command and letting the user press enter to execute. Go Packages+1

FR3.1: run command

Search workflows (local index) by title/tag/text.

Select workflow; show overview and step list.

Execute step-by-step:

show command

show last output snippet

press Enter to run, s to skip, q to quit, r to rerun last, e to edit step (optional)

Persist run metadata locally (not necessarily committed):

last-run time, last success/failure, run logs (optional).

FR3.2: run --local

Run from local repo cache even without network; Savvy explicitly supports offline via local workflows. GitHub

FR3.3: Non-interactive mode

run --yes or run --non-interactive for CI usage (with required params supplied via flags/env).

FR4: Placeholders (Parameterization)
Savvy uses <parameters> and prompts once per unique placeholder during run. GitHub

FR4.1: Placeholder syntax

Detect <param> tokens in commands.

Prompt once per unique param in a run.

Replace consistently across steps.

FR4.2: Placeholder metadata

Optional placeholders section for:

default values

validation regex

“secret” masking (do not echo)

FR4.3: Persist filled values (optional)

Option A: ephemeral per-run

Option B: store in local secure storage (keychain) keyed by workflow+param

FR5: Sync (Git-based)
Savvy provides sync to keep a local copy of workflows and allow run --local. GitHub
git-savvy generalizes this as Git pull/push, with conflict handling.

FR5.1: sync

git fetch + fast-forward/rebase local branch (configurable).

Update local search index.

Show summary:

new workflows, updated workflows, conflicts.

FR5.2: Conflict resolution TUI

If merge conflicts occur in workflow files:

show side-by-side diff (Bubble Tea viewport)

choose ours/theirs/manual open editor

FR5.3: status

Show dirty files, ahead/behind, last sync time.

FR5.4: “Write policy” modes

Direct push mode (solo user)

PR mode: write commits on a feature branch, print instructions/URL for PR creation (or integrate with gh if present).

FR6: AI-Assisted Workflow Generation (ask)
Savvy supports generating workflows/commands from natural language. Go Packages+1

FR6.1: ask

TUI prompt for goal (“publish go module”, “rotate certs”, etc.)

Output either:

a complete workflow, or

a single command step

User reviews/edits before saving.

FR6.2: Provider model

No default SaaS required.

Support at least:

“bring your own API key” for a few providers

a “local” provider hook (OpenAI-compatible endpoint, Ollama-style HTTP, etc.)

FR6.3: Privacy guardrails

Redaction options (strip tokens, IPs, paths)

explicit “send to model?” confirmation if prompts include command history

FR7: Explain (explain)
Savvy includes explain in its CLI (documented in README). Go Packages+1

FR7.1: explain <command|step>

Explain a shell command in plain language

optionally annotate risk (“this deletes recursively”)

if provided a workflow step, explain in workflow context

FR8: Export & Interop
Savvy supports exporting workflows to Markdown. Go Packages+1

FR8.1: export <workflow> → Markdown

Generate README.md with:

title/description

prerequisites

parameter list

numbered steps with code blocks

FR8.2: export --format yaml|json|md

FR8.3: Import (nice-to-have)

Import from Markdown template or from Savvy export format if discoverable.

FR9: Editing & Browsing

FR9.1: list / search

quick list and fuzzy search in TUI

FR9.2: view <workflow>

render markdown (glamour) and show step details

FR9.3: edit <workflow>

open $EDITOR or built-in TUI editor for steps

FR10: Versioning & Self-Update
Savvy has an upgrade command and versions/releases. GitHub

FR10.1: version

FR10.2: upgrade

Use goreleaser-compatible upgrade flow (can reuse getsavvyinc/upgrade-cli patterns or implement similar)

6) Command Surface (proposed)
A pragmatic CLI that maps to Savvy’s conceptual commands:

git-savvy init

git-savvy sync

git-savvy record

git-savvy record history

git-savvy run [--local] [--yes]

git-savvy ask

git-savvy explain

git-savvy export

git-savvy list | search | view | edit

git-savvy status

git-savvy whoami (prints identity path + git identity)

git-savvy upgrade

git-savvy version

(We can alias some commands for closer muscle memory.)

7) UX / TUI Requirements (Bubble Tea + charm.sh)
Key screens

Init wizard (use huh forms):

repo URL/local path

identity path

branch + push mode

History picker (record history):

fuzzy search + multi-select checklist

preview pane

Workflow runner (run):

left: step list with status

right: current step command + output viewport

bottom: keybindings help (bubbles/help)

Conflict resolver (sync):

file list

diff viewer (viewport)

actions: ours/theirs/manual

Workflow editor:

reorder steps

edit command text

mark placeholder tokens

charm.sh packages to use

bubbletea (app)

bubbles (list, table, viewport, textinput, help, spinner)

lipgloss (layout/styles)

glamour (render Markdown in terminal)

huh (forms/wizards)

Optional: charmbracelet/log or your own logger

8) Security, Privacy, and Compliance

No automatic exfiltration: workflows live in Git; any AI use is opt-in with clear disclosure.

Secrets handling:

discourage secrets in workflows; support <secret> placeholders

optional integration with pass, 1Password CLI, aws ssm, etc. (post-MVP)

Repo boundary safety:

identity path must not allow path traversal

avoid following symlinks when writing

Execution safety:

before running each step, show command and require confirmation unless --yes

“dangerous command” heuristics optional (rm -rf, dd, mkfs…)

9) Performance Requirements

Indexing/search: < 100ms for ~1k workflows locally (build an index.json cache).

sync: incremental; show progress.

Runner output: stream output with backpressure; keep last N lines in memory.

10) Open Design Questions (worth settling early)

Single shared branch vs PR-based contributions
Recommendation: support both, default to PR mode for teams.

What is “the unit of sharing”?
Likely a workflow folder; Git URL is the “share link.”

Where do run logs live?
Keep logs local by default (privacy), optionally commit run summaries to a runs/ directory if desired.

Placeholder typing and validation
Start with strings + regex; add types later.

Schema and compatibility guarantees
Version the schema and add migrators.

11) MVP Scope (first shippable)
Must-have

init, sync, record history, record, run, placeholders, export md, list/search/view

Git commit on save; push optional

Solid Bubble Tea runner UX

Should-have

Conflict resolver UI

ask with pluggable provider + explicit consent

Could-have

explain

PR helper (auto-branch naming, integrates with gh)

12) Milestones

M0: Repo + config + identity path + workflow schema + save/commit

M1: record history → workflow → run

M2: Placeholders + export md

M3: sync + indexing + conflict handling

M4: record subshell capture

M5: ask provider plugin + privacy UX

M6: polish + packaging + upgrade command

If you want, next I can turn this PRD into:

a concrete CLI spec (exact flags, config file schema),

a repo layout + Go package architecture (internal/modules),

and a Bubble Tea screen map with message/update/model breakdowns—so you can start implementing immediately.

---

### **You**

yes please add those

---

### **ChatGPT**

Awesome — here’s a concrete add-on to the PRD: (A) CLI spec, (B) config file schema, (C) repo layout + Go package architecture, and (D) Bubble Tea screen map (models/messages/keybindings). I’m going to keep it implementable: if you start coding from this, you won’t have to invent much.

A) CLI spec (commands, flags, exit codes)
Global conventions

Binary: gitsavvy (placeholder name)

Default config: ~/.config/gitsavvy/config.toml

Default local repo checkout: ~/.local/share/gitsavvy/repo (configurable)

All commands support:

--config <path>

--repo <path> (override repo location)

--no-tui (force plain output)

--json (machine output where meaningful)

--verbose / --quiet

gitsavvy init
Initialize configuration and (optionally) clone repo.
Flags:

--remote <url>: remote Git URL to clone

--local <path>: use existing local repo instead of clone

--branch <name>: default branch (auto-detect if omitted)

--identity <path>: claimed path in repo (e.g., chaz or platform/chaz)

--mode <direct|pr>: write mode

--sign <true|false>: sign commits (best-effort)

--author-name <name> / --author-email <email>

--editor <cmd>: override editor

--shell <bash|zsh|sh|pwsh> default for steps

Behavior:

TUI wizard by default (huh).

Creates local config.

Validates identity path safety.

Optionally bootstraps repo structure (workflows/, .gitsavvy/).

Exit codes:

0 ok

2 invalid config/args

10 git failure

gitsavvy whoami
Print identity config: repo path, identity path, branch, mode, author.
Flags:

--json

gitsavvy status
Show git status + tool status.
Output includes:

dirty/clean

ahead/behind counts

last sync time

index freshness

Exit codes:

0 ok

11 repo missing/unreadable

gitsavvy sync
Fetch + integrate + reindex.
Flags:

--strategy <ff-only|rebase|merge> (default from config)

--remote <name> (default origin)

--branch <name> (default config)

--no-push (do not push local commits)

--push (push after successful integrate, if mode allows)

--conflicts <tui|ours|theirs|abort> default tui

--pr (in PR mode, ensure current feature branch is up-to-date with base)

Behavior:

git fetch

integrate according to strategy

handle conflicts

update index (.gitsavvy/index.json)

summarize changes

Exit codes:

0 ok

10 git failure

12 conflicts unresolved

gitsavvy record
Record commands from a subshell session and turn into a workflow.
Flags:

--title <string>

--desc <string>

--tags <csv>

--shell <bash|zsh|sh|pwsh> (override)

--cwd <path>

--identity <path> (override claim path)

--no-commit (still write files)

--draft (write under drafts/ instead of workflows/)

--from-log <path> (advanced: read captured session log)

Behavior:

spawn a subshell

capture executed commands

on exit, open TUI editor to select/reorder/rename steps

save workflow under identity path, commit (and maybe push/branch)

Exit codes:

0 ok

13 nothing captured / user canceled

10 git failure

gitsavvy record history
Pick commands from shell history.
Flags:

--shell <bash|zsh|pwsh>

--limit <n> (default 500)

--since <duration> (optional; best-effort)

--title, --desc, --tags, --identity, --draft, --no-commit

Behavior:

parse history file(s)

TUI multi-select

convert selected lines into steps

save + commit

Exit codes: same as record
gitsavvy list
List workflows.
Flags:

--mine (only under identity path)

--shared

--tag <tag> (repeatable)

--format <table|json|plain> default table

gitsavvy search
Fuzzy search (TUI by default).
Flags:

--query <string> (non-interactive)

--mine, --shared, --tag

--json

gitsavvy view <workflow-ref>
View details (markdown render if available).
workflow-ref may be:

slug my-workflow

path workflows/platform/chaz/my-workflow

id wf\_abc123 (if you choose stable IDs)

Flags:

--raw (print YAML)

--md (print markdown)

gitsavvy edit <workflow-ref>
Open in $EDITOR (or optional TUI editor).
Flags:

--tui

--no-commit

gitsavvy run [workflow-ref]
Run a workflow. If omitted, opens search/select UI.
Flags:

--local (don’t fetch; use current checkout)

--yes (auto-confirm steps)

--param key=value (repeatable; prefill placeholders)

--env key=value (repeatable; overlay env)

--cwd <path> (override)

--until <step-index|step-name>

--from <step-index|step-name>

--dry-run (show commands after substitution)

--no-tui (plain runner)

--log <path> (write run log)

--save-params <session|keychain|none> (default session)

Exit codes:

0 all steps succeeded

20 a step failed (and continue\_on\_error=false)

21 placeholder missing in non-interactive mode

13 canceled

gitsavvy export <workflow-ref>
Export workflow.
Flags:

--format <md|yaml|json> (default md)

--out <path> (default stdout)

--update-readme (write README.md into workflow folder)

gitsavvy ask
AI-assisted workflow/command generation (opt-in).
Flags:

--provider <name>

--model <name>

--api-key-env <ENVVAR> (don’t store key)

--prompt <string> (non-interactive)

--as <workflow|step> (default workflow)

--identity <path>

--no-commit

--redact <none|basic|strict> default basic

Exit codes:

0 generated (even if not saved)

30 provider not configured

31 provider error

gitsavvy explain
Explain a command or a workflow step.
Forms:

gitsavvy explain "<command>"

gitsavvy explain <workflow-ref> --step <idx|name>

Flags:

--provider, --model (if using AI)

--offline (rule-based explain only)

B) Config schema (TOML)
File: ~/.config/gitsavvy/config.toml

\`\`\`toml
tomlCopy code[repo]
path = "/Users/chaz/.local/share/gitsavvy/repo"
remote = "origin"
branch = "main"
sync\_strategy = "rebase" # ff-only|rebase|merge
auto\_reindex = true

[identity]
path = "platform/chaz"   # claimed write root inside repo
mode = "pr"              # direct|pr
team\_prefix = "platform" # optional helper for validation/UI

[git]
author\_name = "Chaz Straney"
author\_email = "chaz@example.com"
sign\_commits = false
push\_on\_save = false     # in direct mode only
pr\_base\_branch = "main"
feature\_branch\_template = "gitsavvy/{identity}/{date}/{slug}"

[workflows]
root = "workflows"       # repo-relative
shared\_root = "shared"   # repo-relative
draft\_root = "drafts"    # repo-relative
index\_path = ".gitsavvy/index.json"
schema\_version = 1

[runner]
default\_shell = "zsh"    # bash|zsh|sh|pwsh
confirm\_each\_step = true
stream\_output = true
max\_output\_lines = 5000
dangerous\_command\_warnings = true

[placeholders]
prompt\_style = "form"    # form|per-step
save\_defaults = "none"   # none|keychain|file
keychain\_service = "gitsavvy"

[tui]
enabled = true
theme = "default"
show\_help = true

[editor]
command = "vim"          # if unset, use $EDITOR

[ai]
enabled = false          # must be explicitly enabled
provider = "openai\_compat"
base\_url = "http://localhost:11434/v1" # optional for compat
model = "gpt-4o-mini"    # example
api\_key\_env = "OPENAI\_API\_KEY"
redact = "basic"         # none|basic|strict
confirm\_send = true
\`\`\`

Notes:

Keep secrets out of config (use env var, keychain, or provider-specific secure storage).

identity.path is the “claim” root; all workflow writes are under:

workflows/<identity.path>/<slug>/workflow.yaml

C) Repository layout and file formats
Repo layout (inside the Git repo)

\`\`\`
pgsqlCopy code.gitsavvy/
  index.json
  schema/
    workflow.v1.json          # optional JSON schema docs
workflows/
  platform/chaz/
    restart-service/
      workflow.yaml
      README.md
shared/
  incident-triage/
    workflow.yaml
drafts/
  platform/chaz/
    tmp-workflow/
      workflow.yaml
\`\`\`

Workflow file (YAML) – v1
workflow.yaml:

\`\`\`yaml
yamlCopy codeschema\_version: 1
id: wf\_01HZY3J9Y3G6Q9T3   # optional stable ID (ULID)
title: Restart service safely
description: Restart foo-service and verify health
tags: [prod, runbook]

defaults:
  shell: zsh
  cwd: .
  confirm\_each\_step: true

placeholders:
  service:
    prompt: "Service name"
    default: "foo-service"
    validate: "^[a-z0-9-]+$"
  namespace:
    prompt: "Kubernetes namespace"
    default: "default"

steps:
  - name: Check current pods
    command: "kubectl -n <namespace> get pods -l app=<service>"
    continue\_on\_error: false
    confirmation: true

  - name: Restart deployment
    command: "kubectl -n <namespace> rollout restart deploy/<service>"
    confirmation: true

  - name: Watch rollout
    command: "kubectl -n <namespace> rollout status deploy/<service>"
\`\`\`

Index format
.gitsavvy/index.json (generated, not hand-edited):

array of workflow metadata: id, title, tags, path, updated\_at, searchable text

used for fast search without walking the tree every time

D) Go project architecture (packages, boundaries)
Top-level repo structure (your Go code)

\`\`\`
makefileCopy codecmd/
  gitsavvy/
    main.go
internal/
  app/                 # root orchestrator: wires config, repo, index, UI
  cli/                 # cobra/urfavecli command definitions
  config/              # load/validate config TOML
  gitrepo/             # git ops: clone, fetch, status, commit, branch, conflicts
  workflows/           # read/write workflow model, validate, slugify, migrations
  index/               # build/search index; fuzzy matching
  runner/              # command execution, substitution, output streaming
  history/             # shell history readers/parsers
  recorder/            # subshell session capture and normalization
  placeholders/        # parse & prompt & substitute
  export/              # md/yaml/json export
  ai/                  # provider interface + redaction + prompt templates
  tui/                 # bubbletea models and shared components
pkg/
  types/               # (optional) public types if you want a library surface
\`\`\`

Key interfaces (so subsystems don’t tangle)
gitrepo.Repo

\`\`\`go
goCopy codetype Repo interface {
  Path() string
  Status(ctx context.Context) (Status, error)
  Fetch(ctx context.Context) error
  Integrate(ctx context.Context, strategy Strategy) (IntegrateResult, error)
  CheckoutBranch(ctx context.Context, branch string) error
  EnsureFeatureBranch(ctx context.Context, base, name string) error
  CommitAll(ctx context.Context, message string) (string, error)
  Push(ctx context.Context, remote, branch string) error
  Diff(ctx context.Context, a, b string, paths []string) (string, error)
}
\`\`\`

workflows.Store

\`\`\`go
goCopy codetype Store interface {
  List(ctx context.Context, filter Filter) ([]WorkflowRef, error)
  Load(ctx context.Context, ref WorkflowRef) (\*Workflow, error)
  Save(ctx context.Context, wf \*Workflow, opts SaveOptions) (WorkflowRef, error)
  Delete(ctx context.Context, ref WorkflowRef) error
}
\`\`\`

runner.Runner

\`\`\`go
goCopy codetype Runner interface {
  Run(ctx context.Context, plan Plan, sink OutputSink) (RunResult, error)
}
\`\`\`

ai.Provider

\`\`\`go
goCopy codetype Provider interface {
  Name() string
  GenerateWorkflow(ctx context.Context, prompt Prompt) (\*workflows.Workflow, error)
  Explain(ctx context.Context, input string) (string, error)
}
\`\`\`

This lets you:

swap out git implementation (go-git vs libgit2 vs shelling out) later

swap out AI providers cleanly

unit test everything with fakes

E) Bubble Tea screen map (models, messages, keybindings)
Shared TUI components

components/ (inside internal/tui/components)

FuzzyList (bubbles/list + filter)

MultiSelectList

ViewportWithHeader

KeyHelp (bubbles/help)

Form wrappers (huh)

DiffViewer

1) Init Wizard (init)
Model: InitModel

Steps (huh form pages):

Repo source: remote URL vs local path

Clone location / repo path

Default branch detection

Identity path claim

Mode direct/pr

Optional author/signing/editor

Summary + apply

Keybindings:

Enter next

Esc back

Ctrl+C cancel

Messages:

InitSubmittedMsg

RepoClonedMsg

ConfigWrittenMsg

InitErrorMsg

2) History Picker (record history)
Model: HistoryPickerModel

State:

items []HistoryLine

selected map[int]bool

filter query

preview viewport

Keybindings:

/ focus filter

Space toggle selection

a select all, n select none

Enter confirm selection → move to Workflow Editor

Esc cancel

Messages:

HistoryLoadedMsg

SelectionConfirmedMsg

3) Workflow Editor (used by record and record history)
Model: WorkflowEditorModel

Panels:

Step list (table)

Step detail editor (textinput/multiline)

Operations:

rename step

reorder (move up/down)

delete

detect placeholders & propose metadata

set title/desc/tags

Keybindings:

j/k move selection

Ctrl+u/Ctrl+d page

e edit step command

r rename step

d delete step

J/K move step down/up

p placeholder scan & edit

s save (writes files + commits unless --no-commit)

q quit (asks confirm if dirty)

Messages:

WorkflowDirtyMsg

WorkflowSavedMsg

CommitCreatedMsg

EditorErrorMsg

4) Workflow Search & Select (run, view, edit, export)
Model: WorkflowSearchModel

Uses index for fast fuzzy search

Preview shows README.md or step list

Keybindings:

type to filter

Enter select

Tab toggle mine/shared/all

Esc back / cancel

Messages:

IndexLoadedMsg

WorkflowSelectedMsg

5) Runner (run)
Model: RunnerModel

State machine:

PromptPlaceholdersState

ReadyToRunStepState

RunningStepState

StepResultState

FinishedState

Panels:

left: steps with status icons

right: current command + output viewport

bottom: help + placeholder summary

Keybindings:

Enter run/advance

s skip step

r rerun step

e edit current step (optional; opens editor sub-model)

p show placeholder values (and edit if allowed)

q quit run (confirm)

? toggle help

c copy current command to clipboard (optional, OS-dependent)

Runner messages:

PlaceholderNeededMsg{Key}

PlaceholderResolvedMsg{Key, Value}

ExecStartedMsg{StepIndex}

ExecOutputMsg{Line}

ExecFinishedMsg{ExitCode}

RunCanceledMsg

RunCompletedMsg

6) Sync + Conflict Resolver (sync)
Model: SyncModel

Shows progress: fetching, integrating, indexing

If conflicts:

transitions to ConflictResolverModel

Keybindings:

q cancel (if safe)

Enter acknowledge summary

ConflictResolverModel:

file list left, diff viewer right

actions:

o choose ours

t choose theirs

m manual edit in $EDITOR

a abort merge/rebase

Messages:

FetchDoneMsg

IntegrateResultMsg

ConflictFoundMsg

ConflictResolvedMsg

SyncCompleteMsg

F) A few implementation “gotchas” to decide early
Git library choice

Shell out to git initially is often simplest and most correct (auth, signing, LFS, config).

go-git is pure-Go but can lag in edge cases (auth, rebase behavior).
Recommendation: MVP shells out to git with good error capture; abstract behind gitrepo.Repo.

Capturing record subshell commands

Easiest cross-shell approach:

start subshell with environment variable that points to a temp “command log”

inject a shell-specific PROMPT\_COMMAND (bash) / precmd (zsh) hook to append last command

Don’t over-engineer: for v1, support bash+zsh reliably; pwsh later.

Placeholder parsing

Simple regex <[a-zA-Z\_][a-zA-Z0-9\_-]\*> works well.

Avoid parsing inside quotes complexity at first; just substitute raw strings (with optional shell-escape toggle later).

If you want the next “no-thinking-required” step, I can also generate:

a full Cobra command tree skeleton (files under cmd/gitsavvy/),

the workflow YAML structs + validation, and

a Bubble Tea runner model scaffold with the message types above.

---
---

## G) Testing Approach

### Philosophy

Git-savvy is a CLI tool with three distinct layers requiring different testing strategies:

1. **Core business logic** (workflows, placeholders, index) → unit tests with table-driven approach
2. **Git operations** → integration tests with temporary repositories
3. **TUI/UX** → golden file testing for Bubble Tea models

No external mocking framework required—use standard library `testing` package and lightweight fakes.

### Unit Testing (Core Logic)

**Scope**: Packages with pure functions and well-defined inputs/outputs:
- `internal/workflows` (validation, migrations, slug generation)
- `internal/placeholders` (parsing, substitution, validation)
- `internal/index` (building, searching, filtering)
- `internal/export` (YAML/JSON/Markdown generation)

**Pattern**: Table-driven tests for all public functions with edge cases:

```go
func TestPlaceholderSubstitution(t *testing.T) {
    tests := []struct {
        name     string
        template string
        values   map[string]string
        want     string
        wantErr  bool
    }{
        {
            name:     "single placeholder",
            template: "kubectl get pod <name>",
            values:   map[string]string{"name": "web-1"},
            want:     "kubectl get pod web-1",
        },
        {
            name:     "missing placeholder",
            template: "kubectl get pod <name>",
            values:   map[string]string{},
            wantErr:  true,
        },
        // ... more cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Substitute(tt.template, tt.values)
            if (err != nil) != tt.wantErr {
                t.Errorf("Substitute() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Substitute() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

**Coverage target**: ≥80% for core logic packages.

### Integration Testing (Git Operations)

**Scope**: `internal/gitrepo` package—must verify real Git behavior.

**Approach**: Create temporary Git repositories for each test, clean up via `t.Cleanup()`:

```go
func TestRepoCommitAll(t *testing.T) {
    // Create temp dir
    tmpDir := t.TempDir()
    
    // Initialize git repo
    repo, err := InitRepo(tmpDir, RepoOptions{
        InitOptions: git.InitOptions{
            DefaultBranch: "main",
        },
    })
    if err != nil {
        t.Fatal(err)
    }
    
    // Write test file
    testFile := filepath.Join(tmpDir, "test.txt")
    if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
        t.Fatal(err)
    }
    
    // Test commit
    commitHash, err := repo.CommitAll(ctx, "test commit")
    if err != nil {
        t.Fatalf("CommitAll() error = %v", err)
    }
    
    if commitHash == "" {
        t.Error("CommitAll() returned empty hash")
    }
    
    // Verify via git log
    // ... verification logic
}
```

**Test scenarios**:
- Clone from remote (use local file:// URL for test repo)
- Fetch, merge, rebase strategies
- Conflict detection
- Branch creation and checkout
- Commit with various git configs (user, signing)

### TUI Testing (Bubble Tea)

**Scope**: All Bubble Tea models in `internal/tui/`.

**Approach**: Golden file testing using bubbletea's standard pattern:

```go
func TestInitModel(t *testing.T) {
    // Update golden files with: go test ./internal/tui -update-golden
    update := len(os.Args) > 1 && os.Args[1] == "-update-golden"
    
    model := InitModel(InitOptions{
        ConfigPath: "/fake/config.toml",
    })
    
    // Simulate user input sequence
    msgs := []tea.Msg{
        tea.KeyMsg{Type: tea.KeyEnter},
        tea.KeyMsg{Type: tea.KeyEnter},
        // ... more input
    }
    
    var buf bytes.Buffer
    var fm *fumes.Fumes // or use bubbletea test utilities
    
    for _, msg := range msgs {
        model, _ = model.Update(msg)
    }
    
    // Compare view output
    got := model.View()
    
    goldenFile := filepath.Join("testdata", "init_model.golden")
    if update {
        os.WriteFile(goldenFile, []byte(got), 0644)
    }
    
    want, _ := os.ReadFile(goldenFile)
    if got != string(want) {
        t.Errorf("View() mismatch\nGot:  %s\nWant: %s", got, want)
    }
}
```

**Store golden files** in `testdata/` directories adjacent to test files.

### Fake Implementations

For packages that depend on external systems (AI providers, keychain), define interfaces and provide test fakes:

```go
// internal/ai/fake.go
type FakeProvider struct {
    GenerateWorkflowFunc func(ctx context.Context, prompt Prompt) (*workflows.Workflow, error)
    ExplainFunc          func(ctx context.Context, input string) (string, error)
}

func (f *FakeProvider) Name() string { return "fake" }

func (f *FakeProvider) GenerateWorkflow(ctx context.Context, prompt Prompt) (*workflows.Workflow, error) {
    if f.GenerateWorkflowFunc != nil {
        return f.GenerateWorkflowFunc(ctx, prompt)
    }
    return &workflows.Workflow{Title: "fake workflow"}, nil
}

func (f *FakeProvider) Explain(ctx context.Context, input string) (string, error) {
    if f.ExplainFunc != nil {
        return f.ExplainFunc(ctx, input)
    }
    return "fake explanation", nil
}
```

### Test Data & Fixtures

Place reusable test workflows in `testdata/workflows/`:
- `minimal.yaml`: Smallest valid workflow
- `with_placeholders.yaml`: Workflow with all placeholder features
- `multi_step.yaml`: Complex workflow for runner testing
- `invalid_v*.yaml`: Samples for migration testing

### CI/CD Requirements

- Run unit tests on all commits: `go test ./...`
- Run integration tests with race detection: `go test -race ./...`
- Golden file checks require explicit `-update-golden` flag
- Target test duration: <30 seconds for full suite

---

## H) Error Handling Principles

### Error Type Hierarchy

Define a small set of error types for consistent handling:

```go
// internal/errors/errors.go
package errors

import (
    "errors"
    "fmt"
)

// Base error types—check with errors.Is()
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrInvalid       = errors.New("invalid")
    ErrConflict      = errors.New("conflict")
    ErrGit           = errors.New("git operation failed")
    ErrIO            = errors.New("I/O error")
    ErrCanceled      = errors.New("operation canceled")
)

// Wrapped errors provide context
type WorkflowError struct {
    Op  string // Operation: "load", "save", "validate"
    Err error  // Underlying error
    ID  string // Workflow ID or path (if applicable)
}

func (e *WorkflowError) Error() string {
    if e.ID != "" {
        return fmt.Sprintf("workflow %s: %s: %v", e.ID, e.Op, e.Err)
    }
    return fmt.Sprintf("workflow: %s: %v", e.Op, e.Err)
}

func (e *WorkflowError) Unwrap() error { return e.Err }

type GitError struct {
    Op  string // "fetch", "push", "commit", "merge"
    Err error
    Cmd string // Git command that failed (for debugging)
}

func (e *GitError) Error() string {
    return fmt.Sprintf("git %s: %v", e.Op, e.Err)
}

func (e *GitError) Unwrap() error { return e.Err }
```

### Wrap-and-Annotate Pattern

Always add context when returning errors:

```go
func (s *Store) Load(ctx context.Context, ref WorkflowRef) (*Workflow, error) {
    path := s.resolvePath(ref)
    
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("%w: %s", ErrNotFound, ref)
        }
        return nil, &WorkflowError{Op: "load", Err: err, ID: ref.String()}
    }
    
    var wf Workflow
    if err := yaml.Unmarshal(data, &wf); err != nil {
        return nil, &WorkflowError{Op: "parse", Err: err, ID: ref.String()}
    }
    
    return &wf, nil
}
```

### Retry Logic

Only retry transient failures with clear backoff:

```go
// internal/gitrepo/fetch.go
func (r *repo) Fetch(ctx context.Context) error {
    const maxRetries = 3
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        if err := r.fetchOnce(ctx); err == nil {
            return nil
        } else if isTransientError(err) {
            lastErr = err
            time.Sleep(backoff(i))
            continue
        } else {
            return err // Non-transient—fail immediately
        }
    }
    
    return fmt.Errorf("fetch failed after %d retries: %w", maxRetries, lastErr)
}

func isTransientError(err error) bool {
    // Network timeouts, TLS handshake failures, etc.
    return strings.Contains(err.Error(), "connection reset") ||
           strings.Contains(err.Error(), "timeout")
}
```

Do NOT retry:
- Validation errors (won't change)
- Git conflicts (require manual resolution)
- File not found (won't appear by retrying)

### User-Facing Messages

Technical errors go to logs (with `--verbose`); users see actionable messages:

```go
func (c *CLI) RunWorkflow(ref string) error {
    wf, err := c.store.Load(ctx, ref)
    if err != nil {
        // User sees: "Workflow 'restart-service' not found"
        // Log shows: "workflow load: open /path/to/file.yaml: no such file or directory"
        if errors.Is(err, ErrNotFound) {
            return fmt.Errorf("workflow %q not found (run 'gitsavvy list' to see available)", ref)
        }
        return fmt.Errorf("failed to load workflow: %w", err)
    }
    // ... rest of logic
}
```

### Exit Code Mapping

Commands map errors to exit codes consistently:

```go
// internal/cli/run.go
func (cmd *RunCmd) Run(ctx context.Context) error {
    wf, err := cmd.loadWorkflow()
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return CLIError{Code: 1, UserMsg: "workflow not found"} // Generic failure
        }
        return CLIError{Code: 11, UserMsg: "repository not initialized"} // Repo missing
    }
    
    result, err := cmd.runner.Run(ctx, wf)
    if err != nil {
        if errors.Is(err, ErrCanceled) {
            return CLIError{Code: 13, UserMsg: "run canceled"} // Canceled
        }
        return CLIError{Code: 20, UserMsg: fmt.Sprintf("step %d failed", result.FailedStep)}
    }
    
    return nil
}

type CLIError struct {
    Code    int
    UserMsg string
}

func (e CLIError) Error() string { return e.UserMsg }
```

### Validation Errors

Collect multiple validation errors before failing:

```go
type ValidationErrors []error

func (ve ValidationErrors) Error() string {
    var sb strings.Builder
    sb.WriteString("validation failed:\n")
    for _, err := range ve {
        sb.WriteString(fmt.Sprintf("  - %s\n", err))
    }
    return sb.String()
}

func (w *Workflow) Validate() error {
    var errs ValidationErrors
    
    if w.Title == "" {
        errs = append(errs, fmt.Errorf("title is required"))
    }
    if len(w.Steps) == 0 {
        errs = append(errs, fmt.Errorf("at least one step required"))
    }
    for i, step := range w.Steps {
        if step.Command == "" {
            errs = append(errs, fmt.Errorf("step %d: command is required", i))
        }
    }
    
    if len(errs) > 0 {
        return errs
    }
    return nil
}
```

---

## I) State Management

### Run State Persistence

Interrupted runs must be recoverable. Persist run state to `~/.local/share/gitsavvy/runs/<run-id>.json`:

```go
// internal/runner/state.go
type RunState struct {
    RunID       string              `json:"run_id"`
    WorkflowRef string              `json:"workflow_ref"`
    StartedAt   time.Time           `json:"started_at"`
    UpdatedAt   time.Time           `json:"updated_at"`
    Status      RunStatus           `json:"status"` // running, paused, completed, failed
    CurrentStep int                 `json:"current_step"`
    Placeholders map[string]string  `json:"placeholders"`
    StepResults []StepResult        `json:"step_results"`
    LogPath     string              `json:"log_path"` // Output file path
}

type RunStatus string

const (
    RunStatusRunning   RunStatus = "running"
    RunStatusPaused    RunStatus = "paused"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed    RunStatus = "failed"
)
```

**Save triggers**:
- Before each step execution
- On Ctrl+C (interrupt)
- On placeholder resolution
- After step completion

**Recovery flow**:
```go
func (r *runner) Run(ctx context.Context, plan Plan) (RunResult, error) {
    // Check for existing state
    if state, err := r.loadState(plan.RunID); err == nil {
        if state.Status == RunStatusPaused {
            return r.resumeRun(ctx, state, plan)
        }
    }
    
    // Fresh run
    return r.startRun(ctx, plan)
}
```

User can resume with: `gitsavvy run --resume <run-id>` or interactive prompt on next run.

### Dirty Workflow Detection

Editor must detect uncommitted changes and prompt user:

```go
// internal/workflows/dirty.go
func (s *Store) Save(ctx context.Context, wf *Workflow, opts SaveOptions) error {
    // Check if workflow file exists with uncommitted changes
    existingPath := s.resolvePath(wf.Ref)
    
    if dirty, err := s.isDirty(ctx, existingPath); err == nil && dirty {
        if !opts.Force {
            return fmt.Errorf("workflow has uncommitted changes (use --force to overwrite)")
        }
    }
    
    // ... save logic
}

func (s *Store) isDirty(ctx context.Context, path string) (bool, error) {
    // Check git status for file
    status, err := s.git.Status(ctx, WithPath(path))
    if err != nil {
        return false, err
    }
    
    // File is dirty if modified but not staged
    for _, entry := range status.Entries {
        if entry.Path == path && entry.Worktree != git.Unmodified {
            return true, nil
        }
    }
    return false, nil
}
```

### Interrupt Handling

Graceful shutdown on signals:

```go
// internal/runner/runner.go
func (r *runner) Run(ctx context.Context, plan Plan) (RunResult, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // Listen for interrupt signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    defer signal.Stop(sigChan)
    
    for i, step := range plan.Steps {
        select {
        case <-sigChan:
            // Save state for resume
            r.saveState(RunState{
                Status:      RunStatusPaused,
                CurrentStep: i,
                // ... other fields
            })
            return RunResult{Canceled: true}, ErrCanceled
            
        case <-ctx.Done():
            return RunResult{}, ctx.Err()
            
        default:
            // Execute step
            result := r.executeStep(ctx, step)
            r.saveState(...) // Update after each step
        }
    }
}
```

### Placeholder Value Persistence

Per-workflow placeholder defaults can be cached locally (not in Git):

```go
// Stored in ~/.local/share/gitsavvy/params/<workflow-id>.json
type ParamCache struct {
    WorkflowID string            `json:"workflow_id"`
    Values     map[string]string `json:"values"`
    UpdatedAt  time.Time         `json:"updated_at"`
}
```

Configured by `save_params` option:
- `none`: Never save (default)
- `session`: Save in memory only (cleared on exit)
- `file`: Persist to local JSON file
- `keychain`: Use OS keychain (future)

### Sync State Tracking

Track last sync to avoid redundant operations:

```go
// Stored in .gitsavvy/state.json (in repo, gitignored)
type RepoState struct {
    LastSyncAt   time.Time              `json:"last_sync_at"`
    LastSyncHash string                  `json:"last_sync_hash"` // HEAD commit hash
    IndexHash    string                  `json:"index_hash"`     // Hash of indexed workflows
    Branch       string                  `json:"branch"`
}

func (s *Syncer) NeedsSync(ctx context.Context) (bool, error) {
    state, err := s.loadRepoState()
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return true, nil // Never synced
        }
        return false, err
    }
    
    // Fetch without integrating
    if err := s.git.Fetch(ctx, WithDryRun(true)); err != nil {
        return false, err
    }
    
    // Compare remote HEAD with last sync hash
    remoteHEAD, _ := s.git.ResolveRef(ctx, "origin/main")
    return remoteHEAD != state.LastSyncHash, nil
}
```

### Cleanup Strategy

Old run states and param caches should be cleaned up:

```go
// internal/app/cleanup.go
func (a *App) Cleanup(ctx context.Context) error {
    // Remove run states older than 30 days
    cutoff := time.Now().AddDate(0, 0, -30)
    
    runsDir := filepath.Join(a.dataDir, "runs")
    entries, _ := os.ReadDir(runsDir)
    
    for _, entry := range entries {
        info, _ := entry.Info()
        if info.ModTime().Before(cutoff) {
            os.Remove(filepath.Join(runsDir, entry.Name()))
        }
    }
    
    // Cleanup is run automatically on:
    // - gitsavvy sync (after successful sync)
    // - gitsavvy init (on startup)
    return nil
}
```

---


---

## J) Shell Capture Specification (record command)

### Overview

The `record` command launches a subshell with injected hooks that capture executed commands to a temporary log file. When the user exits the subshell, commands are parsed and presented in a TUI editor for workflow creation.

### Supported Shells (MVP)

| Shell | Detection | Status | Notes |
|-------|-----------|--------|-------|
| bash  | `$SHELL`, `basename` | ✅ MVP | PROMPT_COMMAND hook |
| zsh   | `$SHELL`, `basename` | ✅ MVP | precmd_function hook |
| sh    | fallback | ⚠️ limited | No hooks, manual capture |
| pwsh  | `$SHELL`, `pwsh` | ❌ v2+ | Different PSReadLine API |

**MVP scope**: bash + zsh only. Explicit error for unsupported shells.

### Architecture

```
gitsavvy record
  │
  ├─> Create temp capture file (/tmp/gitsavvy-record-<uuid>.log)
  │
  ├─> Inject shell hooks via --rcfile or temporary init script
  │   └─> Hook runs after each command, appends to capture file
  │
  ├─> Spawn subshell with modified environment
  │   └─> GITSAVVY_CAPTURE_FILE=/tmp/gitsavvy-record-<uuid>.log
  │   └─> GITSAVVY_SESSION_ID=<uuid>
  │   └─> PS1 mod (indicator we're recording)
  │
  └─> Wait for shell exit
      │
      └─> Read capture file
          └─> Parse, normalize, present in TUI
```

### Capture File Format

Simple line-oriented format:

```
<timestamp>|<cwd>|<command>
<timestamp>|<cwd>|<command with|literal pipes|and special chars>
```

Example:
```
1738301234|/Users/chaz/projects/svc|kubectl get pods
1738301245|/Users/chaz/projects/svc|kubectl delete pod web-1
1738301250|/Users/chaz/projects/svc|vim config.yaml
1738301300|/Users/chaz/projects/svc|git status
```

**Delimiter**: `|` is unlikely but not impossible in commands. Use `\x1F` (unit separator) if needed.

**Timestamp**: Unix epoch (seconds). Enables ordering and deduplication.

**CWD**: Current working directory for context (useful for workflows with `cwd` steps).

### Shell Hook Implementation

#### Bash (PROMPT_COMMAND)

```bash
# Injected via bash --rcfile <(generate_init_script)
# or: bash -c 'source <(generate_init_script); exec bash'

GITSAVVY_CAPTURE_FILE="/tmp/gitsavvy-record-<uuid>.log"

_gitsavvy_capture() {
    local cmd="$BASH_COMMAND"
    local cwd="$(pwd)"
    local ts="$(date +%s)"
    
    # Skip empty commands, duplicates, built-ins
    [[ -z "$cmd" ]] && return
    [[ "$cmd" == "$_GITSAVVY_LAST_CMD" ]] && return
    
    # Built-ins to skip (navigation, job control, etc.)
    case "$cmd" in
        cd|pushd|popd|dirs|pwd|ls|la|ll|clear|history|exit|logout) return ;;
    esac
    
    echo "${ts}|${cwd}|${cmd}" >> "$GITSAVVY_CAPTURE_FILE"
    _GITSAVVY_LAST_CMD="$cmd"
}

# Store command before execution
trap '_gitsavvy_capture' DEBUG
```

**Why `trap DEBUG` instead of `PROMPT_COMMAND`?**
- `PROMPT_COMMAND` runs BEFORE the prompt (after command completes)
- `trap DEBUG` runs AFTER command completes, with access to `$BASH_COMMAND`
- Actually, for capture we want PROMPT_COMMAND - it has the full command in history

**Corrected bash approach:**

```bash
GITSAVVY_CAPTURE_FILE="/tmp/gitsavvy-record-<uuid>.log"
GITSAVVY_LAST_LINE=""

_gitsavvy_prompt_command() {
    local cmd=$(history 1 | sed 's/^ *[0-9]* *//')
    local cwd="$(pwd)"
    local ts="$(date +%s)"
    
    # Skip empty, duplicates, built-ins
    [[ -z "$cmd" ]] && return
    [[ "$cmd" == "$GITSAVVY_LAST_LINE" ]] && return
    
    # Built-ins to skip
    case "$cmd" in
        cd|pushd|popd|dirs|pwd|ls|la|ll|clear|history|exit|logout|jobs|fg|bg) return ;;
    esac
    
    echo "${ts}|${cwd}|${cmd}" >> "$GITSAVVY_CAPTURE_FILE"
    GITSAVVY_LAST_LINE="$cmd"
}

# Hook into prompt
PROMPT_COMMAND="_gitsavvy_prompt_command"
```

#### Zsh (precmd)

```zsh
GITSAVVY_CAPTURE_FILE="/tmp/gitsavvy-record-<uuid>.log"
GITSAVVY_LAST_LINE=""

_gitsavvy_precmd() {
    local cmd="$history[$((HISTCMD-1))]"
    local cwd="$(pwd)"
    local ts="$(date +%s)"
    
    # Skip empty, duplicates, built-ins
    [[ -z "$cmd" ]] && return
    [[ "$cmd" == "$GITSAVVY_LAST_LINE" ]] && return
    
    # Built-ins to skip
    case "$cmd" in
        cd|pushd|popd|dirs|pwd|ls|la|ll|clear|history|exit|logout|jobs|fg|bg) return ;;
    esac
    
    echo "${ts}|${cwd}|${cmd}" >> "$GITSAVVY_CAPTURE_FILE"
    GITSAVVY_LAST_LINE="$cmd"
}

# Hook into zsh
precmd_functions+=(_gitsavvy_precmd)
```

### Shell Invocation

#### Detecting User's Shell

```go
func detectShell() string {
    // 1. Check SHELL env var
    if shell := os.Getenv("SHELL"); shell != "" {
        return filepath.Base(shell)
    }
    
    // 2. Fallback to /etc/passwd entry
    if usr, err := user.Current(); err == nil {
        return filepath.Base(usr.Shell)
    }
    
    // 3. Default to bash
    return "bash"
}
```

#### Spawning the Recorded Shell

```go
func (r *Recorder) StartRecordingSession(ctx context.Context) error {
    // 1. Create temp capture file
    captureFile, err := os.CreateTemp("", "gitsavvy-record-*.log")
    if err != nil {
        return err
    }
    defer captureFile.Close()
    
    // 2. Generate shell init script
    shell := detectShell()
    initScript, err := r.generateInitScript(shell, captureFile.Name())
    if err != nil {
        return err
    }
    
    // 3. Build command with appropriate flags
    cmd := exec.CommandContext(ctx, shell, r.getShellFlags(shell, initScript)...)
    
    // 4. Set environment
    cmd.Env = append(os.Environ(),
        "GITSAVVY_CAPTURE_FILE="+captureFile.Name(),
        "GITSAVVY_SESSION_ID="+uuid.New().String(),
        "PS1="+r.modifiedPrompt(), // Add indicator
    )
    
    // 5. Attach stdio for interactive use
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    // 6. Run shell (blocks until exit)
    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            // User exited normally with some code
            return r.parseCaptureFile(captureFile.Name(), exitErr.ExitCode())
        }
        return err
    }
    
    return r.parseCaptureFile(captureFile.Name(), 0)
}

func (r *Recorder) getShellFlags(shell, initScript string) []string {
    switch shell {
    case "bash":
        // Use --rcfile to inject our hooks
        return []string{"--rcfile", initScript, "-i"}
    case "zsh":
        // Zsh doesn't have --rcfile equivalent for interactive mode
        // Use: zsh -i -c 'source /init/script; exec zsh'
        return []string{"-i", "-c", fmt.Sprintf("source %s; exec zsh", initScript)}
    default:
        return []string{"-i"}
    }
}

func (r *Recorder) modifiedPrompt() string {
    // Add visual indicator we're recording
    basePrompt := os.Getenv("PS1")
    if basePrompt == "" {
        basePrompt = "\\$ " // Fallback
    }
    return "[REC] " + basePrompt
}
```

### Command Parsing & Normalization

After shell exits, parse the capture file:

```go
type CapturedCommand struct {
    Timestamp int64     `json:"timestamp"`
    CWD       string    `json:"cwd"`
    Command   string    `json:"command"`
}

func (r *Recorder) parseCaptureFile(path string, exitCode int) ([]CapturedCommand, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer os.Remove(path) // Cleanup temp file
    
    var commands []CapturedCommand
    scanner := bufio.NewScanner(file)
    seen := make(map[string]bool) // Deduplicate
    
    for scanner.Scan() {
        line := scanner.Text()
        parts := strings.SplitN(line, "|", 3)
        if len(parts) != 3 {
            continue // Skip malformed lines
        }
        
        ts, _ := strconv.ParseInt(parts[0], 10, 64)
        cwd := parts[1]
        cmd := parts[2]
        
        // Deduplicate
        key := fmt.Sprintf("%s:%s", cwd, cmd)
        if seen[key] {
            continue
        }
        seen[key] = true
        
        commands = append(commands, CapturedCommand{
            Timestamp: ts,
            CWD:       cwd,
            Command:   cmd,
        })
    }
    
    // Sort by timestamp
    sort.Slice(commands, func(i, j int) bool {
        return commands[i].Timestamp < commands[j].Timestamp
    })
    
    if len(commands) == 0 {
        return nil, fmt.Errorf("no commands captured (exit code: %d)", exitCode)
    }
    
    return commands, nil
}
```

### Alias Expansion

The PRD mentions alias expansion. Implement after capture:

```go
func (r *Recorder) expandAliases(shell string, cmd string) (string, error) {
    // Get alias definitions from current shell
    var aliasCmd string
    switch shell {
    case "bash":
        aliasCmd = "alias"
    case "zsh":
        aliasCmd = "alias -L"  // zsh uses -L for list in define form
    }
    
    out, err := exec.Command(shell, "-c", aliasCmd).Output()
    if err != nil {
        return cmd, err // Return original if expansion fails
    }
    
    // Parse alias definitions and build map
    aliases := parseAliasOutput(string(out))
    
    // Simple prefix expansion (doesn't handle all edge cases)
    for name, expansion := range aliases {
        if strings.HasPrefix(cmd, name+" ") || cmd == name {
            return strings.Replace(cmd, name, expansion, 1), nil
        }
    }
    
    return cmd, nil
}
```

**Limitation**: This only handles simple `alias ll='ls -la'` cases. Complex aliases with arguments, functions, and completions are not expandable without executing the shell.

### Edge Cases

#### Multi-line Commands

Commands with line continuations (`\`) or heredocs appear as multiple lines in history.

**Strategy**: Reconstruct multi-line commands by:
1. Tracking line continuation in the shell hook (complex)
2. Post-processing: merge lines ending with `\`
3. MVP: Accept multi-line commands as separate steps

**MVP decision**: Treat as separate steps. User can merge in TUI editor.

#### Commands with Special Characters

Pipes, redirects, quotes, semicolons:

```bash
cat file.txt | grep pattern > output.txt & 
```

All captured as a single line (shell normalizes). No special handling needed.

#### Empty Shell Session

User types `exit` immediately or runs no commands.

**Behavior**: 
- Detect 0 commands in capture file
- Show message: "No commands captured. Shell exited with code <code>."
- Exit with code 13 (user canceled / nothing captured)

#### Hook Failure

Shell fails to load init script or hooks error.

**Detection**:
- Set a marker file: `/tmp/gitsavvy-ready-<uuid>`
- Init script writes this on successful load
- If missing on exit, warn user and offer options

**Fallback**: Offer `record history` as alternative.

#### Shell Not Installed

Requested shell isn't available on system.

**Detection**: `exec.LookPath(shell)` before spawning

**Behavior**: 
```bash
Error: /bin/zsh not found. Available shells: /bin/bash, /bin/sh
Use --shell bash to override.
```

### TUI Integration

After capture and parsing, launch the Workflow Editor (see section E) with:

- Pre-populated steps from captured commands
- Timestamp info in metadata (for sorting)
- CWD per step (if it varies)

### Testing Strategy

Unit tests for:
- `parseCaptureFile()` with various inputs
- `expandAliases()` with mock alias output
- `detectShell()` with mocked env

Integration tests:
- Spawn a subshell with our init script
- Execute commands programmatically via stdin
- Verify capture file contents
- Clean up subshell

**Test fixture**: `testdata/capture/` with sample capture files.

### Future Enhancements (Post-MVP)

1. **Fish shell**: Use `fish_prompt` function
2. **PowerShell**: Use `PSReadLine` history API
3. **Function expansion**: Expand shell functions, not just aliases
4. **Session replay**: Save full session with timing for demo workflows
5. **Environment capture**: Capture env var changes between steps

---

## K) Design Decisions Based on Savvy CLI Research

The following open questions were researched by analyzing Savvy CLI's implementation and behavior. This section documents the decisions made for the Git-backed implementation.

### Summary of Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Placeholder validation timing | Validate at workflow load time | Savvy validates at runtime, not creation |
| Workflow ID generation | Optional ULIDs in YAML metadata | File paths are primary; ULIDs provide rename safety |
| Index update strategy | Rebuild on sync and manual `--reindex` | Savvy uses single binary file; we use rebuildable JSON |
| Concurrent editing | Rely on Git merge conflict resolution | Savvy has cloud-only editing; Git's model is sufficient |
| Secret placeholder masking | Skip for MVP - manual redaction only | Savvy has no automatic secret masking |
| Shell history parsing delimiter | Use `\x1F` (unit separator) | Savvy reads native formats; `|` conflicts with commands |
| Capture file format | `\x1F` delimiter for safety | Avoids conflicts with literal pipes in commands |
| Run state persistence | Skip for MVP | Savvy doesn't have it; interactive model doesn't require it |
| AI redaction levels | Skip for MVP - manual redaction only | Savvy has no automatic redaction levels |
| PR mode branch naming | Auto-generate from template, prompt if exists | Savvy has no PR integration |
| Export templates | Support custom template files | Improvement over Savvy's hard-coded approach |
| `run --local` behavior | Use Git checkout, error if not found | Same as Savvy's local cache approach |
| Step `cwd` resolution | Support per-step, relative to repo root | Savvy doesn't support this; improvement |
| `continue_on_error` | Skip for MVP interactive mode | Savvy has no automatic error handling |
| Drafts directory | Gitignored, local-only | Savvy has no drafts concept |
| Shared/ permissions | Convention only, optional CODEOWNERS | Savvy uses cloud permissions |

---

### K.1 Placeholder Validation Timing

**Decision**: Validate placeholder syntax at workflow load time, not during creation.

**Savvy's approach:**
- Validation occurs at runtime when the runbook loads
- Uses regex `<([a-zA-Z0-9-_]+)>` to extract placeholders
- No validation at creation time - placeholders are manually inserted via TUI

**Implementation:**
```go
// internal/placeholders/extract.go
var placeholderRegex = regexp.MustCompile(`<([a-zA-Z0-9-_]+)>`)

func Extract(input string) []string {
    matches := placeholderRegex.FindAllStringSubmatch(input, -1)
    seen := make(map[string]bool)
    var result []string
    for _, m := range matches {
        if !seen[m[1]] {
            seen[m[1]] = true
            result = append(result, m[1])
        }
    }
    return result
}
```

**UX implications:**
- Users can freely insert placeholders during workflow editing
- Syntax errors are caught when workflow is loaded for running
- Error message: "Invalid placeholder '<invalid-name>' in step 'X'"

---

### K.2 Workflow ID Generation

**Decision**: ULIDs are **optional** in the schema. File paths are the primary reference.

**Savvy's approach:**
- Server-assigned stable IDs with `rb-` prefix
- IDs are immutable and assigned once by cloud backend
- Step IDs use local generator with prefixes (`cmd-`, `f-`, `llm-`)

**Schema:**
```yaml
schema_version: 1
id: wf_01HZY3J9Y3G6Q9T3   # OPTIONAL - provides rename safety
title: Restart service safely
```

**When to use ULIDs:**
- Generated on workflow creation if not present
- Preserved when workflow is renamed (file path changes)
- Used as fallback identifier if file path lookup fails

**File path as primary ID:**
- `workflows/platform/chaz/restart-service/workflow.yaml`
- Slugified from title for new workflows
- Can be renamed by moving the directory

---

### K.3 Index Update Strategy

**Decision**: Rebuild `.gitsavvy/index.json` on `sync` and manual `--reindex`. Index is gitignored.

**Savvy's approach:**
- Single gob-encoded binary file at `~/.config/savvy/savvy.local`
- In-memory map, no separate index
- Search loads entire map into memory

**Index location:** `.gitsavvy/index.json` (gitignored)

**When to rebuild:**
```go
// internal/index/builder.go
func (b *Builder) NeedsRebuild(ctx context.Context) bool {
    // Rebuild if:
    // 1. Index doesn't exist
    // 2. Explicitly requested (--reindex flag)
    // 3. After successful sync
    // 4. Index is stale (older than any workflow file)
}
```

**Index format:**
```json
{
  "version": 1,
  "updated_at": "2026-01-31T12:00:00Z",
  "workflows": [
    {
      "id": "wf_01HZY3J9Y3G6Q9T3",
      "title": "Restart service safely",
      "path": "workflows/platform/chaz/restart-service",
      "tags": ["prod", "kubernetes"],
      "updated_at": "2026-01-31T12:00:00Z"
    }
  ]
}
```

**Conflict handling:** Last sync wins (index is rebuildable from source files)

**Gitignore entry:**
```
# Local search index (auto-generated)
.gitsavvy/index.json
```

---

### K.4 Concurrent Workflow Editing

**Decision**: Rely on standard Git merge conflict resolution. No additional locking.

**Savvy's approach:**
- No concurrent editing - cloud-only editing via dashboard
- No PUT/PATCH endpoints - only POST to create new runbooks
- Server-side conflict resolution (proprietary)

**Git-native approach:**
- File-per-workflow design enables Git's diff/merge
- Users resolve conflicts via their preferred merge tool
- No special "semantic merge" needed - YAML is human-readable

**Example conflict scenario:**
```
<<<<<<< HEAD
steps:
  - name: Check pods
    command: kubectl get pods
  - name: Restart
    command: kubectl rollout restart
=======
steps:
  - name: Verify pods
    command: kubectl get pods -w
  - name: Rolling restart
    command: kubectl rollout restart
>>>>>>> feature/their-changes
```

User resolves in their editor or via `git mergetool`.

---

### K.5 Secret Placeholder Masking

**Decision**: Skip "secret" type for MVP. All placeholders treated uniformly.

**Savvy's approach:**
- No automatic secret masking
- Manual redaction via TUI only
- All placeholders treated uniformly (no "secret" type)
- Users manually replace sensitive values with `<placeholders>`

**MVP schema:**
```yaml
placeholders:
  api_token:
    prompt: "API token"
    # No "secret" field - all treated the same
```

**Future enhancement** (if requested):
```yaml
placeholders:
  api_token:
    prompt: "API token"
    secret: true  # Mask in TUI output, omit from logs
```

**Security note:** The safest approach is to never persist secrets - prompt for them each run.

---

### K.6 Shell History Parsing & Capture File Format

**Decision**: Use `\x1F` (unit separator) instead of `|` for capture file delimiter.

**Savvy's approach:**
- Bash: Reads `~/.bash_history` with timestamp format (`#1616420000`)
- Zsh: Reads `~/.zsh_history` with format (`: timestamp;command`)
- Fish: YAML-like format with `- cmd:` prefix
- Multi-line: Zsh reconstructs via escaped backslashes

**Problem with pipe delimiter:**
```
1738301245|/Users/chaz/projects/svc|cat file.txt | grep pattern
```
The literal `|` in the command breaks parsing.

**Solution: Use ASCII unit separator (`\x1F`)**
```
1738301245\x1F/Users/chaz/projects/svx1Fcat file.txt | grep pattern
```

**Capture file format:**
```
<timestamp>\x1F<cwd>\x1F<command>
<timestamp>\x1F<cwd>\x1F<command with | pipes & and special chars>
```

**Multi-line commands:** MVP treats as separate steps. Users can merge in TUI editor.

---

### K.7 Run State Persistence

**Decision**: Skip run state persistence for MVP.

**Savvy's approach:**
- No run state persistence
- State is in-memory only during single run session
- `RunServer` maintains `currIndex` in memory
- Unix socket communication (`/tmp/savvy-run.sock`)
- When run exits, all state is lost

**Key insight:** Savvy's model is interactive "wizard" - user controls each step. No automation means no need for complex state persistence.

**MVP approach:**
- Interactive runs don't need state persistence
- Sessions are short-lived
- User is always in control

**Future enhancement** (if there's demand):
```go
// internal/runner/state.go
type RunState struct {
    RunID       string              `json:"run_id"`
    WorkflowRef string              `json:"workflow_ref"`
    StartedAt   time.Time           `json:"started_at"`
    UpdatedAt   time.Time           `json:"updated_at"`
    Status      RunStatus           `json:"status"` // running, paused, completed, failed
    CurrentStep int                 `json:"current_step"`
    Placeholders map[string]string  `json:"placeholders"`
    StepResults []StepResult        `json:"step_results"`
    LogPath     string              `json:"log_path"`
}

// Stored in: ~/.local/share/gitsavvy/runs/<run-id>.json
// Cleanup: Delete states older than 30 days
```

---

### K.8 AI Redaction Levels

**Decision**: Skip redaction levels for MVP. Manual redaction only.

**Savvy's approach:**
- No automatic redaction levels ("basic", "strict", etc.)
- Users manually edit commands via `huh` TUI form before sending to AI
- Description: "Replace sensitive data with `<REDACTED>`. To remove a command, simply delete the text."
- Happens locally before data leaves the machine

**MVP approach:**
- Manual redaction TUI is sufficient
- Follow Savvy's simple model

**Future enhancement** (if requested):

**Redaction levels:**
- **none**: No automatic redaction
- **basic**: Redact common patterns (API keys, tokens, emails)
- **strict**: Basic + file paths, hostnames, project-specific terms

---

### K.9 PR Mode Branch Naming

**Decision**: Auto-generate from template, prompt user if branch exists.

**Savvy's approach:**
- No PR integration - cloud-first architecture
- No Git commands, branch management, or PR workflows

**Template format:**
```toml
[git]
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"
```

**Generated example:**
```
gitsavvy/platform/chaz/2026-01-31/restart-service
```

**If branch exists:** Prompt user with options
```
Branch 'gitsavvy/platform/chaz/2026-01-31/restart-service' already exists.

Options:
  1) Reuse - Continue working on existing branch
  2) Create new - Create with timestamp suffix
  3) Fail - Exit and let you handle manually

Choice [1-3]:
```

**Workflow after save:**
1. Create/update workflow file on feature branch
2. Git commit with message: `"Update workflow: Restart service safely"`
3. Print instructions for PR creation
4. If `gh` CLI available, optionally run `gh pr create`

---

### K.10 Export Templates

**Decision**: Support custom template files (improvement over Savvy).

**Savvy's approach:**
- Hard-coded Markdown template
- Compiled at init with `template.Must()`
- Not customizable

**Default template:** Similar to Savvy's with support for frontmatter

**Customization:**
```bash
# Check for custom template in order:
# 1. .gitsavvy/templates/export.md (repo-specific)
# 2. ~/.config/gitsavvy/templates/export.md (user-specific)
# 3. Use built-in default
```

**Template data available:**
- Workflow metadata: title, description, tags, created_at, updated_at
- Steps: name, description, command, shell, cwd
- Placeholders: name, prompt, default, validate
- Custom frontmatter support

---

### K.11 `run --local` Behavior

**Decision**: Use local Git checkout, error if workflow not found.

**Savvy's approach:**
- Users run `savvy sync` to download runbooks to local storage
- Local storage: `~/.config/savvy/savvy.local` (gob-encoded)
- When `--local` is set, uses `local.RunbookClient` instead of API
- No fallback - if runbook not in cache, returns `ErrNotFound`

**Implementation:**
```go
// internal/cli/run.go
if cmd.Local {
    // Skip fetch, use current checkout
    // No error if offline
} else {
    // Fetch latest via git pull
    if err := git.Fetch(ctx); err != nil {
        // Check if network error, suggest --local
        return fmt.Errorf("fetch failed: %w (use --local for offline mode)", err)
    }
}
```

**Error message:**
```
Error: workflow 'restart-service' not found.
Run 'gitsavvy sync' to update your local checkout, or use --local if offline.
```

---

### K.12 Step `cwd` Resolution

**Decision**: Support per-step `cwd`, relative to repository root.

**Savvy's approach:**
- No per-step working directories
- All steps execute in current working directory where `savvy run` was invoked

**Schema:**
```yaml
steps:
  - name: Check pods
    command: "kubectl get pods"
    cwd: "ops/k8s"  # Relative to repo root
```

**Resolution rules:**
- **Absolute paths:** Allowed but warned (reduces portability)
- **Relative paths:** Relative to **repository root** (not workflow directory)
- **Empty/omitted:** Use current working directory of `gitsavvy run` invocation

**Validation:**
```go
// internal/workflows/validate.go
func (s *Step) ValidateCWD(repoRoot string) error {
    if s.CWD == "" {
        return nil
    }

    // Check for path traversal
    if strings.Contains(s.CWD, "..") {
        return fmt.Errorf("step '%s': cwd cannot contain '..'", s.Name)
    }

    // Resolve path
    fullPath := filepath.Join(repoRoot, s.CWD)

    // Check if exists
    if _, err := os.Stat(fullPath); os.IsNotExist(err) {
        return fmt.Errorf("step '%s': cwd '%s' does not exist", s.Name, s.CWD)
    }

    return nil
}
```

**Error message:**
```
Error: Step 'Check pods': cwd 'ops/k8s' does not exist.
```

---

### K.13 `continue_on_error` Behavior

**Decision**: Skip for MVP interactive mode. Follow Savvy's model - user decides interactively.

**Savvy's approach:**
- No `continue_on_error` feature
- Interactive execution only - users manually press Enter to execute each step
- Exit codes tracked but no automatic behavior on failure
- When a command fails, workflow does NOT automatically proceed
- User sees the error and decides whether to continue or stop

**MVP interactive mode:**
```
Step 2/3: Restart deployment
Command: kubectl rollout restart deploy/foo

Error: Process exited with status 1

Options:
  [Enter] Continue to next step
  [s] Skip this step
  [r] Retry this step
  [q] Quit workflow

Your choice:
```

**Future enhancement** (for `--yes` non-interactive mode):
```yaml
steps:
  - name: Optional step
    command: "kubectl get pods"
    continue_on_error: true  # Don't stop on failure
```

---

### K.14 Drafts Directory

**Decision:** `drafts/` is gitignored (local-only), not committed to Git.

**Savvy's approach:**
- No drafts concept
- Workflows are either in the cloud or not
- The `redact` TUI is effectively the draft editing phase

**Location:** `drafts/<identity>/<workflow-slug>/`

**Behavior:**
- Drafts are **not committed to Git**
- Drafts are **not synced** by `gitsavvy sync`
- `gitsavvy record --draft` saves to `drafts/` instead of `workflows/`
- `gitsavvy edit` on a draft keeps it in `drafts/`
- `gitsavvy publish <draft>` moves from `drafts/` to `workflows/<identity>/` and commits

**Gitignore entry:**
```
# Git-backed workflow drafts
drafts/
```

**Additional commands:**
```bash
gitsavvy drafts list           # Show local drafts
gitsavvy drafts clean          # Remove drafts older than N days
gitsavvy publish <draft-name>  # Move draft to workflows and commit
```

---

### K.15 Shared/ Directory Permissions

**Decision:** `shared/` is a convention only, with optional Git-native enforcement.

**Savvy's approach:**
- No shared workflow concept
- All runbooks stored in Savvy's cloud backend
- Permissions managed by proprietary cloud service

**No built-in permissions:**
- Anyone with repo write access can write to `shared/`
- Git's permissions model applies (repo-level access control)
- `shared/` is just a directory like any other

**Optional enforcement via Git:**

**1. CODEOWNERS file:**
```
# Shared workflows require team approval
shared/ @platform-team

# Personal workflows - owner can approve
workflows/**/ @${owner}
```

**2. Branch protection rules:**
- Require PR review for changes to `shared/**`
- Auto-merge for personal `workflows/<user>/**`

**3. Pre-commit hook** (optional, set up by `gitsavvy init`):
```bash
#!/bin/bash
# .git/hooks/pre-commit
# Warn if non-team member pushes to shared/

CHANGED_SHARED=$(git diff --cached --name-only | grep "^shared/")
if [ -n "$CHANGED_SHARED" ]; then
    TEAM_PREFIX=$(git config gitsavvy.team-prefix)
    CURRENT_USER=$(git config user.name)

    # Simple check - can be enhanced
    if ! echo "$CURRENT_USER" | grep -q "$TEAM_PREFIX"; then
        echo "Warning: You're pushing to shared/ but aren't a member of $TEAM_PREFIX"
        echo "Continue? (y/n)"
        read -r response
        if [ "$response" != "y" ]; then
            exit 1
        fi
    fi
fi
```

---

### K.16 Updated Workflow Schema (v1)

Based on all decisions, the final workflow schema:

```yaml
schema_version: 1
# Optional stable ID (ULID) - provides rename safety
id: wf_01HZY3J9Y3G6Q9T3

title: Restart service safely
description: Restart foo-service and verify health
tags: [prod, kubernetes, runbook]

created_at: "2026-01-31T12:00:00Z"
updated_at: "2026-01-31T12:00:00Z"

# Optional defaults for all steps
defaults:
  shell: zsh
  cwd: .
  confirm_each_step: true

# Placeholder definitions (all treated uniformly - no "secret" type)
placeholders:
  service:
    prompt: "Service name"
    default: "foo-service"
    validate: "^[a-z0-9-]+$"
  namespace:
    prompt: "Kubernetes namespace"
    default: "default"

steps:
  - name: Check current pods
    description: "List all pods in the namespace"
    command: "kubectl -n <namespace> get pods -l app=<service>"
    shell: zsh  # Optional, overrides default
    cwd: "ops/k8s"  # Relative to repo root
    confirmation: true
    # continue_on_error not supported in MVP

  - name: Restart deployment
    command: "kubectl -n <namespace> rollout restart deploy/<service>"
    confirmation: true

  - name: Watch rollout
    command: "kubectl -n <namespace> rollout status deploy/<service>"
```

---

### K.17 Updated Config Schema

Based on all decisions, the updated config (changes from previous version noted):

```toml
[repo]
path = "/Users/chaz/.local/share/gitsavvy/repo"
remote = "origin"
branch = "main"
sync_strategy = "rebase"  # ff-only|rebase|merge
auto_reindex = true

[identity]
path = "platform/chaz"   # claimed write root inside repo
mode = "pr"              # direct|pr
team_prefix = "platform" # optional helper for validation/UI

[git]
author_name = "Chaz Straney"
author_email = "chaz@example.com"
sign_commits = false
push_on_save = false     # in direct mode only
pr_base_branch = "main"
feature_branch_template = "gitsavvy/{identity}/{date}/{slug}"

[workflows]
root = "workflows"       # repo-relative
shared_root = "shared"   # repo-relative
draft_root = "drafts"    # repo-relative (gitignored)
index_path = ".gitsavvy/index.json"  # gitignored
schema_version = 1

[runner]
default_shell = "zsh"    # bash|zsh|sh|pwsh
confirm_each_step = true
stream_output = true
max_output_lines = 5000
dangerous_command_warnings = true

# Placeholder settings
[placeholders]
prompt_style = "form"    # form|per-step
save_defaults = "none"   # none|session|file (keychain is future)

[tui]
enabled = true
theme = "default"
show_help = true

[editor]
command = "vim"          # if unset, use $EDITOR

[ai]
enabled = false          # must be explicitly enabled
provider = "openai_compat"
base_url = "http://localhost:11434/v1" # optional for compat
model = "gpt-4o-mini"
api_key_env = "OPENAI_API_KEY"
# redact = "basic"  # REMOVED - manual redaction only for MVP
confirm_send = true
```

---

### K.18 Gitignore Entries

Add to the repository's `.gitignore`:

```
# Git-backed workflow tool
.gitsavvy/index.json       # Local search index (auto-generated)

# Drafts are local-only, never committed
drafts/
```

---

