# Usage examples

This page walks you through concrete runx usage flows, organized around specific commands.

## Azure CLI with runx

With Azure CLI, you can switch the `AZURE_CONFIG_DIR` environment variable to use a different Azure profile per directory — letting `az` automatically pick up the right credentials depending on where you run it.

### Goal

- In directory A, `az` uses Azure profile A
- In directory B, `az` uses Azure profile B
- The `az` command itself is invoked exactly as usual

### 1. Prepare directories and env files

By switching `AZURE_CONFIG_DIR`, Azure CLI credentials and configuration are kept separate per directory.

```bash
mkdir -p ~/work/az_profile1 ~/work/az_profile2

cat > ~/work/az_profile1/.azclienv <<'EOF'
AZURE_CONFIG_DIR=~/.azure_profile1
EOF

cat > ~/work/az_profile2/.azclienv <<'EOF'
AZURE_CONFIG_DIR=~/.azure_profile2
EOF
```

### 2. Register a command proxy for az

Associate `.azclienv` with the `az` command:

```bash
runx add az --envfile=.azclienv
```

> [!NOTE]
> `--envfile` accepts either a filename or an absolute path.  
> When a filename is given, runx searches for it at command execution time, starting from the current directory, then parent directories, up to the home directory.

On Linux/macOS, a shell function is added to your shell configuration file. Reload your shell to activate it:

```bash
# bash
source ~/.bashrc
```

```zsh
# zsh
source ~/.zshrc
```

### 3. Use az in each directory

The environment is automatically switched depending on the current directory:

```bash
cd ~/work/az_profile1
az account show

cd ~/work/az_profile2
az account show
```

On first use, you may need to log in for each `AZURE_CONFIG_DIR`:

```bash
cd ~/work/az_profile1
az login

cd ~/work/az_profile2
az login
```

> [!NOTE]
> When you specify multiple env files in `runx add` (for example, by passing `--envfile` multiple times), they are merged with later entries taking precedence.

### 4. Check registered proxies

```bash
runx list
```

If `az` appears in the output, the proxy is registered.

### 5. Preview which environment variables will be applied

Before running a command, you can inspect which env vars will be injected in the current directory.

```bash
cd ~/work/az_profile1
runx env az
# AZURE_CONFIG_DIR=/home/user/.azure_profile1

cd ~/work/az_profile2
runx env az
# AZURE_CONFIG_DIR=/home/user/.azure_profile2
```

You can also add extra env files on top of the registered ones to see how an override would look:

```bash
runx env az --envfile=.extra.env
```

### 7. One-off execution (no registration)

To run a command with a specific env file once without registering a proxy, use `runx exec`:

```bash
cd ~/work/az_profile1
runx exec --envfile=.azclienv az account show
```

You can preview what that one-off execution would inject before actually running it:

```bash
runx env --envfile=.azclienv
```

### 8. Remove the proxy

When the proxy is no longer needed, remove it:

```bash
runx remove az
```


