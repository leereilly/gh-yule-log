#!/usr/bin/env bash
#
# Tmux option variables for yule-log plugin
#

# Option names (user can set these in .tmux.conf)
readonly yule_log_idle_time_option="@yule-log-idle-time"
readonly yule_log_mode_option="@yule-log-mode"
readonly yule_log_show_ticker_option="@yule-log-show-ticker"

# Default values
readonly default_idle_time="300"      # 5 minutes
readonly default_mode="fire"          # "fire" or "contribs"
readonly default_show_ticker="on"     # "on" or "off"

# Minimum supported tmux version
readonly supported_tmux_version="3.2"
