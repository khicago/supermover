# Profile SSOT

Profiles are the single source of truth for sync behavior.

Runtime commands should edit or select a profile rather than silently overriding
important policy. Each successful run should store a profile snapshot in the
target control plane so the resulting state can be audited later.

Planned profile sections:

- roots
- include and exclude rules
- consistency mode
- delete policy
- metadata policy
- privacy policy
- pairing and target identity
- supplemental migration rules
- agent knowledge categories

