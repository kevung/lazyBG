---
status: accepted
---

# The transcription session logic is a plain Go service, independent of Wails

While locking the UI toolkit (ADR-0002) the user raised a longer-term idea: running lazyBG
headless behind a REST API, so the same backend could be driven as a service (automation,
remote/web client, batch use) rather than only through the desktop app. We are not specifying a
REST API now — that would be premature — but the toolkit choice is a good moment to take the
architectural precaution that keeps it cheap later. We decided the turn-entry/candidate-ranking/
export logic (`docs/functional-spec.md`, `docs/session-format-spec.md`) lives in a plain Go
package with no Wails-specific types or assumptions in its exported API (no `wails.Context`,
no JS-shaped return values chosen for frontend convenience). `cmd/lazybg`'s Wails bindings call
into this package as a thin adapter, exactly the way a future REST handler would.

## Consequences

- New/extended internal package (name TBD at implementation time, e.g. `internal/session`) owns
  session lifecycle, turn entry, candidate ranking, retroactive-edit cascade, and `.lbg`/`.mat`/
  manifest export — all pure Go, testable without Wails or a browser.
- Wails-bound methods in `cmd/lazybg` are thin: parameter/return shaping only, no business logic.
- No REST API, HTTP framework, or auth model is chosen now — deferred to when/if the headless
  mode is actually built, at which point this package is the thing a REST handler wraps.
