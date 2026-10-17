<!-- personal-config v0.8.0 · 2026-10-16 · config a354b538 · standard v2.1.0 -->
# Go conventions

The standard for Go in this repo. **Self-contained** — apply it without reading
anything else.

Every rule has an ID, and IDs are never renumbered — they are how commit messages, other
docs and lint justifications cite a rule. Every rule carries an enforcement tag: *lint* (a
linter catches it) · *gate* (a repo checker catches it) · *CI* (a job catches it) · *review*
(nothing catches it — it rots without discipline). The tag tells you whether a clean run
means anything.

Provenance labels mark which rules are choices: *[STANDARD]* canonical for the ecosystem ·
*[COMMON]* widespread, alternatives exist · *[OURS]* a house preference, justified on its
own terms. This is what stops a later agent "correcting" a deliberate call.

gofmt settles formatting entirely, and golangci-lint settles much of the rest. Neither is restated here.

> **Seeded by `personal-config` from your answers.** Every correct/incorrect pair below is
> illustrative until someone replaces it with real code from this repo — the standard asks
> for real pairs, and a seeded one is a placeholder that happens to compile.
>
> Every rule is seeded *review* for the same reason: a tag is a claim about tooling, and
> nothing here has configured a linter, a type gate or a CI job that catches these. Promote
> a rule to *lint*, *gate* or *CI* yourself once something does.

## C1 — Comments say why, never what

*review* · **[OURS]**

A comment restating the line below it is banned — rename or extract instead. A comment recording *why* is required where the reason is not derivable from the code. **Never bulk-delete comments**, and never strip one in a protected category: product or design rationale, a lint-suppression justification, an external-constraint workaround, or documentation on a public export.

```
// right: // Retry once: the upstream returns 502 on its first call after a deploy.
// wrong: // increment i
```

## L1 — Functions stay short — roughly 6–15 lines

*review* · **[OURS]**

A function that fits on a screen is reviewed at a glance and tested alone. Extract for a *concept*, not to relocate lines: a helper that needs three parameters to explain itself was the wrong cut. Markup and view bodies are exempt from the count.

```
// right: function priceWithTax(order) { return applyTax(subtotal(order), order.region); }
// wrong: function priceWithTax(order) { /* 40 lines of inlined subtotal and tax */ }
```

## E1 — Minimize the exported surface

*review* · **[STANDARD]**

Capitalization *is* the export mechanism. Export an identifier only when something outside the package calls it, and avoid stutter: `helpcontent.Load`, not `helpcontent.LoadHelpContent`.

```
// right: func Load(ctx context.Context) (*Content, error)  // package helpcontent
// wrong: func LoadHelpContent(ctx context.Context) (*HelpContent, error)
```

## F1 — Filenames are lowercase with underscores

*review* · **[COMMON]**

Acronyms are words, not shouts: `userId`, `apiUrl`, `HttpClient` — except in Go, where initialisms keep uniform case (`userID`, `apiURL`, `HTTPClient`). Where two standards disagree on this, the file extension decides which applies, and both say so.

```
// right: listing_unit_postgres.go
// wrong: listingUnitPostgres.go
```

## T1 — The type layer has no silent escape hatches

*review* · **[OURS]**

No `any` at an API boundary — it defers type checking to runtime. Typed string constants for anything that crosses a wire or a database. Make the zero value useful.

```
// right: type Status string

const StatusOpen Status = "open"
// wrong: func Handle(v any) error
```

## L2 — Logic is extracted by concept

*review* · **[OURS]**

SQL lives only in `repository/`; controllers and services call it. Errors become HTTP status codes in the handler and nowhere else.

```
// right: rows, err := r.db.Query(ctx, `SELECT …`)  // repository/listing.go
// wrong: rows, err := c.db.Query(ctx, `SELECT …`)  // controller/listing.go
```

## D1 — One typed client; raw calls are banned in app code

*review* · **[OURS]**

Every network and database call goes through one typed client; a bare `fetch` or a raw driver call in app code fails review, because the client is the one place retries, auth, error shape and types are decided. Environment variables are read and validated once at startup and passed down — never read inline in app code, where a missing value surfaces as a runtime undefined three layers away from the cause.

```
// right: row := r.db.QueryRow(ctx, q, id)  // repository/
// wrong: row := db.QueryRow(context.Background(), q, id)  // service/
```

## X1 — Pure logic gets tests, in a new file named for the feature

*review* · **[COMMON]**

A new file rather than an addition to an existing suite, so two sessions working in parallel do not collide in one file. Write characterization tests *before* moving logic, not after — a test written after the move proves the new shape, not that the behaviour survived.

```
// right: func TestApplyTax(t *testing.T) { … }  // table-driven, t.Run subtests
// wrong: one Test func asserting six unrelated behaviours
```
