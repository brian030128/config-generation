# Project guidance for Claude Code

## Git worktree discipline (IMPORTANT)

When this session is running inside a git worktree (working directory under
`.claude/worktrees/<name>/`), **every file operation and command must target that
worktree**, never the original repository checkout.

- Use paths **under the current worktree directory**. Prefer paths relative to the
  worktree cwd; if using absolute paths, they must start with the worktree root
  (`…/config-generation/.claude/worktrees/<name>/…`), **not** the bare repo root
  (`…/config-generation/…`).
- Editing the bare repo path is a real bug: the worktree is a separate checkout, often
  on a different branch with its own uncommitted work. Writing there silently pollutes
  that other branch and means your build/tests run against the *worktree* (which never
  saw your edits), so green tests prove nothing.
- After editing, sanity-check with `git status` **in the worktree** (`git -C <worktree>
  status` or from the worktree cwd). If your changes don't show up there, they went to
  the wrong tree — stop and relocate them before continuing.
- `go` builds/tests: the module root is `backend/`. From the worktree run
  `go -C backend build ./...` and `go -C backend test ./...` (the BDD suite spins up a
  Postgres container via `dbtest.StartPostgres` and applies migrations).
