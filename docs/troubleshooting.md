# Troubleshooting

## Windows: Proxy not working after creation

**Symptoms**: Created a User proxy but command still uses original without environment.

**Cause**: Original command is in Machine PATH, which has higher priority.

**Solution**: Remove User proxy and create Machine proxy:

```cmd
runx remove terraform
runx add terraform --envfile=.env
# Answer "y" when prompted to create Machine proxy
```

## Linux: Function not found

**Symptoms**: `bash: terraform: command not found` after `runx add`.

**Cause**: Shell config not reloaded.

**Solution**:

```bash
# Reload shell config
source ~/.bashrc  # for bash
source ~/.zshrc   # for zsh
# or just restart your terminal
```

## Environment file not found

**Symptoms**: `envfile not found: .env`

**Possible causes:**
1. File doesn't exist in search path (current dir -> root -> home)
2. Typo in file name
3. File has wrong permissions

**Solution**:

```bash
# Check search trace with debug mode
export RUNX_DEBUG=2
runx exec --envfile=.env terraform plan

# Output shows where it searched:
# [runx][debug] envfile search trace:
# [runx][debug]   .env:
# [runx][debug]     - /current/dir/.env
# [runx][debug]     - /parent/dir/.env
# [runx][debug]     - /home/user/.env
```

## Multiple proxies interfering

**Symptoms**: Command behaves unexpectedly or uses wrong environment.

**Cause**: Multiple tools creating proxies for the same command.

**Solution**:

```bash
# List all runx proxies
runx list

# Remove conflicting proxy
runx remove terraform

# Recreate with correct settings
runx add terraform --envfile=.env
```
