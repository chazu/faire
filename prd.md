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