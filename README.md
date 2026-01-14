# Tmux Yule Log

> Forked from [leereilly/gh-yule-log](https://github.com/leereilly/gh-yule-log)

![Yule Log GIF](screencap.gif)

A tmux screensaver plugin that turns your terminal into a festive, animated Yule log. Displays scrolling git commits from your current repository over the fire animation.

## Requirements

- tmux 3.2+ (for popup support)
- Go 1.18+ (for building from source)
- A modern terminal that supports ANSI colors

## Installation

### Using TPM (recommended)

Add to your `~/.tmux.conf`:

```bash
set -g @plugin 'gfanton/tmux-yule-log'
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

Press `prefix + Y` to trigger the screensaver manually.

The screensaver displays full-screen, covering all panes and windows. Press any key to exit and return to your previous view.

### Controls

- <kbd>↑</kbd> Increase flame intensity
- <kbd>↓</kbd> Decrease flame intensity
- Any other key: Exit

### Command-Line Options

| Flag | Description |
|------|-------------|
| `--contribs` | Use GitHub contribution graph-style green visualization |
| `--dir PATH` | Git directory for commit ticker (default: current pane path) |
| `--no-ticker` | Disable git commit ticker (fire animation only) |

### tmux Configuration

Add to your `~/.tmux.conf`:

```bash
# Idle timeout in seconds before screensaver activates (default: 300, 0=disabled)
set -g @yule-log-idle-time "300"

# Visualization mode: "fire" or "contribs" (default: fire)
set -g @yule-log-mode "fire"

# Show git commit ticker: "on" or "off" (default: on)
set -g @yule-log-show-ticker "on"
```

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
