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
#   set -g @yule-log-idle-time "300"       # seconds before screensaver (0=disabled)
#   set -g @yule-log-mode "fire"           # "fire" or "contribs"
#   set -g @yule-log-show-ticker "on"      # show git commits ticker
#   set -g @yule-log-lock-enabled "off"    # enable lock mode (requires password)
#   set -g @yule-log-lock-timeout "0"      # auto-lock timeout (0=manual only)
#   set -g @yule-log-lock-socket-protect "on" # restrict socket during lock
#
# Usage:
#   prefix + Y       - trigger screensaver manually
#   prefix + Alt+Y   - toggle idle watcher on/off
#   prefix + L       - lock session (if lock enabled)
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

# Find or build yule-log binary
get_binary() {
    local bin_dir="$CURRENT_DIR/bin"

    # Check PATH first (for nix/go install users)
    if command -v yule-log >/dev/null 2>&1; then
        command -v yule-log
        return 0
    fi

    # Check local bin/
    if [ -x "$bin_dir/yule-log" ]; then
        echo "$bin_dir/yule-log"
        return 0
    fi

    # Try to build if Go is available
    if command -v go >/dev/null 2>&1; then
        mkdir -p "$bin_dir"
        echo "Building yule-log..." >&2
        if (cd "$CURRENT_DIR" && go build -o "$bin_dir/yule-log" .); then
            echo "$bin_dir/yule-log"
            return 0
        fi
    fi

    return 1
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

get_lock_enabled() {
    get_tmux_option "@yule-log-lock-enabled" "off"
}

get_lock_timeout() {
    get_tmux_option "@yule-log-lock-timeout" "0"
}

get_lock_socket_protect() {
    get_tmux_option "@yule-log-lock-socket-protect" "on"
}

# Build screensaver command with options
build_screensaver_cmd() {
    local cmd="$YULE_LOG_BIN run"

    if [ "$(get_mode)" = "contribs" ]; then
        cmd="$cmd --contribs"
    fi

    if [ "$(get_show_ticker)" = "off" ]; then
        cmd="$cmd --no-ticker"
    fi

    # Add current pane path for git context
    cmd="$cmd --dir '#{pane_current_path}'"

    echo "$cmd"
}

# Build lock command with options
build_lock_cmd() {
    local cmd="$YULE_LOG_BIN lock"

    if [ "$(get_mode)" = "contribs" ]; then
        cmd="$cmd --contribs"
    fi

    if [ "$(get_show_ticker)" = "off" ]; then
        cmd="$cmd --no-ticker"
    fi

    if [ "$(get_lock_socket_protect)" = "off" ]; then
        cmd="$cmd --socket-protect=false"
    fi

    echo "$cmd"
}

# Check if password is configured
is_password_configured() {
    "$YULE_LOG_BIN" lock status 2>&1 | grep -q "Password: configured"
}

# Lock the session
lock_session() {
    if [ "$(get_lock_enabled)" != "on" ]; then
        tmux display-message "Lock mode is disabled. Set @yule-log-lock-enabled 'on' to enable."
        return 1
    fi

    if ! is_password_configured; then
        tmux display-message "No password configured. Run: $YULE_LOG_BIN lock set-password"
        return 1
    fi

    local lock_cmd
    lock_cmd=$(build_lock_cmd)

    # Use display-popup for the lock screen
    tmux display-popup -E -w 100% -h 100% "$lock_cmd"
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
    local idle_cmd="$YULE_LOG_BIN idle --timeout $idle_time"

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

    local lock_cmd
    lock_cmd=$(build_lock_cmd)

    # Bind prefix + Y to trigger screensaver popup
    tmux bind-key Y display-popup -E -w 100% -h 100% "$cmd"

    # Bind prefix + Alt+Y to toggle idle watcher
    tmux bind-key M-Y run-shell "$CURRENT_DIR/yule-log.tmux toggle"

    # Bind prefix + L to lock session (if lock is enabled)
    tmux bind-key L run-shell "$CURRENT_DIR/yule-log.tmux lock"

    # Command aliases (use with prefix + : then type the command)
    # Example: prefix + : then "yule-log" or "yule-stop"
    tmux set -s command-alias[100] "yule-log=display-popup -E -w 100% -h 100% \"$cmd\""
    tmux set -s command-alias[101] "yule-start=run-shell \"$CURRENT_DIR/yule-log.tmux start\""
    tmux set -s command-alias[102] "yule-stop=run-shell \"$CURRENT_DIR/yule-log.tmux stop\""
    tmux set -s command-alias[103] "yule-toggle=run-shell \"$CURRENT_DIR/yule-log.tmux toggle\""
    tmux set -s command-alias[104] "yule-status=run-shell \"$CURRENT_DIR/yule-log.tmux status\""
    tmux set -s command-alias[105] "yule-lock=run-shell \"$CURRENT_DIR/yule-log.tmux lock\""
    tmux set -s command-alias[106] "yule-set-password=run-shell \"$YULE_LOG_BIN lock set-password\""
}

# Setup hook to clean up when tmux server exits
setup_cleanup_hook() {
    local pid_file
    pid_file=$(get_pid_file)

    # Hook to stop watcher when last session closes
    tmux set-hook -g session-closed "run-shell '$CURRENT_DIR/yule-log.tmux stop 2>/dev/null || true'"
}

main() {
    # Find yule-log binary (needed for most commands)
    # This is non-fatal for stop/status commands
    YULE_LOG_BIN=$(get_binary) || true
    export YULE_LOG_BIN

    # Handle command-line arguments for start/stop/toggle/status/lock
    case "${1:-}" in
        start)
            if [ -z "$YULE_LOG_BIN" ]; then
                tmux display-message "yule-log binary not found. Install via: go install or nix"
                return 1
            fi
            start_idle_watcher
            return
            ;;
        stop)
            stop_idle_watcher
            return
            ;;
        toggle)
            if [ -z "$YULE_LOG_BIN" ]; then
                tmux display-message "yule-log binary not found. Install via: go install or nix"
                return 1
            fi
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
        lock)
            if [ -z "$YULE_LOG_BIN" ]; then
                tmux display-message "yule-log binary not found. Install via: go install or nix"
                return 1
            fi
            lock_session
            return
            ;;
    esac

    # Default: initialize plugin
    if ! check_tmux_version; then
        return 1
    fi

    # Verify binary is found for plugin initialization
    if [ -z "$YULE_LOG_BIN" ]; then
        tmux display-message "yule-log binary not found. Install via: go install github.com/gfanton/tmux-yule-log@latest or nix profile install github:gfanton/tmux-yule-log#yule-log"
        return 1
    fi

    # Setup key bindings
    setup_key_bindings

    # Setup cleanup hook
    setup_cleanup_hook

    # Start idle watcher if @yule-log-idle-time is set
    start_idle_watcher
}

main "$@"
