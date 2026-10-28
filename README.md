# scrub-reddit

Simple CLI tool to delete Reddit comments and posts older than N days.
Zero dependencies. Standard library only. No Reddit developer app required.

## How to get your Token (from Browser Cookies)

1. Open `reddit.com` in your browser while logged in.
2. Press `F12` (or right click -> Inspect) to open Developer Tools.
3. Go to **Application** (Chrome/Brave) or **Storage** (Firefox).
4. In the left sidebar, expand **Cookies** and click `https://www.reddit.com`.
5. Find the cookie named **`token_v2`** and copy its **Value**.

## Config

Create a `.env` file in this folder:

```ini
REDDIT_TOKEN=your_token_v2_value_here
```

*(Username is automatically detected from your token).*

## Usage

Preview items to delete (dry-run by default):

```sh
go run . -days 14
```

Delete items (prompts for confirmation):

```sh
go run . -days 14 -delete
```

Delete without confirmation prompt (for cron):

```sh
go run . -days 14 -delete -yes
```

## Flags

- `-days`    Days of history to keep (default: 14)
- `-dry-run` Preview mode only (default: true)
- `-delete`  Enable deletion mode
- `-yes`     Skip [y/N] confirmation prompt
- `-token`   Pass token_v2 directly via CLI flag
