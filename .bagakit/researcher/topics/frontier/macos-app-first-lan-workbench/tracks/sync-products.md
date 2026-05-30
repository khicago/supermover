# sync-products

## Track Contract

- track id: `sync-products`
- parent pass: `pass-001`
- parent charter: `charter.md`

## Track Question

comparable sync and backup app boundaries

## Required Source Types
- Official documentation from mature sync, migration, or backup products.
- Evidence must map to a Supermover plan boundary, not general UI taste.

## Preferred Sources
- Syncthing docs for device identity and mutual configuration.
- CCC docs for verification and task history.
- Compare-first sync tool docs for preview/dry-run expectations.

## Disallowed Sources
- Marketing pages without operator workflow details.
- Bidirectional sync behavior as a direct Supermover requirement.

## Source Id Range

`pr01`-`pr09`

## Owned Output Files
- tracks/sync-products.md

## Minimum Evidence

At least one device-pairing source and one verification/history source.

## Lead Policy

Keep leads bounded to plan sequencing, operator trust states, or evidence UX.

## Drift Check

Do not import automatic conflict-resolution, bidirectional sync, or cloud
identity assumptions into Supermover's one-way migration model.

## Merge Notes
- Result: compare/dry-run and evidence history should be core workflow stages,
  not optional reports hidden behind a command log.
