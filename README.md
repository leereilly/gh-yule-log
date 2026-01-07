# GitHub Yule Log

![Yule Log GIF](screencap.gif)

A [GitHub CLI](https://cli.github.com/) extension that turns your terminal into a festive, animated Yule log :fire:

Enjoy your Git logs over toasted marshmallows and your favorite beverage :beers:

Vibe-coded with GitHub Copilot Agent and GPT-5.1 the week before Christmas 2025.

## Requirements

- `gh` (GitHub CLI) installed and configured
- A modern terminal that supports ANSI colors

## Installation

```bash
gh extension install leereilly/gh-yule-log
```

For local development:

```bash
git clone leereilly/gh-yule-log
cd gh-yule-log
gh extension install .
```

## Usage

Run the extension with:

```bash
gh yule-log
```

![](images/gh-yule-log-vanilla.gif)

And thanks to [@shplok](https://github.com/shplok) via [#7](https://github.com/leereilly/gh-yule-log/pull/7) you can now press <kbd>↑</kbd> or <kbd>↓</kbd> to adjust flame intensity.

Try the experimental `--contribs` flag to see a Yule log themed around your GitHub contributions:

```bash
gh yule-log --contribs
```

![](images/gh-yule-log-contribs.gif)
 
## Inspiration

I was surfing Netflix the other night and was astonished at how many [branded Yule logs there were](https://youtu.be/ytMdeo9Re1k?si=Fowy4F-40MmdwMcp). I figured GitHub should get in on that action! Also inspired by [@msimpson's curses-based ASCII art fire art from back in the day](https://gist.github.com/msimpson/1096950).

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
