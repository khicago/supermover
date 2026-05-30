feature(f-235nwsp4y): task(T-003) Add persisted drift review and CLI-truthful narrow reconcile operator surfaces to the macOS shell.

Plan:
- Feat Goal: Add a native macOS SuperMover desktop operator shell that launches and reviews the current CLI truthfully, without renaming bounded publish and network workflows as ongoing sync or background daemon product behavior.
- Task: Add persisted drift review and narrow reconcile operator surfaces

Check:
- `go test -count=1 ./...` => PASS
- `go run ./cmd/supermover help` => PASS
- `cd macos && swift build` => PASS
- `git diff --check` => PASS

Learn:
- Keep the macOS shell aligned to CLI truth: `reconcile plan` may run without ids and `reconcile apply` keeps repeatable `--id` selection instead of app-only single-id narrowing.
- The daemon foreign-state full-suite failure was a test-assertion precision issue; fix the status-line assertion without changing daemon status scope semantics.
- Follow-up work stays separate: prune approval lifecycle surfaces remain planned under T-004.

Feature-ID: f-235nwsp4y
Task-ID: T-003
Gate-Result: pass
Task-Status: done
