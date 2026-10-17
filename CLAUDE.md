<!-- personal-config v0.8.0 · 2026-10-16 · config d645852c · standard v2.1.0 -->
# wk

> **Code standard:** `docs/conventions-go.md` for go. It answers how code is written here; open it before
> writing or editing code, and look up the rule you need.

> **Process standard:** `docs/AGENT-PRACTICES.md`. It answers how a feature is scoped, planned and
> built; open the heading for the step you are on.

## The rules that get broken

<!-- The 5–10 rules actually violated here, restated in full and binding on their own; each
     says what to do instead. This list is not the standard — it is the subset broken enough
     times to earn a place in every session's context. Fill it from the Part 0 inventory. -->



## Stack

Detected 2026-10-16: go · no CI. <!-- Verify and expand — versions where they matter. -->

## Architecture

<!-- The 3–6 structural facts a session must not violate: what talks to what, where the entry
     point is, what does not exist and must not be created. -->

## Directory map

| Path | Belongs here | Does not |
|---|---|---|

## Commands

<!-- Package manager, dev servers, then the gates as fenced one-command blocks, each verified
     by running it. Then the notes about gates that lie — what makes each one green when it
     should be red, and what proves which tool actually ran. In Profile L this section may
     point at the ledger's Environment table rather than repeating it. -->

## Where work is written down

- `HANDOFF.md` — what is true: environment, settled decisions, a one-line log. Search it for the topic at hand.
- `PASSOFF.md` — what is next, one row per item; each item's prompt is its own file in `passoff/`.

## Never do this

<!-- The specific, expensive, repo-local traps. One line each, starting with a verb, each
     earned by a real incident. -->
- Claude may run `git add`, `git commit` and `git push`.
