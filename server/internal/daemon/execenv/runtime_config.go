package execenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// runtimeGOOS is the host-platform string used by buildMetaSkillContent and
// BuildCommentReplyInstructions to emit Windows-specific guidance. Defaults
// to runtime.GOOS; tests override it to exercise the cross-platform branches
// deterministically without having to run on every target OS.
var runtimeGOOS = runtime.GOOS

// formatProjectResource renders a single resource as a human-readable bullet.
// Unknown resource types fall back to a JSON-encoded ref so the agent can
// still read what the user attached. New resource types should add a case
// here AND in the API validator (handler/project_resource.go).
func formatProjectResource(r ProjectResourceForEnv) string {
	label := r.Label
	switch r.ResourceType {
	case "github_repo":
		var payload struct {
			URL               string `json:"url"`
			DefaultBranchHint string `json:"default_branch_hint,omitempty"`
		}
		_ = json.Unmarshal(r.ResourceRef, &payload)
		out := fmt.Sprintf("**GitHub repo**: %s", payload.URL)
		if payload.DefaultBranchHint != "" {
			out += fmt.Sprintf(" (default branch: `%s`)", payload.DefaultBranchHint)
		}
		if label != "" {
			out += " — " + label
		}
		return out
	default:
		ref := string(r.ResourceRef)
		if ref == "" {
			ref = "{}"
		}
		out := fmt.Sprintf("**%s**: `%s`", r.ResourceType, ref)
		if label != "" {
			out += " — " + label
		}
		return out
	}
}

// InjectRuntimeConfig writes the meta skill content into the runtime-specific
// config file so the agent discovers its environment through its native mechanism.
//
// For Claude:   writes {workDir}/CLAUDE.md  (skills discovered natively from .claude/skills/)
// For Codex:    writes {workDir}/AGENTS.md  (skills discovered natively via CODEX_HOME)
// For Copilot:  writes {workDir}/AGENTS.md  (skills discovered natively from .github/skills/)
// For OpenCode: writes {workDir}/AGENTS.md  (skills discovered natively from .opencode/skills/)
// For OpenClaw: writes {workDir}/AGENTS.md  (skills discovered natively from {workDir}/skills/ via per-task openclaw-config.json that pins agents.defaults.workspace)
// For Hermes:   writes {workDir}/AGENTS.md  (skills fall back to .agent_context/skills/; AGENTS.md points there)
// For Gemini:   writes {workDir}/GEMINI.md  (discovered natively by the Gemini CLI)
// For Pi:       writes {workDir}/AGENTS.md  (skills discovered natively from .pi/skills/)
// For Cursor:   writes {workDir}/AGENTS.md  (skills discovered natively from .cursor/skills/)
// For Kimi:     writes {workDir}/AGENTS.md  (Kimi Code CLI reads AGENTS.md natively; skills auto-discovered from project skills dirs)
// For Kiro:     writes {workDir}/AGENTS.md  (Kiro CLI reads AGENTS.md natively; skills auto-discovered from project skills dirs)
func InjectRuntimeConfig(workDir, provider string, ctx TaskContextForEnv) (string, error) {
	content := buildMetaSkillContent(provider, ctx)

	switch provider {
	case "claude":
		return content, os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(content), 0o644)
	case "codex", "copilot", "opencode", "openclaw", "hermes", "pi", "cursor", "kimi", "kiro":
		return content, os.WriteFile(filepath.Join(workDir, "AGENTS.md"), []byte(content), 0o644)
	case "gemini":
		return content, os.WriteFile(filepath.Join(workDir, "GEMINI.md"), []byte(content), 0o644)
	default:
		// Unknown provider — skip config injection, prompt-only mode.
		return content, nil
	}
}

// buildMetaSkillContent generates the meta skill markdown that teaches the agent
// about the Multica runtime environment and available CLI tools.
func buildMetaSkillContent(provider string, ctx TaskContextForEnv) string {
	var b strings.Builder

	b.WriteString("# Multica Agent Runtime\n\n")
	b.WriteString("You are a coding agent in the Multica platform. Use the `multica` CLI to interact with the platform.\n\n")

	// Always emit agent identity so the agent knows who it is, even when
	// dispatched via @mention on an issue assigned to a different agent.
	if ctx.AgentName != "" || ctx.AgentID != "" {
		b.WriteString("## Agent Identity\n\n")
		if ctx.AgentName != "" {
			fmt.Fprintf(&b, "**You are: %s**", ctx.AgentName)
			if ctx.AgentID != "" {
				fmt.Fprintf(&b, " (ID: `%s`)", ctx.AgentID)
			}
			b.WriteString("\n\n")
		}
		if ctx.AgentInstructions != "" {
			b.WriteString(ctx.AgentInstructions)
			b.WriteString("\n\n")
		}
	} else if ctx.AgentInstructions != "" {
		b.WriteString("## Agent Identity\n\n")
		b.WriteString(ctx.AgentInstructions)
		b.WriteString("\n\n")
	}

	// Knowledge base sections — injected before commands so the agent sees
	// workspace context and KB index early in its prompt.
	writeKBSections(&b, ctx)

	b.WriteString("## Available Commands\n\n")
	b.WriteString("**Use `--output json` for structured data.** Human table output now prints routable issue keys (for example `MUL-123`) and short UUID prefixes for workspace resources; use `--full-id` on list commands when you need canonical UUIDs.\n\n")
	b.WriteString("The default brief includes the commands needed for the core agent loop and common issue create/update tasks. For everything else, run `multica --help`, `multica <command> --help`, or `multica <command> <subcommand> --help`; prefer `--output json` when the command supports it.\n\n")
	b.WriteString("### Core\n")
	b.WriteString("- `multica issue get <id> --output json` — Get full issue details.\n")
	b.WriteString("- `multica issue comment list <issue-id> [--since <RFC3339>] --output json` — List comments on an issue; use `--since` for incremental polling.\n")
	b.WriteString("- `multica issue create --title \"...\" [--description \"...\" | --description-stdin | --description-file <path>] [--priority X] [--status X] [--assignee X | --assignee-id <uuid>] [--parent <issue-id>] [--project <project-id>] [--due-date <RFC3339>] [--attachment <path>]` — Create a new issue; `--attachment` may be repeated.\n")
	b.WriteString("- `multica issue update <id> [--title X] [--description X | --description-stdin | --description-file <path>] [--priority X] [--status X] [--assignee X | --assignee-id <uuid>] [--parent <issue-id>] [--project <project-id>] [--due-date <RFC3339>]` — Update issue fields; use `--parent \"\"` to clear parent.\n")
	b.WriteString("- `multica repo checkout <url> [--ref <branch-or-sha>]` — Check out a repository into the working directory (creates a git worktree with a dedicated branch; use `--ref` for review/QA on a specific branch, tag, or commit)\n")
	b.WriteString("- `multica issue status <id> <status>` — Shortcut for `issue update --status` when you only need to flip status (todo, in_progress, in_review, done, blocked, backlog, cancelled)\n")
	// Available Commands lists `multica issue comment add` neutrally —
	// three input modes, pick what fits.
	// The previous "MUST pipe via stdin" mandate (#1795 / #1851) was
	// originally a Codex-specific fix for codex emitting literal `\n`
	// escapes inside `--content "..."`, but it landed in this global
	// section and ended up steering every provider at stdin, which then
	// burned non-ASCII bytes on Windows where the agent's shell layer
	// (typically PowerShell) re-encodes the pipe through an ASCII /
	// non-UTF-8 codepage and drops non-representable bytes as `?`
	// (issues #2198 / #2236 / #2376).
	//
	// Strong "MUST" wording lives in the Codex-Specific section below
	// where it actually belongs; non-Codex providers handle inline
	// escaping correctly and can pick whichever flag suits their
	// content. The `--content-file` line in the menu doubles as a
	// pointer at the Windows-safe path.
	b.WriteString("- `multica issue comment add <issue-id> [--content \"...\" | --content-stdin | --content-file <path>] [--parent <comment-id>] [--attachment <path>]` — Post a comment. Pick the input mode that preserves your content; run `multica issue comment add --help` for details.\n")
	b.WriteString("- `multica doc list [--path-prefix <prefix>] [--tag <tag>] [--pinned] [--output json]` — List knowledge base documents\n")
	b.WriteString("- `multica doc get <path>` — Read a KB document's content\n")
	b.WriteString("- `multica doc tree [--path-prefix <prefix>]` — Show KB document tree\n")
	b.WriteString("- `multica doc search \"<query>\" [--limit N]` — Full-text search across KB documents\n")
	b.WriteString("- `multica doc grep \"<regex>\" [--path-prefix <prefix>] [--ignore-case]` — Client-side regex search across KB documents\n")
	b.WriteString("- `multica doc put <path> --content-stdin [--title T] [--description D] [--tags t1,t2]` — Create or update a KB document (pipe content via stdin)\n")
	b.WriteString("- `multica doc patch <path> --find \"...\" --replace \"...\" [--summary \"...\"]` — Surgically edit a KB document\n")
	b.WriteString("- `multica doc link <issue-id> <path> [--type referenced|consumed|produced]` — Link a KB document to an issue\n")
	b.WriteString("- `multica doc unlink <issue-id> <path>` — Unlink a KB document from an issue\n")
	b.WriteString("- `multica doc pin <path>` / `multica doc unpin <path>` — Pin/unpin a document for auto-injection\n\n")

	if provider == "codex" {
		b.WriteString("## Codex-Specific Comment Formatting\n\n")
		if runtimeGOOS == "windows" {
			b.WriteString("Codex often follows the per-turn reply command literally. On Windows, **always write the comment body to a UTF-8 file with your file-write tool first, then post it with `--content-file <path>`** — do NOT pipe via `--content-stdin`. PowerShell 5.1's `$OutputEncoding` defaults to ASCIIEncoding when piping to a native command, silently dropping non-ASCII characters as `?` before they reach `multica.exe`. Never use inline `--content` for agent-authored comments. ")
			b.WriteString("Keep the same `--parent` value from the trigger comment when replying. ")
			b.WriteString("Do not compress a multi-paragraph answer into one line and do not rely on `\\n` escapes.\n\n")
		} else {
			b.WriteString("Codex often follows the per-turn reply command literally. For issue comments, always use `--content-stdin` with a HEREDOC, even for short single-line replies. ")
			b.WriteString("Never use inline `--content` for agent-authored comments. Keep the same `--parent` value from the trigger comment when replying. ")
			b.WriteString("Do not compress a multi-paragraph answer into one line and do not rely on `\\n` escapes.\n\n")
		}
	}

	// Inject available repositories section.
	if len(ctx.Repos) > 0 {
		b.WriteString("## Repositories\n\n")
		b.WriteString("The following code repositories are available in this workspace.\n")
		b.WriteString("Use `multica repo checkout <url>` to check out a repository into your working directory. Add `--ref <branch-or-sha>` when you need an exact branch, tag, or commit.\n\n")
		for _, repo := range ctx.Repos {
			fmt.Fprintf(&b, "- %s\n", repo.URL)
		}
		b.WriteString("\nThe checkout command creates a git worktree with a dedicated branch. You can check out one or more repos as needed, and can pass `--ref` for review/QA on a non-default branch or commit.\n\n")
	}

	// Inject project-scoped context (resources attached to the issue's project).
	// The full structured payload is also available at .multica/project/resources.json
	// so skills can consume it programmatically.
	if ctx.ProjectID != "" || len(ctx.ProjectResources) > 0 {
		b.WriteString("## Project Context\n\n")
		if ctx.ProjectTitle != "" {
			fmt.Fprintf(&b, "This issue belongs to **%s**.\n\n", ctx.ProjectTitle)
		}
		if len(ctx.ProjectResources) > 0 {
			b.WriteString("Project resources (also written to `.multica/project/resources.json`):\n\n")
			for _, r := range ctx.ProjectResources {
				fmt.Fprintf(&b, "- %s\n", formatProjectResource(r))
			}
			b.WriteString("\nResources are pointers — open them only when relevant to the task. ")
			b.WriteString("For `github_repo` resources, use `multica repo checkout <url>` to fetch the code. Add `--ref <branch-or-sha>` when a task or handoff names an exact revision.\n\n")
		} else {
			b.WriteString("This project has no resources attached yet.\n\n")
		}
	}

	b.WriteString("### Workflow\n\n")

	if ctx.ChatSessionID != "" {
		// Chat task: interactive assistant mode
		b.WriteString("**You are in chat mode.** A user is messaging you directly in a chat window.\n\n")
		b.WriteString("- Respond conversationally and helpfully to the user's message\n")
		b.WriteString("- You have full access to the `multica` CLI to look up issues, workspace info, members, agents, etc.\n")
		b.WriteString("- If asked about issues, use `multica issue list --output json` or `multica issue get <id> --output json`\n")
		b.WriteString("- If asked about the workspace, use `multica workspace get --output json`\n")
		b.WriteString("- If asked to perform actions (create issues, update status, etc.), use the appropriate CLI commands\n")
		b.WriteString("- If the task requires code changes, use `multica repo checkout <url>` to get the code first. Use `--ref <branch-or-sha>` when you need an exact revision\n")
		b.WriteString("- Keep responses concise and direct\n\n")
	} else if ctx.QuickCreatePrompt != "" {
		// Quick-create task: detailed field / output rules live in the
		// per-turn prompt (BuildPrompt → buildQuickCreatePrompt) so they
		// have a single source of truth. Quick-create is one-shot, so the
		// per-turn message is always present and the agent reads the rules
		// from there. We only keep the hard guardrails here so a provider
		// that doesn't propagate the user message into its working context
		// (or a resumed session) still avoids the assignment-task workflow
		// pointing at an empty issue id.
		b.WriteString("**This task was triggered by quick-create.** There is NO existing Multica issue. Follow the field and output rules in the user message you just received; ignore the default assignment-task workflow.\n\n")
		b.WriteString("Hard guardrails (apply even if the user message is missing):\n")
		b.WriteString("- Run exactly one `multica issue create` invocation, then exit.\n")
		b.WriteString("- Do NOT call `multica issue get`, `multica issue status`, or `multica issue comment add` for this task — there is no issue to query, transition, or comment on. The platform writes the user's success/failure inbox notification automatically based on whether `multica issue create` succeeded.\n")
		b.WriteString("- If the CLI returns an error, exit with that error as the only output. Do not retry.\n\n")
	} else if ctx.AutopilotRunID != "" {
		// Autopilot run_only task: no issue exists, so the agent must not
		// follow the assignment/comment workflow.
		b.WriteString("**This task was triggered by an Autopilot in run-only mode.** There is no assigned Multica issue for this run.\n\n")
		fmt.Fprintf(&b, "- Autopilot run ID: `%s`\n", ctx.AutopilotRunID)
		if ctx.AutopilotID != "" {
			fmt.Fprintf(&b, "- Autopilot ID: `%s`\n", ctx.AutopilotID)
		}
		if ctx.AutopilotTitle != "" {
			fmt.Fprintf(&b, "- Autopilot title: %s\n", ctx.AutopilotTitle)
		}
		if ctx.AutopilotSource != "" {
			fmt.Fprintf(&b, "- Trigger source: %s\n", ctx.AutopilotSource)
		}
		if ctx.AutopilotTriggerPayload != "" {
			fmt.Fprintf(&b, "- Trigger payload:\n\n```json\n%s\n```\n", ctx.AutopilotTriggerPayload)
		}
		if strings.TrimSpace(ctx.AutopilotDescription) != "" {
			b.WriteString("\nAutopilot instructions:\n\n")
			b.WriteString(ctx.AutopilotDescription)
			b.WriteString("\n\n")
		}
		if ctx.AutopilotID != "" {
			fmt.Fprintf(&b, "- Run `multica autopilot get %s --output json` if you need the full autopilot configuration\n", ctx.AutopilotID)
		}
		b.WriteString("- Complete the autopilot instructions directly\n")
		b.WriteString("- Do not run `multica issue get`, `multica issue comment add`, or `multica issue status` for this run unless the autopilot instructions explicitly tell you to create or update an issue\n\n")
	} else if ctx.TriggerCommentID != "" {
		// Comment-triggered: focus on reading and replying
		b.WriteString("**This task was triggered by a NEW comment.** Your primary job is to respond to THIS specific comment, even if you have handled similar requests before in this session.\n\n")
		fmt.Fprintf(&b, "1. Run `multica issue get %s --output json` to understand the issue context\n", ctx.IssueID)
		fmt.Fprintf(&b, "2. Run `multica issue comment list %s --output json` to read the conversation (returns all comments, capped server-side at 2000)\n", ctx.IssueID)
		b.WriteString("   - For incremental polling, use `--since <RFC3339-timestamp>` to fetch only comments newer than a known cursor\n")
		fmt.Fprintf(&b, "3. Find the triggering comment (ID: `%s`) and understand what is being asked — do NOT confuse it with previous comments\n", ctx.TriggerCommentID)
		if ctx.IsSquadLeader {
			b.WriteString("4. **Decide whether a reply is warranted.** If you produced actual work this turn (investigated, fixed, answered a real question), post the result via step 6 — that is a normal reply, not a noise comment. If the triggering comment was a pure acknowledgment / thanks / sign-off from another agent AND you produced no work this turn, do NOT post a reply — and do NOT post a comment saying 'No reply needed' or similar. Simply exit with no output. Silence is a valid and preferred way to end agent-to-agent conversations.\n")
			fmt.Fprintf(&b, "   - **Squad leader rule:** If your evaluation outcome is `no_action`, call `multica squad activity %s no_action --reason \"...\"` and then EXIT IMMEDIATELY. DO NOT post any comment whose only purpose is to announce that you are taking no action, exiting silently, or acknowledging another agent. A comment like \"No action needed\" or \"Exiting silently\" is noise — the `squad activity` call already records your decision in the timeline.\n", ctx.IssueID)
		} else {
			b.WriteString("4. **Decide whether a reply is warranted.** If you produced actual work this turn (investigated, fixed, answered a real question), post the result via step 6 — that is a normal reply, not a noise comment. If the triggering comment was a pure acknowledgment / thanks / sign-off from another agent AND you produced no work this turn, do NOT post a reply — and do NOT post a comment saying 'No reply needed' or similar. Simply exit with no output. Silence is a valid and preferred way to end agent-to-agent conversations.\n")
		}
		b.WriteString("5. If a reply IS warranted: do any requested work first, then **decide whether to include any `@mention` link.** The default is NO mention. Only mention when you are escalating to a human owner who is not yet involved, delegating a concrete new sub-task to another agent for the first time, or the user explicitly asked you to loop someone in. Never @mention the agent you are replying to as a thank-you or sign-off.\n")
		b.WriteString("6. **If you reply, post it as a comment — this step is mandatory when you reply.** Text in your terminal or run logs is NOT delivered to the user. ")
		b.WriteString(BuildCommentReplyInstructions(provider, ctx.IssueID, ctx.TriggerCommentID))
		b.WriteString("7. Do NOT change the issue status unless the comment explicitly asks for it\n\n")
	} else {
		// Assignment-triggered: defer to agent Skills for workflow specifics.
		b.WriteString("You are responsible for managing the issue status throughout your work.\n\n")
		fmt.Fprintf(&b, "1. Run `multica issue get %s --output json` to understand your task\n", ctx.IssueID)
		fmt.Fprintf(&b, "2. Run `multica issue comment list %s --output json` to read the full comment history (returns all comments, capped server-side at 2000) — this is mandatory, not optional. Earlier comments often carry context the issue body lacks (e.g. which repo to work in, the prior agent's findings, the reason the issue was reassigned to you). Skipping this step is the most common cause of agents acting on stale or incomplete instructions.\n", ctx.IssueID)
		fmt.Fprintf(&b, "3. Run `multica issue status %s in_progress`\n", ctx.IssueID)
		b.WriteString("4. Follow your Skills and Agent Identity to complete the task (write code, investigate, etc.)\n")
		if ctx.IsSquadLeader {
			fmt.Fprintf(&b, "5. **Post your final results as a comment** (unless your outcome is `no_action` — in that case, calling `multica squad activity %s no_action --reason \"...\"` alone is sufficient; you MUST exit without posting any comment. DO NOT post a comment announcing no_action or saying you are exiting silently): `multica issue comment add %s --content \"...\"`. Your results are only visible to the user if posted via this CLI call; text in your terminal or run logs is NOT delivered.\n", ctx.IssueID, ctx.IssueID)
		} else {
			fmt.Fprintf(&b, "5. **Post your final results as a comment — this step is mandatory**: `multica issue comment add %s --content \"...\"`. Your results are only visible to the user if posted via this CLI call; text in your terminal or run logs is NOT delivered.\n", ctx.IssueID)
		}
		fmt.Fprintf(&b, "6. When done, run `multica issue status %s in_review`\n", ctx.IssueID)
		fmt.Fprintf(&b, "7. If blocked, run `multica issue status %s blocked` and post a comment explaining why\n\n", ctx.IssueID)
	}

	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Skills\n\n")
		switch provider {
		case "claude":
			// Claude discovers skills natively from .claude/skills/ — just list names.
			b.WriteString("You have the following skills installed (discovered automatically):\n\n")
		case "codex", "copilot", "opencode", "openclaw", "pi", "cursor", "kimi", "kiro":
			// Codex, Copilot, OpenCode, OpenClaw, Pi, Cursor, Kimi, and Kiro discover skills
			// natively from their respective paths. For OpenClaw, the daemon also writes a
			// per-task openclaw-config.json (exported via OPENCLAW_CONFIG_PATH) that pins
			// agents.defaults.workspace to the task workdir so the CLI's scanner picks up
			// {workDir}/skills/.
			b.WriteString("You have the following skills installed (discovered automatically):\n\n")
		case "gemini", "hermes":
			// Gemini reads GEMINI.md directly. Hermes has no native skills discovery
			// path wired up in resolveSkillsDir; both fall back to referencing the
			// files explicitly under .agent_context/skills/.
			b.WriteString("Detailed skill instructions are in `.agent_context/skills/`. Each subdirectory contains a `SKILL.md`.\n\n")
		default:
			b.WriteString("Detailed skill instructions are in `.agent_context/skills/`. Each subdirectory contains a `SKILL.md`.\n\n")
		}
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Mentions\n\n")
	b.WriteString("Mention links are **side-effecting actions**, not just formatting:\n\n")
	b.WriteString("- `[MUL-123](mention://issue/<issue-id>)` — clickable link to an issue (safe, no side effect)\n")
	b.WriteString("- `[@Name](mention://member/<user-id>)` — **sends a notification to a human**\n")
	b.WriteString("- `[@Name](mention://agent/<agent-id>)` — **enqueues a new run for that agent**\n\n")
	b.WriteString("### When NOT to use a mention link\n\n")
	b.WriteString("- Referring to someone in prose (e.g. \"GPT-Boy is right\") — write the plain name, no link.\n")
	b.WriteString("- **Replying to another agent that just spoke to you.** By default, do NOT put a `mention://agent/...` link anywhere in your reply. The platform already shows your comment to everyone on the issue; re-mentioning the other agent will make them run again, and if they reply with a mention back, you will be triggered again. That is a loop and it costs the user money.\n")
	b.WriteString("- Thanking, acknowledging, wrapping up, or signing off. These are exactly the moments where an accidental `@mention` causes the other agent to reply \"you're welcome\" and restart the loop. If the work is done, **end with no mention at all**.\n\n")
	b.WriteString("### When a mention IS appropriate\n\n")
	b.WriteString("- Escalating to a human owner who is not yet involved.\n")
	b.WriteString("- Delegating a concrete sub-task to another agent for the first time, with a clear request.\n")
	b.WriteString("- The user explicitly asked you to loop someone in.\n\n")
	b.WriteString("If you are unsure whether a mention is warranted, **don't mention**. Silence ends conversations; `@` restarts them.\n\n")
	b.WriteString("If you need IDs for mention links, inspect the relevant CLI help path and request JSON output when available.\n\n")

	b.WriteString("## Attachments\n\n")
	b.WriteString("Issues and comments may include file attachments (images, documents, etc.).\n")
	b.WriteString("When a task includes attachment IDs and you need the files, inspect `multica attachment --help` and use the authenticated CLI path. Do not open Multica resource URLs directly.\n\n")

	b.WriteString("## Important: Always Use the `multica` CLI\n\n")
	b.WriteString("All interactions with Multica platform resources — including issues, comments, attachments, images, files, and any other platform data — **must** go through the `multica` CLI. ")
	b.WriteString("Do NOT use `curl`, `wget`, or any other HTTP client to access Multica URLs or APIs directly. ")
	b.WriteString("Multica resource URLs require authenticated access that only the `multica` CLI can provide.\n\n")
	b.WriteString("If you need to perform an operation that is not covered by any existing `multica` command, ")
	b.WriteString("do NOT attempt to work around it. Instead, post a comment mentioning the workspace owner to request the missing functionality.\n\n")

	b.WriteString("## Output\n\n")
	switch {
	case ctx.AutopilotRunID != "":
		b.WriteString("This is a run-only autopilot task, so there may be no issue comment to post. Your final assistant output is captured automatically as the autopilot run result. Keep it concise and state the outcome.\n")
	case ctx.QuickCreatePrompt != "":
		b.WriteString("This is a quick-create task. There is NO existing issue to comment on. Your final stdout is captured automatically and the platform writes the user's success/failure inbox notification based on whether `multica issue create` succeeded.\n\n")
		b.WriteString("- Do NOT call `multica issue comment add` — the issue you just created has no conversation context for this run.\n")
		b.WriteString("- Print exactly one final line: `Created <identifier-or-id>: <title>` after a successful `multica issue create`. Use the created issue's `identifier` from JSON output when available; otherwise use its `id`. Do not assume any workspace issue prefix such as `MUL-`; workspaces can use custom prefixes.\n")
		b.WriteString("- On CLI failure, exit with the CLI error as the only output. The platform translates that into a `quick_create_failed` inbox item carrying the original prompt for the user.\n")
	default:
		if ctx.IsSquadLeader {
			b.WriteString("⚠️ **Final results MUST be delivered via `multica issue comment add`** — unless your outcome is `no_action`. When you evaluate a trigger and decide no action is needed, calling `multica squad activity <issue-id> no_action --reason \"...\"` alone is sufficient; you MUST exit without posting any comment. DO NOT post a comment that announces no_action, acknowledges another agent, or says you are exiting silently — such comments are noise. For all other outcomes (`action`, `failed`), a comment is still mandatory.\n\n")
		} else {
			b.WriteString("⚠️ **Final results MUST be delivered via `multica issue comment add`.** The user does NOT see your terminal output, assistant chat text, or run logs — only comments on the issue. A task that finishes without a result comment is invisible to the user, even if the work itself was correct.\n\n")
		}
		b.WriteString("Keep comments concise and natural — state the outcome, not the process.\n")
		b.WriteString("Good: \"Fixed the login redirect. PR: https://...\"\n")
		b.WriteString("Bad: \"1. Read the issue 2. Found the bug in auth.go 3. Created branch 4. ...\"\n")
		b.WriteString("When referencing an issue in a comment, use the issue mention format `[MUL-123](mention://issue/<issue-id>)` so it renders as a clickable link. (Issue mentions have no side effect; only member/agent mentions do — see the Mentions section above.)\n")
	}

	return b.String()
}

// Token budget constants for KB injection.
const (
	maxPinnedDocs      = 5
	maxPinnedTokens    = 4000
	maxIndexEntries    = 200
	maxLinkedDocs      = 10
	maxLinkedTokens    = 6000
)

// estimateTokens returns a rough token count (len/4).
func estimateTokens(s string) int {
	return len(s) / 4
}

// scopeIndexForBudget returns a subset of the document index that fits within
// maxItems. When the index exceeds the budget and a projectTitle is available,
// entries whose path starts with the project name (case-insensitive) are
// always included, and the remaining slots are filled alphabetically.
// The second return value is the count of items dropped.
func scopeIndexForBudget(entries []DocumentIndexEntry, projectTitle string, maxItems int) ([]DocumentIndexEntry, int) {
	if len(entries) <= maxItems {
		return entries, 0
	}

	// Normalize project title to a path prefix heuristic.
	// e.g. "My Project" → "my-project" or "my project"; we match case-insensitively
	// against the first path segment.
	projectPrefix := strings.ToLower(strings.TrimSpace(projectTitle))

	// If no project context, just take the first maxItems (alphabetically sorted from DB).
	if projectPrefix == "" {
		return entries[:maxItems], len(entries) - maxItems
	}

	// Partition into project-scoped and other entries.
	var projectEntries, otherEntries []DocumentIndexEntry
	for _, e := range entries {
		pathLower := strings.ToLower(e.Path)
		if strings.HasPrefix(pathLower, projectPrefix+"/") || strings.HasPrefix(pathLower, projectPrefix) {
			projectEntries = append(projectEntries, e)
		} else {
			otherEntries = append(otherEntries, e)
		}
	}

	// Always include all project entries (they're high-signal).
	// Fill remaining budget with alphabetically-first other entries.
	result := make([]DocumentIndexEntry, 0, maxItems)
	result = append(result, projectEntries...)
	remaining := maxItems - len(result)
	if remaining > 0 && len(otherEntries) > 0 {
		if remaining > len(otherEntries) {
			remaining = len(otherEntries)
		}
		result = append(result, otherEntries[:remaining]...)
	}

	// Re-sort by path for consistent tree rendering.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result, len(entries) - len(result)
}

// truncateDocsForBudget returns a prefix of docs that fits within maxItems and
// maxTokens (estimated as len/4). If truncated, the second return value is the
// count of items dropped.
func truncateDocsForBudget(docs []DocumentForEnv, maxItems, maxTokens int) ([]DocumentForEnv, int) {
	if len(docs) == 0 {
		return docs, 0
	}
	var result []DocumentForEnv
	totalTokens := 0
	for _, d := range docs {
		if len(result) >= maxItems {
			break
		}
		t := estimateTokens(d.Content)
		if len(result) > 0 && totalTokens+t > maxTokens {
			break
		}
		totalTokens += t
		result = append(result, d)
	}
	return result, len(docs) - len(result)
}

// writeKBSections renders the 4 knowledge base sections into the builder.
// Sections with no data are omitted entirely.
func writeKBSections(b *strings.Builder, ctx TaskContextForEnv) {
	hasAnyKB := ctx.WorkspaceContext != "" || len(ctx.PinnedDocuments) > 0 ||
		len(ctx.DocumentIndex) > 0 || len(ctx.IssueLinkedDocuments) > 0

	if !hasAnyKB {
		return
	}

	// §1 — Workspace overview
	if ctx.WorkspaceContext != "" {
		b.WriteString("## Workspace overview\n\n")
		b.WriteString(ctx.WorkspaceContext)
		b.WriteString("\n\n")
	}

	// §2 — Pinned documents
	if len(ctx.PinnedDocuments) > 0 {
		pinned, dropped := truncateDocsForBudget(ctx.PinnedDocuments, maxPinnedDocs, maxPinnedTokens)
		b.WriteString("## Pinned documents\n\n")
		for i, d := range pinned {
			fmt.Fprintf(b, "### %s\n\n", d.Path)
			b.WriteString(d.Content)
			b.WriteString("\n")
			if i < len(pinned)-1 {
				b.WriteString("\n---\n\n")
			}
		}
		if dropped > 0 {
			fmt.Fprintf(b, "\n\n> *(%d more pinned documents omitted due to token budget)*\n", dropped)
		}
		b.WriteString("\n")
	}

	// §3 — Knowledge base index
	if len(ctx.DocumentIndex) > 0 {
		b.WriteString("## Knowledge base index\n\n")
		b.WriteString("You have access to a workspace knowledge base. The tree below is everything\n")
		b.WriteString("that exists. Read individual files with `multica doc get <path>`.\n\n")
		b.WriteString("```\n")
		index, dropped := scopeIndexForBudget(ctx.DocumentIndex, ctx.ProjectTitle, maxIndexEntries)
		b.WriteString(renderIndexTree(index))
		if dropped > 0 {
			fmt.Fprintf(b, "\n... and %d more entries (use `multica doc list` to see all)\n", dropped)
		}
		b.WriteString("```\n\n")
	}

	// §4 — Linked to this issue
	if len(ctx.IssueLinkedDocuments) > 0 {
		linked, dropped := truncateDocsForBudget(ctx.IssueLinkedDocuments, maxLinkedDocs, maxLinkedTokens)
		b.WriteString("## Linked to this issue\n\n")
		for i, d := range linked {
			fmt.Fprintf(b, "### %s\n\n", d.Path)
			b.WriteString(d.Content)
			b.WriteString("\n")
			if i < len(linked)-1 {
				b.WriteString("\n---\n\n")
			}
		}
		if dropped > 0 {
			fmt.Fprintf(b, "\n\n> *(%d more linked documents omitted due to token budget)*\n", dropped)
		}
		b.WriteString("\n")
	}

	// §5 — Knowledge base usage instructions
	b.WriteString("## Knowledge base usage\n\n")
	b.WriteString("1. Look at the index above; identify docs likely relevant to this task.\n")
	b.WriteString("2. Read with `multica doc get <path>`.\n")
	b.WriteString("3. Cite docs you used in your final comment with `[path](mention://doc/<path>)`.\n")
	b.WriteString("4. Write back insights worth keeping with `multica doc put <path>`.\n")
	b.WriteString("   Use a clear path like `clients/<name>/research/<topic>.md`.\n")
	b.WriteString("5. Update existing docs surgically with `multica doc patch <path> --find ... --replace ...`.\n")
	b.WriteString("6. Link the issue to docs you used or produced:\n")
	b.WriteString("   `multica doc link <issue-id> <path> --type referenced|consumed|produced`.\n")
	b.WriteString("Pinned docs above are already loaded — do not re-read them.\n\n")
}

// renderIndexTree renders the document index as an ASCII tree.
// Paths must be sorted alphabetically (they come from the DB ORDER BY path).
func renderIndexTree(entries []DocumentIndexEntry) string {
	if len(entries) == 0 {
		return ""
	}

	// Build a simple tree structure.
	type node struct {
		name        string
		desc        string
		pinned      bool
		isFile      bool
		children    []*node
		childrenMap map[string]*node
	}

	root := &node{childrenMap: make(map[string]*node)}

	for _, e := range entries {
		parts := strings.Split(e.Path, "/")
		cur := root
		for i, part := range parts {
			isLast := i == len(parts)-1
			child, ok := cur.childrenMap[part]
			if !ok {
				child = &node{
					name:        part,
					childrenMap: make(map[string]*node),
				}
				cur.children = append(cur.children, child)
				cur.childrenMap[part] = child
			}
			if isLast {
				child.isFile = true
				child.desc = e.Description
				child.pinned = e.Pinned
			}
			cur = child
		}
	}

	// Sort children at each level.
	var sortChildren func(n *node)
	sortChildren = func(n *node) {
		sort.Slice(n.children, func(i, j int) bool {
			// Directories first, then files.
			iDir := !n.children[i].isFile || len(n.children[i].children) > 0
			jDir := !n.children[j].isFile || len(n.children[j].children) > 0
			if iDir != jDir {
				return iDir
			}
			return n.children[i].name < n.children[j].name
		})
		for _, c := range n.children {
			sortChildren(c)
		}
	}
	sortChildren(root)

	// Render tree with box-drawing characters.
	var out strings.Builder
	var render func(n *node, prefix string, isLast bool, isRoot bool)
	render = func(n *node, prefix string, isLast bool, isRoot bool) {
		if !isRoot {
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			label := n.name
			if len(n.children) > 0 && !n.isFile {
				label += "/"
			}
			if n.isFile && n.desc != "" {
				label += "  — " + n.desc
			}
			if n.isFile && n.pinned {
				label += " 📌"
			}
			out.WriteString(prefix + connector + label + "\n")
		}
		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}
		for i, child := range n.children {
			render(child, childPrefix, i == len(n.children)-1, false)
		}
	}
	render(root, "", true, true)

	return out.String()
}
