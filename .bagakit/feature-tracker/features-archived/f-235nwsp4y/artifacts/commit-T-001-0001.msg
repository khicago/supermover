feature(f-235nwsp4y): task(T-001) Add initial native macOS operator app shell over current CLI with bundled CLI packaging and truthful operator task naming

Plan:
- Feat Goal: Add a native macOS SuperMover desktop operator shell that launches and reviews the current CLI truthfully, without renaming bounded publish and network workflows as ongoing sync or background daemon product behavior.
- Task: Implement initial macOS app shell scaffold

Check:
- `go test -count=1 ./internal/driftreview ./internal/cli ./internal/verify ./internal/report ./internal/health` => PASS
- `go vet ./...` => PASS
- `go mod tidy -diff` => PASS
- `git diff --check` => PASS
- `go run ./cmd/supermover drift acknowledge --help` => PASS

Learn:
- Add key learnings, risks, or follow-up notes here.

Feature-ID: f-235nwsp4y
Task-ID: T-001
Gate-Result: pass
Task-Status: done
