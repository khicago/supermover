feature(f-235nwsp4y): task(T-005) Add prune approval artifact mutation surfaces to the macOS shell.

Plan:
- Feat Goal: Add a native macOS SuperMover desktop operator shell that launches and reviews the current CLI truthfully, without renaming bounded publish and network workflows as ongoing sync or background daemon product behavior.
- Task: Add prune approval artifact mutation surfaces

Check:
- `go test -count=1 ./...` => PASS
- `go run ./cmd/supermover help` => PASS
- `cd macos && swift build` => PASS
- `git diff --check` => PASS
- `cd macos && ./script/build-app.sh` => PASS

Learn:
- Keep prune approval mutation inputs explicit: use dedicated approval id and soft-delete ids rather than overloading drift or session selectors.
- Structure mutation results around artifact writing and review-state change; do not imply receipt writing, target deletion, or apply semantics in this slice.
- `prune --apply --approval <id>` remains the only physical prune path and stays outside the current macOS shell scope.

Feature-ID: f-235nwsp4y
Task-ID: T-005
Gate-Result: pass
Task-Status: done
