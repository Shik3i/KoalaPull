# KoalaPull Roadmap

Last updated: 2026-06-06

## Purpose

This file is the shared product roadmap for KoalaPull.
It is written for both humans and AI agents.

Use it to:

- capture approved work
- explain why a feature matters
- estimate effort, user value, and risk
- define acceptance criteria before implementation
- keep scope under control

This is not a dump of random ideas.
This is an execution document.

## Product Goals

Every roadmap item should strengthen at least one of these:

1. Fast, simple downloads for normal users.
2. Safe defaults for dependency updates and media downloads.
3. Good visibility into what the app is doing.
4. Strong local-first UX with low friction.
5. Stable performance on large queues and histories.

If a new item does not support one of these goals, it should usually not enter the roadmap.

## Prioritization Rules

Use this order when deciding what to build next:

1. Security and trust problems.
2. User-facing pain in core flows.
3. High-value QoL in frequent workflows.
4. Performance and maintainability work that unlocks future speed.
5. Nice-to-have ideas.

Good roadmap items usually have at least one of these traits:

- removes repeated user friction
- reduces chance of data loss or broken installs
- makes failure easier to understand and recover from
- improves core flows without adding heavy complexity
- unlocks future features cleanly

Bad roadmap items usually have at least one of these traits:

- vague scope
- unclear owner or acceptance criteria
- low user value but high implementation surface
- duplicates existing behavior
- adds many settings for a niche use case

## Status Model

Use one of these statuses:

- `proposed`: idea exists, not approved
- `approved`: accepted for roadmap, not started
- `planned`: broken down enough to implement soon
- `in-progress`: actively being built
- `blocked`: cannot continue without a decision or prerequisite
- `done`: shipped
- `dropped`: intentionally not doing

## Scoring Model

Use simple 1-5 scoring.

### Effort

- `1`: tiny change, low surface area
- `2`: small feature or isolated refactor
- `3`: medium feature across multiple files
- `4`: large feature across backend and frontend
- `5`: major project, migration, or risky redesign

### User Value

- `1`: barely noticeable
- `2`: useful for a small group
- `3`: meaningful improvement
- `4`: strong daily UX win
- `5`: major product-level improvement

### Error Potential

- `1`: very safe, easy rollback
- `2`: low risk
- `3`: moderate regression risk
- `4`: high risk in critical flows
- `5`: very risky, broad blast radius

### Priority Heuristic

Prefer items with:

- high user value
- low or moderate effort
- low or moderate error potential

Security items can still rank high even when effort or risk is higher.

## Roadmap Entry Template

Every new entry should follow this shape:

```md
## [ID] Title

- Status: `approved`
- Priority: `P0` | `P1` | `P2`
- Area: `frontend` | `backend` | `security` | `performance` | `ux`
- Effort: `1-5`
- User Value: `1-5`
- Error Potential: `1-5`
- Depends on: `none` or list of IDs

### Problem

Short explanation of current pain.

### Outcome

What users get after ship.

### Scope

- included thing
- included thing
- included thing

### Out of Scope

- not included thing
- not included thing

### Acceptance Criteria

- measurable behavior
- measurable behavior
- measurable behavior

### Notes

Extra design or implementation guidance.
```

## Writing Rules For New Entries

When adding a new roadmap item:

1. Start with the user problem, not the implementation.
2. Keep title short and concrete.
3. Keep scope tight enough for one focused PR series.
4. Add explicit out-of-scope notes.
5. Add acceptance criteria that can be tested.
6. Mention security, UX, and performance impact if relevant.
7. If an item is vague, split it before approval.

### Best Practices

- Prefer outcome language over tool language.
- Prefer one strong feature over five weak subfeatures.
- Use warnings and confirmations instead of hard blocks when user intent can be valid.
- Keep default flows simple; push complexity into advanced controls.
- Favor reversible changes and safe fallback behavior.
- Preserve local-first and privacy-first behavior.
- Do not add settings unless they solve a real repeated need.
- For risky work, define rollback behavior before implementation.

### AI-Agent Guidance

AI agents editing this file should:

- preserve existing IDs
- avoid silently changing approved scope
- append new items instead of rewriting history
- update `Last updated`
- keep acceptance criteria concrete
- mark assumptions clearly
- never mark `done` without shipped code and verification

## Current Approved Roadmap

## KP-004 QoL Pack

- Status: `approved`
- Priority: `P1`
- Area: `frontend`, `backend`, `ux`
- Effort: `4`
- User Value: `4`
- Error Potential: `3`
- Depends on: `none`

### Problem

Frequent users need faster repeat workflows.
Current settings and queue flows make repeated patterns more manual than needed.

### Outcome

Power users get faster repeated downloads without making the default UI heavy.

### Scope

- saved preset profiles
- import and export settings
- desktop notifications on finish and fail
- recent output folders
- "download again" shortcut
- "retry failed" shortcut

### Out of Scope

- cloud sync
- account system
- remote profile sharing

### Acceptance Criteria

- users can save and reuse named preset profiles
- settings import and export validate file shape safely
- notifications can be enabled without becoming noisy
- recent output folders speed up repeated workflows
- retry failed action works for multiple failed items

### Notes

Keep preset UX simple.
Do not create a settings maze.

## KP-005 Performance And Maintainability Pack

- Status: `approved`
- Priority: `P1`
- Area: `performance`, `frontend`, `backend`
- Effort: `4`
- User Value: `4`
- Error Potential: `3`
- Depends on: `none`

### Problem

`frontend/src/App.tsx` is too large.
Long queue and history lists will become harder to maintain and may cost responsiveness.
Metadata fetch behavior can also be smarter.

### Outcome

The app stays responsive at scale and becomes easier to extend safely.

### Scope

- split `frontend/src/App.tsx` into smaller focused modules
- virtualize long queue and history lists
- cache metadata by URL with sensible invalidation
- cancel stale metadata fetches faster
- reduce unnecessary rerenders in queue-heavy scenarios

### Out of Scope

- premature micro-optimizations with no measured need
- backend rewrites unrelated to app responsiveness
- large state-management migration unless justified

### Acceptance Criteria

- `App.tsx` no longer holds most feature logic in one file
- large queue and history views remain smooth
- duplicate metadata fetches for same URL are reduced
- stale fetches do not overwrite newer results
- performance changes do not weaken correctness or accessibility

### Notes

Prefer measured improvements.
Avoid clever code that makes future work harder.

## Future Candidate Backlog

These are not approved yet.
Add here before moving into the main roadmap.

- Better per-site capability hints before download
- Download scheduler or quiet hours
- Advanced history filters and saved searches
- Per-download proxy support
- Integrity view for installed dependency versions

## Definition Of Ready

A roadmap item is ready for implementation when:

- status is `planned` or `approved`
- scope is small enough to estimate
- acceptance criteria are testable
- obvious risks are documented
- dependencies are known

## Definition Of Done

A roadmap item is done when:

- code is merged
- tests are updated where needed
- docs are updated where needed
- user-visible behavior matches acceptance criteria
- verification passes through the canonical verifier

## Change Log Rules

When editing roadmap state:

- keep history simple
- do not delete shipped items
- move status forward explicitly
- if scope changes materially, add a note under the item

