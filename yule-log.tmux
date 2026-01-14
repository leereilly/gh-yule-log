#!/usr/bin/env bash
#
# Yule Log - tmux screensaver plugin
# https://github.com/gfanton/tmux-yule-log
#
# Forked from: https://github.com/leereilly/gh-yule-log
#
# Installation:
#   set -g @plugin 'gfanton/tmux-yule-log'
#
# Configuration options:
#   set -g @yule-log-idle-time "300"    # seconds before screensaver (0=disabled)
#   set -g @yule-log-mode "fire"        # "fire" or "contribs"
#   set -g @yule-log-show-ticker "on"   # show git commits ticker
#
# Usage:
#   prefix + Y       - trigger screensaver manually
#   prefix + Alt+Y   - toggle idle watcher on/off
#

set -euo pipefail

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

source "$CURRENT_DIR/scripts/tmux/helpers.sh"
source "$CURRENT_DIR/scripts/tmux/variables.sh"

# PID file location (unique per tmux server)
get_pid_file() {
    local tmux_pid
    tmux_pid=$(tmux display-message -p "#{pid}")
    echo "/tmp/yule-log-idle-${tmux_pid}.pid"
}

# Check tmux version (need 3.2+ for display-popup)
check_tmux_version() {
    local version
    version=$(tmux -V | cut -d' ' -f2 | sed 's/[^0-9.]//g')
    local major minor
    major=$(echo "$version" | cut -d. -f1)
    minor=$(echo "$version" | cut -d. -f2)

    if [ "$major" -lt 3 ] || { [ "$major" -eq 3 ] && [ "$minor" -lt 2 ]; }; then
        echo "Warning: tmux 3.2+ required for yule-log popup support"
        return 1
    fi
    return 0
}

# Build Go binary if needed
build_binaries() {
    local bin_dir="$CURRENT_DIR/bin"
    mkdir -p "$bin_dir"

    # Check if Go is available
    if ! command -v go >/dev/null 2>&1; then
        echo "Warning: Go not found. Pre-built binaries required."
        return 1
    fi

    # Build yule-log if not present
    if [ ! -x "$bin_dir/yule-log" ]; then
        echo "Building yule-log..."
        (cd "$CURRENT_DIR" && go build -o "$bin_dir/yule-log" .) || return 1
    fi

    return 0
}

# Get plugin options from tmux
get_idle_time() {
    get_tmux_option "@yule-log-idle-time" "$default_idle_time"
}

get_mode() {
    get_tmux_option "@yule-log-mode" "$default_mode"
}

get_show_ticker() {
    get_tmux_option "@yule-log-show-ticker" "$default_show_ticker"
}

# Build screensaver command with options
build_screensaver_cmd() {
    local cmd="$CURRENT_DIR/bin/yule-log"

    if [ "$(get_mode)" = "contribs" ]; then
        cmd="$cmd --contribs"
    fi

    if [ "$(get_show_ticker)" = "off" ]; then
        cmd="$cmd --no-ticker"
    fi

    # Add current pane path for git context
    cmd="$cmd --dir \"#{pane_current_path}\""

    echo "$cmd"
}

# Check if idle watcher is running
is_watcher_running() {
    local pid_file
    pid_file=$(get_pid_file)

    if [ -f "$pid_file" ]; then
        local pid
        pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            return 0
        fi
        # Stale PID file, remove it
        rm -f "$pid_file"
    fi
    return 1
}

# Stop the idle watcher
stop_idle_watcher() {
    local pid_file
    pid_file=$(get_pid_file)

    if [ -f "$pid_file" ]; then
        local pid
        pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            tmux display-message "Yule log idle watcher stopped (pid: $pid)"
        fi
        rm -f "$pid_file"
    fi
}

# Start idle watcher as a background daemon
start_idle_watcher() {
    local idle_time
    idle_time=$(get_idle_time)

    # If idle time is 0 or empty, don't start the watcher
    if [ -z "$idle_time" ] || [ "$idle_time" = "0" ]; then
        return 0
    fi

    # Check if already running
    if is_watcher_running; then
        return 0
    fi

    # Build idle watcher command
    local idle_cmd="$CURRENT_DIR/bin/yule-log idle --timeout $idle_time"

    if [ "$(get_mode)" = "contribs" ]; then
        idle_cmd="$idle_cmd --contribs"
    fi

    if [ "$(get_show_ticker)" = "off" ]; then
        idle_cmd="$idle_cmd --no-ticker"
    fi

    # Start the idle watcher in background
    nohup $idle_cmd >/dev/null 2>&1 &
    local pid=$!

    # Save PID to file
    local pid_file
    pid_file=$(get_pid_file)
    echo "$pid" > "$pid_file"

    tmux display-message "Yule log idle watcher started (timeout: ${idle_time}s, pid: $pid)"
}

# Toggle idle watcher on/off
toggle_idle_watcher() {
    if is_watcher_running; then
        stop_idle_watcher
    else
        start_idle_watcher
    fi
}

# Setup key bindings and command aliases
setup_key_bindings() {
    local cmd
    cmd=$(build_screensaver_cmd)

    # Bind prefix + Y to trigger screensaver popup
    tmux bind-key Y display-popup -E -w 100% -h 100% "$cmd"

    # Bind prefix + Alt+Y to toggle idle watcher
    tmux bind-key M-Y run-shell "$CURRENT_DIR/yule-log.tmux toggle"

    # Command aliases (use with prefix + : then type the command)
    # Example: prefix + : then "yule-log" or "yule-stop"
    tmux set -s command-alias[100] "yule-log=display-popup -E -w 100% -h 100% \"$cmd\""
    tmux set -s command-alias[101] "yule-start=run-shell \"$CURRENT_DIR/yule-log.tmux start\""
    tmux set -s command-alias[102] "yule-stop=run-shell \"$CURRENT_DIR/yule-log.tmux stop\""
    tmux set -s command-alias[103] "yule-toggle=run-shell \"$CURRENT_DIR/yule-log.tmux toggle\""
    tmux set -s command-alias[104] "yule-status=run-shell \"$CURRENT_DIR/yule-log.tmux status\""
}

# Setup hook to clean up when tmux server exits
setup_cleanup_hook() {
    local pid_file
    pid_file=$(get_pid_file)

    # Hook to stop watcher when last session closes
    tmux set-hook -g session-closed "run-shell '$CURRENT_DIR/yule-log.tmux stop 2>/dev/null || true'"
}

main() {
    # Handle command-line arguments for start/stop/toggle/status
    case "${1:-}" in
        start)
            start_idle_watcher
            return
            ;;
        stop)
            stop_idle_watcher
            return
            ;;
        toggle)
            toggle_idle_watcher
            return
            ;;
        status)
            if is_watcher_running; then
                local pid_file
                pid_file=$(get_pid_file)
                tmux display-message "Yule log idle watcher is running (pid: $(cat "$pid_file"))"
            else
                tmux display-message "Yule log idle watcher is not running"
            fi
            return
            ;;
    esac

    # Default: initialize plugin
    if ! check_tmux_version; then
        return 1
    fi

    if ! build_binaries; then
        echo "Warning: Could not build yule-log binaries"
    fi

    # Setup key bindings
    setup_key_bindings

    # Setup cleanup hook
    setup_cleanup_hook

    # Start idle watcher if @yule-log-idle-time is set
    start_idle_watcher
}

main "$@"
