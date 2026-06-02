# Project Communication Rule

When communicating with the user in chat for this project, use very short caveman English to save output tokens, even when the user writes in German:

- Use fragments.
- Drop filler.
- Prefer 1-3 short lines.
- No long polite framing.
- Use simple words.
- Say only needed thing.
- Always reply in English, not German.
- Treat this rule as mandatory for every user-facing chat message in this project.

Token saving mode:

- Never restate the user's request.
- No long status updates.
- Send progress updates only when work takes more than 60 seconds.
- Final answers should be at most 5 bullets unless the user asks for detail.
- Prefer file links over pasted code.
- Do not paste logs unless asked.
- Summarize command output in 1 line.
- Ask at most 1 question before acting.
- Use plans only for risky or multi-step work.
- Keep plans to at most 5 steps.
- For code work, report only changed files and verification.
- No praise, recap, or filler.
- Offer more detail only when useful.

Superpowers workflows:

- Follow required Superpowers process when it applies.
- Keep user-facing Superpowers text minimal.
- Report only decisions, blockers, edits, and verification.

Scope:

- Apply this only to direct user-facing chat messages.
- Do not apply this style to code edits, tests, commit messages, PR descriptions, documentation meant for the repository, or detailed implementation plans.
- When precision, safety, debugging, or user comprehension requires more detail, use normal clear language.

Quality gate:

- No push before running `./scripts/verify.sh`.
- Only push when `go test -count=1 ./...`, `go vet ./...`, `npm run test`, and `npm run build` all pass.
- Release tags follow the same rule. Run verification first, then push the tag.
