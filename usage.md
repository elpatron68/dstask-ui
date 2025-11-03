# Usage

```
Usage: dstask [id...] <cmd> [task summary/filter]

Where [task summary] is text with tags/project/priority specified. Tags are
specified with + (or - for filtering) eg: +work. The project is specified with
a project:g prefix eg: project:dstask -- no quotes. Priorities run from P3
(low), P2 (default) to P1 (high) and P0 (critical). Text can also be specified
for a substring search of description and notes.

Cmd and IDs can be swapped, multiple IDs can be specified for batch
operations.

run "dstask help <cmd>" for command specific help.

Add -- to ignore the current context. / can be used when adding tasks to note
any words after.

Available commands:

next              : Show most important tasks (priority, creation date -- truncated and default)
add               : Add a task
template          : Add a task template
log               : Log a task (already resolved)
start             : Change task status to active
note              : Append to or edit note for a task
stop              : Change task status to pending
done              : Resolve a task
context           : Set global context for task list and new tasks (use "none" to set no context)
modify            : Set attributes for a task
edit              : Edit task with text editor
undo              : Undo last action with git revert
sync              : Pull then push to git repository, automatic merge commit.
open              : Open all URLs found in summary/annotations
git               : Pass a command to git in the repository. Used for push/pull.
remove            : Remove a task (use to remove tasks added by mistake)
show-projects     : List projects with completion status
show-tags         : List tags in use
show-active       : Show tasks that have been started
show-paused       : Show tasks that have been started then stopped
show-open         : Show all non-resolved tasks (without truncation)
show-resolved     : Show resolved tasks
show-templates    : Show task templates
show-unorganised  : Show untagged tasks with no projects (global context)
bash-completion   : Print bash completion script to stdout
zsh-completion    : Print zsh completion script to stdout
help              : Get help on any command or show this message
version           : Show dstask version information
```

# Syntax

## Priority

| Symbol | Name     | Note                                                                |
| ------ | -------- | ------------------------------------------------------------------- |
| `P0`   | Critical | Must be resolved immediately. May appear in all contexts in future. |
| `P1`   | High     | Need to be resolved in near future, display in highlighted          |
| `P2`   | Normal   | Default priority                                                    |
| `P3`   | Low      | Shown at bottom and faded.                                          |

## Operators

| Symbol          | Syntax                   | Description                                                      | Example                                       |
| --------------- | ------------------------ | ---------------------------------------------------------------- | --------------------------------------------- |
| `+`             | `+<tag>`                 | Include tag. Filter/context, or when adding task.                | `dstask add fix server +work`                 |
| `-`             | `-<tag>`                 | Exclude tag. Filter/context only.                                | `dstask next -feature`                        |
| `--`            | `--`                     | Ignore context. When listing or adding tasks.                    | `dstask --`, `task add -- +home do guttering` |
| `/`             | `/`                      | When adding a task, everything after will be a note.             | `dstask add check out ipfs / https://ipfs.io` |
| `project:`      | `project:<project>`      | Set project. Filter/context, or when adding task.                | `dstask context project:dstask`               |
| `-project:`     | `-project:<project>`     | Exclude project, filter/context only.                            | `dstask next -project:dstask -work`           |
| `template:`     | `template:<id>`          | Base new task on a template.                                     | `dstask add template:24`                      |
| `due:`          | `due:<date>`             | Set or filter by due date.                                       | `dstask add task due:friday`                  |
| `due.[filter]`  | `due.[filter]:<date>`    | Filter tasks based on due date filter (before, after, in/on).    | `dstask next due.before:tomorrow`             |


## State

| State    | Description                                   |
| -------- | --------------------------------------------- |
| Pending  | Tasks that have never been started            |
| Active   | Tasks that have been started                  |
| Paused   | Tasks that have been started but then stopped |
| Resolved | Tasks that have been done/close/completed     |

## Due Dates

Due dates can be specified when adding tasks or filtering existing tasks.

### Date Formats

| Format          | Description                                           | Examples                                                    |
| --------------- | ----------------------------------------------------- | ----------------------------------------------------------- |
| Relative dates  | `yesterday`, `today`, `tomorrow`                      | `due:tomorrow`                                              |
| Weekdays        | Full names or 3-letter abbreviations                  | `due:monday`, `due:wed`, `due:this-fri`, `due:next-tuesday` |
| Full dates      | YYYY-MM-DD format                                     | `due:2024-12-25`                                            |
| Month-day       | MM-DD format (sets current year automatically)        | `due:12-25`                                                 |
| Day of month    | DD format (sets current month and year automatically) | `due:15`                                                    |

### Adding Tasks with Due Dates

```bash
dstask add task with due date due:next-monday
dstask add finish report due:friday
dstask add pay bills due:15  # 15th of current month
dstask add halloween party due:2025-10-31
```

### Filtering Tasks by Due Date

```bash
dstask next due.before:friday        # Tasks due before Friday
dstask next due.after:tomorrow       # Tasks due after tomorrow  
dstask next due.on:tue               # Tasks due on Tuesday
dstask next due:overdue              # Overdue tasks
dstask next due.before:2025-12-31    # Tasks due before end of year
```

# Contexts

When dstask runs, a context can be set to filter the task output. Run `dstask help context`
for more examples. There are two ways to set a context.

1. The `context` command, which sets a global context on disk.
1. The `DSTASK_CONTEXT` environment variable. Contexts set by this environment
   variable override the global context on disk.

Use the `context` to set a context that will apply by default, no matter what
terminal window you're using.

Use the `DSTASK_CONTEXT` environment variable to override context in specific
uses. For instance, a [direnv](https://direnv.net/) config can set a context for
particular directories.

Context is not synchronised between machines.

# Dealing with merge conflicts

Dstask is written in such a way that merge conflicts should not happen, unless
a task is edited independently on 2 or more machines without synchronising. In
practice this happens rarely; however when it does happen dstask will fail to
commit and warn you. You'll then need to go to the underlying `~/.dstask` git
repository and resolve manually before committing and running `dstask sync`. In
some rare cases the ID can conflict. This is something dstask will soon be
equipped to handle automatically when the `sync` command runs.

# Performance

See [etc/PERFORMANCE.md](etc/PERFORMANCE.md)

# General tips

- Overwhelmed by tasks? Try focussing by prioritising (set priorities) or narrowing the context. The `show-tags` and `show-projects` commands are useful for creating a context.
- Use dstask to track things you might forget, rather than everything. SNR is important. Don't track tasks for the sake of it, and don't track _ideas_. Track ideas separately.
- Spend regular time reviewing tasks. You'll probably find some you've already resolved, and many you've forgotten. The `show-unorganised` command is good for this.
- Try to work through tasks from the top of the list. Dstask sorts by priority then creation date -- the most important tasks are at the top.
- Use `start`/`stop` to mark what you're genuinely working on right now; it makes resuming work faster. Paused tasks will be slightly highlighted, so you won't lose track of them. `show-paused` helps if they start to pile up.
- Keep a [github-style check list](https://help.github.com/en/articles/about-task-lists) in the markdown note of complex or procedural tasks
- Failing to get started working? Start with the smallest task
- Record only required tasks. Track ideas separately, else your task list will grow unboundedly! I keep an `ideas.md` for various projects for this reason.
- set `DSTASK_CONTEXT` in a `.envrc` per-project repository. With direnv, this allows you to automatically switch context