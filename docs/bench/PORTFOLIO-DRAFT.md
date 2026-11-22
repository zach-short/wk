# Portfolio draft for the wk story

A draft only. Nothing here is in `~/Developer/zach-short/portfolio`, and the wording is the owner's to change or drop. The wk story uses `display: 'terminal'` frames with a text `body` (`src/lib/projects.ts`), so these drafts are terminal frames, not PNGs, as the owner chose on 2026-11-21 (`METHOD.md`, question 14).

Each body is real output from the 2026-11-22 benchmark. Following the rule in `projects.ts`, the repository path is shortened to `~/code/demo` and the row number to 21, and lines are cut whole, never reworded. Six "skip clone of ...: the template has none" lines are cut from the second frame because they name the measured repository's package folders. Each sub states a measured number and makes no claim about why wk exists.

## Frame 1, the pool

```ts
{
  eyebrow: 'The pool',
  headline: 'A ready workspace in a tenth of a second.',
  sub: 'With a pool slot ready, wk new took a median of 0.10 s, against 2.44 s for git worktree add and a manual install (2026-11-22, Apple M5 Pro, 5 runs each).',
  terminal: {
    title: '~/code/demo · wk new',
    body: `$ cd "$(wk new 21)"
wk: claim pool slot wk-pool-ca989759   0.03s
wk: branch wk/21                       0.02s
wk: skip the post-clone commands: the clones were made at this commit`,
  },
},
```

Source: `data/A.csv` (way `wk_pool` and `baseline`, step `ready`, runs 2 to 5), and the log of a pool claim in the benchmark run.

## Frame 2, from the template

```ts
{
  eyebrow: 'From the template',
  headline: 'Ten workspaces at once, the last ready in 13 seconds.',
  sub: 'Ten wk new calls started together were all ready after a median of 13.03 s, against 15.23 s for ten plain worktrees with their own install (2026-11-22, Apple M5 Pro, 5 bursts each).',
  terminal: {
    title: '~/code/demo · wk new',
    body: `$ cd "$(wk new 21)"
wk: worktree add --no-checkout         0.02s
wk: clone node_modules                 1.02s
wk: clone native/node_modules          1.02s
wk: sparse checkout                    1.63s
wk: skip the post-clone commands: the clones were made at this commit`,
  },
},
```

Source: `data/C.csv` (way `wk_mp2` and `baseline`, step `wall`), and the log of a template create in the benchmark run.

The second frame's body names `native/node_modules`, a folder of the measured repository. If that is too specific for the portfolio's rule, cut that line too.
