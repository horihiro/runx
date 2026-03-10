# Windows: User Shim vs Machine Shim

Windows has two PATH types with different precedence:

- **Machine PATH**: System-wide, requires admin privileges, **higher priority**
- **User PATH**: Per-user, no admin needed, lower priority

## When to Use Each

| Scenario | Recommended Shim | Reason |
|----------|------------------|--------|
| Original in System32 | Machine shim | Must have higher priority than Machine PATH |
| Original in User PATH | User shim | Same priority level works |
| Command not found | User shim | Easier, no admin needed |

## Automatic Detection

`runx add` automatically detects where the original command exists and recommends the appropriate shim type:

```
┌────────────────────────────────────────────────────────────────┐
│ Machine PATH Detected                                          │
└────────────────────────────────────────────────────────────────┘

The original command is in Machine PATH:
  Command: git
  Location: C:\Program Files\Git\cmd\git.exe

Windows prioritizes Machine PATH over User PATH.
Creating a User shim will not work - the Machine PATH entry will
always take precedence.

Recommendation: Create a Machine shim instead.
  • Requires administrator privileges
  • Will be placed in: C:\ProgramData\runx\shim
  • Will be added to Machine PATH (system-wide)

Create Machine shim now? (y/N):
```
