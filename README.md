# OpenRig

OpenRig is an agentless coding harness exposed through MCP.

## Agent Workflow

Call `help` when the lifecycle is unfamiliar. Routine work starts directly with
`turn op=begin`; use `status` only to recover context or diagnose runtime state.
`help` explains workflows supported by the effective catalog without repeating
the tool definitions already returned by MCP `tools/list`.

Common flow:

1. `turn op=begin` with `repo` and a concise `goal`. OpenRig creates the
   isolated worktree.
2. Call work tools with the returned `turn_id`.
3. Call `diff` with that `turn_id`.
4. `turn op=end` with an `outcome` and concise summary.

Use `status`, explicit `worktree` management, advanced diff ranges, and process
supervision only when the task requires them. Use freestyle mode as an explicit
escape hatch when isolated worktree execution is not appropriate.

Native tools:

- `help`: lifecycle and contract guidance.
- `status`: compact current-session readiness and recovery summary.
- `worktree`: isolated Git worktree lifecycle.
- `turn`: scoped agent-work lifecycle.
- `diff`: bounded turn, worktree, Git, and revision diffs.
- `shell`: bounded one-shot shell execution.
- `apply_patch`: structured file edits.
- `process`: supervised long-running processes.
- `skill_activate`: conditional on local skill availability.

Public naming is intentional:

- `repo` selects a configured repository name, alias, or allowed path.
- `base` is the existing Git ref used to create a worktree.
- `branch` is an optional new branch created for that worktree.
- `workdir` is an optional path relative to the scoped workspace root.
- `cwd` appears only in responses that report a resolved absolute directory.

OpenRig resolves each `repo` once before domain work. Configured names and
aliases take precedence over path interpretation; absolute paths are accepted,
while relative paths are transport policy and are disabled for remote-capable
transports. Resolved workspaces and contained paths are canonicalized through
physical symlinks. Existing symlinks cannot be used to escape the scoped
workspace, including when the final target does not exist yet.

Each tool call receives immutable invocation metadata with fields for its
resolved workspace and lifecycle identity when applicable, plus tool-call and
trace correlation, transport, and MCP session ID. A transport session is
correlation only: it is not authentication, workspace ownership, or a
substitute for `turn_id`. Resource access compares the canonical workspace and
lifecycle IDs exactly.

Shell, process, diff, and skill output bounds are runtime policy rather than
agent-selected tuning knobs. Worktree, turn, and process list operations return
runtime-bounded snapshots of recent matching resources in deterministic order.
Exact bounds and sort keys are runtime policy rather than agent-selected knobs
or wire-compatibility promises.

OpenRig publishes explicit MCP annotations for every available native tool.
Flat operation-union tools use conservative tool-wide hints: if any operation
can mutate or destroy state, the whole tool is annotated accordingly. Shell and
process are open-world because their commands may interact with external
systems.

The common catalog intentionally omits full output schemas. Its names,
descriptions, input schemas, and annotations are kept under a hard serialized
budget so every request does not repeatedly spend agent context on large
response schemas. Structured output DTOs and wire shapes remain tested in Go.
