feature(f-235nwsp4y): task(T-004) Add read-only prune lifecycle operator surfaces to the macOS shell.

Plan:
- Feat Goal: Add a native macOS SuperMover desktop operator shell that launches and reviews the current CLI truthfully, without renaming bounded publish and network workflows as ongoing sync or background daemon product behavior.
- Task: Add prune approval lifecycle read-only operator surfaces

Check:
- `go test -count=1 ./...` => PASS
- `go run ./cmd/supermover help` => PASS
- `cd macos && swift build` => PASS
- `git diff --check` => PASS
- `cd macos && ./script/build-app.sh` => PASS

Learn:
- Keep prune lifecycle read and write split: this slice adds `prune approvals` inventory plus richer `prune review` truth, but deliberately leaves `prune approve` / `prune supersede` mutation for the next task.
- Preserve `prune review` as release-review truth with authorization contract, candidates, refusals, approvals, receipts, and artifact problems; do not reduce it to approval inventory.
- Follow-up work remains explicit under T-005, and `prune --apply --approval <id>` target deletion stays outside the current slice.

Feature-ID: f-235nwsp4y
Task-ID: T-004
Gate-Result: pass
Task-Status: done
