# runx - Environment-Aware Command Shim Manager

`runx` is an environment-aware command shim manager—a cross-platform tool that creates command shims (wrappers) that automatically load environment variables from files before executing commands. Useful for managing cloud profiles, API keys, runtime settings, or any directory-scoped configuration.

## Features

- **🔧 Cross-Platform**: Works on Windows, Linux, and macOS
- **📦 Single Binary, No Runtime Required**: Just one executable, no separate runtime installation needed
- **📁 Directory-Based Context**: Automatically searches for environment files from current directory to root, then home directory
- **🔄 Multiple Environment Files**: Merge multiple `.env` files with later values overriding earlier ones
- **🎯 Command Shims**: Create persistent command wrappers that automatically load the correct environment
- **🐚 Shell Support**: Bash, Zsh, Fish on Linux/macOS; CMD on Windows
- **🔐 Windows PATH Management**: Smart User/Machine PATH detection with automatic privilege escalation when needed
- **🐛 Debug Mode**: Set `RUNX_DEBUG=1` or `RUNX_DEBUG=2` for detailed environment file resolution tracing

## Installation

Install from GitHub Release Assets.

### Ubuntu (.deb)

1. Open the releases page: `https://github.com/horihiro/runx/releases`
2. Open the release for the version you want to install (tag example: `v0.1.0`).
3. Download the `.deb` asset for your architecture.
	- `amd64`: `runx_<version>_amd64.deb`
	- `arm64`: `runx_<version>_arm64.deb`
4. Install with `apt`:

```bash
sudo apt install ./runx_<version>_<arch>.deb
```

5. Verify:

```bash
runx --version
```

### Windows (winget + manifest zip)

1. Open the releases page: `https://github.com/horihiro/runx/releases`
2. Open the release for the version you want to install (tag example: `v0.1.0`).
3. Download `winget-manifest-<tag>.zip` from Release Assets.
4. Extract the zip to a local directory.
5. Install using `winget` with the extracted manifest files:

```powershell
winget install --manifest <ExtractedDirectory>
```

6. Verify:

```powershell
runx --version
```

### Manual Install (No Package Manager)

#### Linux/macOS

1. Open the releases page: `https://github.com/horihiro/runx/releases`
2. Open the release for the version you want.
3. Download the archive for your OS/arch.
	- Linux: `runx-linux-amd64-<tag>.tar.gz`, `runx-linux-arm64-<tag>.tar.gz`
	- macOS: `runx-darwin-amd64-<tag>.tar.gz`, `runx-darwin-arm64-<tag>.tar.gz`
4. Extract and install the binary:

```bash
tar xzf runx-<os>-<arch>-<tag>.tar.gz
sudo install -m 0755 runx /usr/local/bin/runx
```

5. Verify:

```bash
runx --version
```

#### Windows

1. Open the releases page: `https://github.com/horihiro/runx/releases`
2. Open the release for the version you want.
3. Download `runx-windows-amd64-<tag>.zip`.
4. Extract `runx.exe` and place it in a directory included in your `PATH`.
5. Verify:

```powershell
runx --version
```

## Build

### Cross-compilation

```bash
# From any OS to Windows
GOOS=windows GOARCH=amd64 go build -o runx.exe main.go

# From any OS to Linux
GOOS=linux GOARCH=amd64 go build -o runx main.go

# From any OS to macOS
GOOS=darwin GOARCH=arm64 go build -o runx main.go
```

## Quick Start

### 1. Create an Environment File

Create a `.myenv` file in your project directory:

```bash
# App/runtime settings
NODE_ENV=development
API_BASE_URL=https://dev-api.example.com
LOG_LEVEL=debug
```

Or any other name like `.env`, `dev.env`, etc.

### 2. Run Commands with Environment

```bash
# Create a persistent shim
runx add node --envfile=.myenv

# Now 'node' automatically loads .myenv
node app.js

# Or

# One-time execution
runx exec --envfile=.myenv node app.js
```

## Commands

### `runx exec` - Execute with Environment

Run a command with merged environment variables from one or more files.

```bash
runx exec [--envfile=NAME ...] COMMAND [ARGS...]
```

**Examples:**

```bash
# Single environment file
runx exec --envfile=.myenv node app.js

# Multiple files (merged in order, later overrides earlier)
runx exec --envfile=base.env --envfile=dev.env node app.js

# No environment files (just pass through)
runx exec echo "Hello"
```

### `runx add` - Create Command Shim

Create a command shim that automatically loads specified environment files.

```bash
runx add ORIGINAL_COMMAND [--alias=SHIM_NAME] [--envfile=NAME ...] [--shell=bash|zsh|fish]
```

**Examples:**

```bash
# Windows
runx add terraform --envfile=.env

# Linux/macOS (auto-detects shell from $SHELL)
runx add terraform --envfile=.env

# Specify shell explicitly
runx add kubectl --envfile=k8s.env --shell=zsh

# Multiple environment files
runx add node --envfile=base.env --envfile=dev.env

# Alias shim name (mytf executes original terraform)
runx add terraform --alias=mytf --envfile=.env
```

**What happens on Windows:**

1. Checks if original command exists in Machine PATH or User PATH
2. If in **Machine PATH**: Recommends creating a Machine shim (requires admin privileges)
3. If in **User PATH** or not found: Creates User shim in `%LOCALAPPDATA%\runx\shim`
4. Automatically adds shim directory to User PATH if needed
5. Handles PATH priority conflicts intelligently

**What happens on Linux/macOS:**

1. Creates a shell function in your shell config file (`~/.bashrc`, `~/.zshrc`, or `~/.config/fish/config.fish`)
2. Function calls `runx exec` with specified environment files
3. No PATH modification needed

### `runx remove` - Remove Command Shim

Remove a previously created command shim.

```bash
runx remove COMMAND [--shell=bash|zsh|fish]
```

**Examples:**

```bash
# Windows
runx remove az

# Linux/macOS
runx remove az --shell=bash
```

### `runx list` - List Command Shims

List all command shims created by runx.

```bash
runx list [--shell=bash|zsh|fish]
```

**Examples:**

```bash
# Windows
runx list

# Linux/macOS
runx list --shell=zsh
```

## Environment File Format

Environment files use simple `KEY=VALUE` format:

```bash
# Comments start with #
AWS_PROFILE=staging
AWS_REGION=us-east-1

# Quotes are optional
API_BASE_URL=https://api.example.com
API_KEY="your-api-key-here"

# export prefix is supported (bash compatibility)
export NODE_ENV=production
```

### Envfile Argument Rules

- **Supported**: file name (`.env`, `dev.env`, `myconfig`) or absolute path (`/home/user/.env`, `C:\config\app.env`)
- **Not supported**: relative paths with separators (`../parent.env`, `configs/app.env`)

### Search Order

When you specify a file name (for example `--envfile=.env`), runx searches in this order:

1. **Current directory**: `./.env`
2. **Parent directories**: Searches up to filesystem root
3. **Home directory**: `~/.env`

First match wins. This allows project-specific configs to override global defaults.

When you specify an absolute path (for example `--envfile=/path/to/app.env`), runx checks only that file.

### Merge Behavior

When multiple `--envfile` options are provided, runx merges variables in the order given.

- Later files override earlier files for the same key.
- Merged values override the current process environment for matching keys.
- File names and absolute paths can be mixed.

Example:

```bash
runx exec --envfile=base.env --envfile=/abs/path/override.env node app.js
```

If both files define `API_URL`, the value from `/abs/path/override.env` is used because `/abs/path/override.env` is later.

## Debug Mode

Set `RUNX_DEBUG` environment variable to see detailed information:

```bash
# Level 1: Show resolved envfiles and merged variables
export RUNX_DEBUG=1
runx exec --envfile=.env terraform plan

# Level 2: Also show file search trace
export RUNX_DEBUG=2
runx exec --envfile=.env terraform plan
```

Output example:

```
[runx][debug] resolved envfiles:
[runx][debug]   .env: /home/user/project/.env
[runx][debug] envfile search trace:
[runx][debug]   .env:
[runx][debug]     - /home/user/project/subdir/.env
[runx][debug]     - /home/user/project/.env
[runx][debug] merged environment entries:
[runx][debug]   - AWS_PROFILE=staging (from: .env)
[runx][debug]   - AWS_REGION=us-east-1 (from: .env)
```

## Usage Examples

### Terraform with Per-Project Environments

```bash
# Create .env files per project
$ cat ~/project-a/.env
TF_WORKSPACE=project-a
AWS_PROFILE=project-a

$ cat ~/project-b/.env
TF_WORKSPACE=project-b
AWS_PROFILE=project-b

# Create terraform shim
$ runx add terraform --envfile=.env

# 'terraform' now picks env from the current directory tree
$ cd ~/project-a && terraform plan  # Uses project-a env
$ cd ~/project-b && terraform plan  # Uses project-b env
```

### AWS CLI with Profile Switching

```bash
$ cat .awsenv
AWS_PROFILE=production
AWS_REGION=us-east-1

$ runx add aws --envfile=.awsenv
$ aws s3 ls  # Uses production profile
```

### Layered Environment Files (Merge)

```bash
# base.env - shared configuration
NODE_ENV=production
LOG_LEVEL=info
API_URL=https://api.example.com

# secrets.env - sensitive overrides
API_KEY=secret-key
DB_PASSWORD=secret-password

# later file wins for duplicate keys
$ runx add node --envfile=base.env --envfile=secrets.env
$ node app.js
```

### Alias Shim Name

```bash
# Create project-specific shim name that executes terraform
runx add terraform --alias=mytf --envfile=.env

# Original 'terraform' still works normally
# 'mytf' runs original 'terraform' with environment loaded by runx
```

### Shell-Specific Shims

```bash
runx add kubectl --envfile=k8s.env --shell=bash
runx add kubectl --envfile=k8s.env --shell=zsh
runx add kubectl --envfile=k8s.env --shell=fish
```

### One-Off Execution (No Shim)

```bash
$ runx exec --envfile=test.env pytest
$ runx exec --envfile=staging.env curl https://api.example.com
```

## Architecture

See architecture details for Windows shim and Bash function behavior:

- [docs/architecture.md](docs/architecture.md)

## Windows: User Shim vs Machine Shim

See detailed guidance:

- [docs/windows-shim-vs-machine-shim.md](docs/windows-shim-vs-machine-shim.md)

## Troubleshooting

See detailed troubleshooting guide:

- [docs/troubleshooting.md](docs/troubleshooting.md)

