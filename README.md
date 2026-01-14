# Tmux Yule Log

> Forked from [leereilly/gh-yule-log](https://github.com/leereilly/gh-yule-log)

![Yule Log GIF](screencap.gif)

A tmux screensaver plugin that turns your terminal into a festive, animated Yule log. Displays scrolling git commits from your current repository over the fire animation.

## Requirements

- tmux 3.2+ (for popup and command-alias support)
- Go 1.18+ (for building from source)
- A modern terminal that supports ANSI colors

## Installation

### Using TPM (recommended)

Add to your `~/.tmux.conf`:

```bash
set -g @plugin 'gfanton/tmux-yule-log'

# Optional: auto-start screensaver after 5 minutes of inactivity
set -g @yule-log-idle-time "300"
```

Then press `prefix + I` to install.

### Manual Installation

```bash
git clone https://github.com/gfanton/tmux-yule-log
cd tmux-yule-log
go build -o bin/yule-log ./cmd/yule-log
go build -o bin/yule-log-idle ./cmd/yule-log-idle
```

Then source the plugin in your `~/.tmux.conf`:

```bash
run-shell /path/to/tmux-yule-log/yule-log.tmux
```

## Usage

### Key Bindings

| Key | Action |
|-----|--------|
| `prefix + Y` | Trigger screensaver |
| `prefix + Alt+Y` | Toggle idle watcher on/off |

### tmux Commands

Press `prefix + :` then type any of these commands (tab-completion works):

| Command | Action |
|---------|--------|
| `:yule-log` | Trigger screensaver |
| `:yule-start` | Start idle watcher |
| `:yule-stop` | Stop idle watcher |
| `:yule-toggle` | Toggle idle watcher on/off |
| `:yule-status` | Check if idle watcher is running |

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
```

### Command-Line Options

When running the binary directly:

| Flag | Description |
|------|-------------|
| `--contribs` | Use GitHub contribution graph-style green visualization |
| `--dir PATH` | Git directory for commit ticker (default: current pane path) |
| `--no-ticker` | Disable git commit ticker (fire animation only) |

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
