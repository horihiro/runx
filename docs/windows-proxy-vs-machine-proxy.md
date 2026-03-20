# Windows: User Proxy vs Machine Proxy

`runx` creates command proxies as `.cmd` files in one of two locations:

- **User Proxy**: Placed in `%LOCALAPPDATA%\runx\proxy`, no admin needed, added to User PATH
- **Machine Proxy**: Placed in `C:\ProgramData\runx\proxy`, requires admin, added to Machine PATH (system-wide)

## PATH Priority

Windows has two PATH types with different precedence:

- **Machine PATH**: System-wide, requires admin privileges, **higher priority**
- **User PATH**: Per-user, no admin needed, lower priority

## When to Use Each

| Scenario | Recommended Proxy | Reason |
|----------|------------------|--------|
| Original in System32 | Machine proxy | Must have higher priority than Machine PATH |
| Original in User PATH | User proxy | Same priority level works |
| Command not found | User proxy | Easier, no admin needed |

## Automatic Detection

`runx add` automatically detects where the original command exists and recommends the appropriate proxy type:

```
┌────────────────────────────────────────────────────────────────┐
│ Machine PATH Detected                                          │
└────────────────────────────────────────────────────────────────┘

The original command is in Machine PATH:
  Command: git
  Location: C:\Program Files\Git\cmd\git.exe

Windows prioritizes Machine PATH over User PATH.
Creating a User proxy will not work - the Machine PATH entry will
always take precedence.

Recommendation: Create a Machine proxy instead.
  • Requires administrator privileges
  • Will be placed in: C:\ProgramData\runx\proxy
  • Will be added to Machine PATH (system-wide)

Create Machine proxy now? (y/N):
```
