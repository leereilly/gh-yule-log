# GitHub Yule Log

<div align="center">
  <img src="screencap.gif" alt="Yule Log GIF" width="60%"/>
</div>

A tiny [GitHub CLI](https://cli.github.com/) extension that turns your terminal into a festive, animated Yule log using a terminal-based fire effect :fire:

Vibe-coded with GitHub Copilot Agent and GPT-5.1 the week before Christmas 2025.

## Requirements

- `gh` (GitHub CLI) installed and configured
- Go toolchain (Go 1.21+) installed
- A modern terminal that supports ANSI colors

## Installation

From the directory containing this repository (for local development):

```bash
git clone https://github.com/leereilly/gh-yule-log
cd gh-yule-log
gh extension install .
```

Or from GitHub (once this repo is pushed, replace the owner as needed):

```bash
gh extension install leereilly/gh-yule-log
```

## Usage

Run the extension with:

```bash
gh yule-log
```

- Your terminal will fill with a flickering, colored fire effect.
- Press any key to exit.
 
## Inspiration

I was surfing Netflix the other night and was astonished at how many [branded Yule logs there were](https://youtu.be/ytMdeo9Re1k?si=Fowy4F-40MmdwMcp). I figured GitHub should get in on that action!
