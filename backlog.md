# Backlog: Missing dstask Features and Enhancements

## 1) Due filters (due.before/after/on/overdue/in)
- Goal: Server-side filters in HTML views per usage.md
- Implementation
  - internal/server/util.go: add applyDueFilter(rows, token)
  - internal/server/render.go: extend filter UI (select Due filter → query `d=`)
  - internal/server/server.go: apply due filter after text/tag/project filter in all HTML routes
- Tests: util tests for before/after/on/overdue

## 2) Templates (show-templates, add template:<id>)
- Goal: List templates and create tasks from template
- Implementation
  - GET /templates: run `dstask show-templates`, table with IDs, summary preview, “Use template” action
  - /tasks/new: template dropdown; backend appends `template:<id>` to args
- Tests: handler for /templates (HTML), task creation with template args

## 3) Undo
- Goal: Roll back last action (git revert)
- Implementation
  - POST /undo: run `dstask undo`; flash success/error; redirect back
  - UI button (navbar or list page)
- Tests: stubbed runner; expect redirect + flash

## 4) Open (URLs)
- Goal: Open URLs from summary/notes
- Implementation
  - GET /tasks/{id}/open: extract URLs (via export/log) and render as link list
  - Add per-row action “open”
- Tests: URL extraction regex unit test

## 5) Edit (web variant of modify)
- Goal: Web-form edit when CLI editor is not available
- Implementation
  - GET /tasks/{id}/edit: form for summary/project/priority/due/tags
  - POST /tasks/{id}/edit: build `modify` args and call runner
- Tests: form validation and args build

## 6) Extended batch actions
- Goal: batch +tag/-tag, set priority/project/due
- Implementation
  - Batch form: new actions and inputs
  - /tasks/batch: handle new actions, validate, aggregate results, flash summary
- Tests: batch counters for ok/skipped/failed

## 7) Projects/Tags convenience links
- Goal: quick filter links
- Implementation
  - /projects: project name links to `/open?html=1&q=project:NAME`
  - /tags: tag links to `/open?html=1&q=+TAG`

## 8) Help page
- Goal: display usage.md in app
- Implementation
  - GET /help: render usage.md (markdown or pre)
  - Add navbar link

## 9) Security/UX polish (optional)
- CSRF tokens for POST
- Confirm dialogs for remove and destructive batch actions
- Pagination of large lists (page/per query)

---

## Rollout order
1. Due filters
2. Templates
3. Undo
4. Open (URLs)
5. Edit form
6. Extended batch actions
7. Help page & filter links
8. CSRF/Confirm/Pagination

## Effort (rough)
- Due filters: 0.5–1 day
- Templates: 0.5 day
- Undo: 0.25 day
- Open: 0.5–1 day
- Edit: 0.5–1 day
- Batch extensions: 0.5–1 day
- Help/links: 0.25 day
- CSRF/Paging: 0.5–1 day

