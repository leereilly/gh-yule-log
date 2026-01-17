# Tmux Yule Log

> Forked from [leereilly/gh-yule-log](https://github.com/leereilly/gh-yule-log)

![Yule Log GIF](screencap.gif)

A tmux screensaver plugin that turns your terminal into a festive, animated Yule log. Displays scrolling git commits from your current repository over the fire animation.

## Requirements

- tmux 3.2+ (for popup and command-alias support)
- Go 1.24+ (for building from source, or use nix/pre-built binary)
- A modern terminal that supports ANSI colors

## Installation

### Using TPM (recommended)

Add to your `~/.tmux.conf`:

```bash
set -g @plugin 'gfanton/tmux-yule-log'

# Optional: auto-start screensaver after 5 minutes of inactivity
set -g @yule-log-idle-time "300"
```

Then press `prefix + I` to install. The binary will be built automatically if Go is available.

### Using Nix

```bash
# Install full tmux plugin (includes binary)
nix profile install github:gfanton/tmux-yule-log

# Or just the binary
nix profile install github:gfanton/tmux-yule-log#yule-log
```

### Manual Installation

```bash
git clone https://github.com/gfanton/tmux-yule-log ~/.tmux/plugins/tmux-yule-log
```

Then source the plugin in your `~/.tmux.conf`:

```bash
run-shell ~/.tmux/plugins/tmux-yule-log/yule-log.tmux
```

## Usage

### Key Bindings

| Key | Action |
|-----|--------|
| `prefix + Y` | Trigger screensaver |
| `prefix + Alt+Y` | Toggle idle watcher on/off |
| `prefix + L` | Lock session (if lock enabled) |

### tmux Commands

Press `prefix + :` then type any of these commands (tab-completion works):

| Command | Action |
|---------|--------|
| `:yule-log` | Trigger screensaver |
| `:yule-start` | Start idle watcher |
| `:yule-stop` | Stop idle watcher |
| `:yule-toggle` | Toggle idle watcher on/off |
| `:yule-status` | Check if idle watcher is running |
| `:yule-lock` | Lock the session |
| `:yule-set-password` | Set lock password |

### Screensaver Controls

| Key | Action |
|-----|--------|
| <kbd>↑</kbd> | Increase flame intensity |
| <kbd>↓</kbd> | Decrease flame intensity |
| Any other key | Exit screensaver |

The screensaver displays full-screen, covering all panes and windows. Press any key to exit and return to your previous view.

## Configuration

Add to your `~/.tmux.conf`:

```bash
# Idle timeout in seconds before screensaver activates (0 = disabled)
set -g @yule-log-idle-time "300"

# Visualization mode: "fire" or "contribs"
set -g @yule-log-mode "fire"

# Show git commit ticker: "on" or "off"
set -g @yule-log-show-ticker "on"

# Lock mode (see Security Considerations below)
set -g @yule-log-lock-enabled "off"        # Enable lock feature
set -g @yule-log-lock-socket-protect "on"  # Restrict socket during lock
```

## Session Locking

The lock feature provides password-protected session locking with visual feedback through the fire animation.

### Setup

1. **Set a password** (supports regular characters and arrow keys):
   ```bash
   ./bin/yule-log lock set-password
   ```

2. **Enable lock mode** in your `~/.tmux.conf`:
   ```bash
   set -g @yule-log-lock-enabled "on"
   ```

3. **Lock your session** with `prefix + L` or `:yule-lock`

### Security Considerations

**Important:** Before enabling session locking, understand its limitations.

#### What Lock Mode Protects Against

- **Casual access**: Prevents opportunistic use of an unattended terminal
- **Accidental input**: Blocks inadvertent commands from passersby
- **Basic bypass attempts**: Socket permission restriction prevents simple `tmux attach`

#### What Lock Mode Does NOT Protect Against

| Threat | Why It Cannot Be Mitigated |
|--------|---------------------------|
| **SIGKILL / SIGSTOP** | Kernel-level signals cannot be blocked by any userspace process |
| **Root access** | A root user can always bypass any lock mechanism |
| **`tmux kill-session`** | Cannot prevent session destruction (though socket protection helps) |
| **Physical access + time** | Extended physical access defeats most software protections |
| **Memory forensics** | While memguard helps, determined attackers with root can dump memory |

#### Race Condition Window

There is a brief window between a new client attaching and the lock hook executing. Socket permission protection (`@yule-log-lock-socket-protect "on"`) significantly narrows this window by preventing new connections entirely during lock.

#### Password Security

- Passwords are hashed using **Argon2id** (OWASP 2025 recommended parameters)
- Password input uses **memguard** for secure memory handling (mlock, secure wipe)
- Constant-time comparison prevents timing attacks
- Password hash stored in `~/.config/tmux-yule-log/passwd` (mode 0600)

#### Recommendations

1. **Don't rely on this as your only security measure** - use it as one layer in defense-in-depth
2. **Keep socket protection enabled** - it's the primary defense against `tmux attach` bypass
3. **Use a strong password** - arrow keys can be included for additional complexity
4. **Consider your threat model** - this protects against casual access, not determined attackers
5. **Lock your screen too** - combine with OS-level screen lock for better security

### Command-Line Options

When running the binary directly:

**Screensaver (`yule-log run`):**

| Flag | Description |
|------|-------------|
| `--contribs` | Use GitHub contribution graph-style green visualization |
| `--dir PATH` | Git directory for commit ticker (default: current pane path) |
| `--no-ticker` | Disable git commit ticker (fire animation only) |
| `--intensity` | Default fire intensity: 10-85 (default: 75) |
| `--playground` | Playground mode: only ESC exits, all keys affect fire |
| `--cooldown` | Fire cooldown speed: `fast`, `medium`, `slow` |
| `--lock` | Lock mode: require password to exit |

**Lock (`yule-log lock`):**

| Subcommand | Description |
|------------|-------------|
| `set-password` | Set or update the lock password |
| `status` | Show lock and password status |

| Flag | Description |
|------|-------------|
| `--socket-protect` | Restrict tmux socket permissions during lock (default: true) |
| `--contribs` | Use contribution graph visualization |
| `--no-ticker` | Disable git commit ticker |
| `--intensity` | Default fire intensity: 10-85 (default: 75) |
| `--cooldown` | Fire cooldown speed |

## Screenshots

### Fire Mode (default)

![](images/gh-yule-log-vanilla.gif)

### Contribution Mode (`--contribs`)

![](images/gh-yule-log-contribs.gif)

## Credits

- Original project by [@leereilly](https://github.com/leereilly): [gh-yule-log](https://github.com/leereilly/gh-yule-log)
- Flame intensity controls by [@shplok](https://github.com/shplok) via [#7](https://github.com/leereilly/gh-yule-log/pull/7)
- Fire algorithm inspired by [@msimpson's curses-based ASCII art fire](https://gist.github.com/msimpson/1096950)

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
