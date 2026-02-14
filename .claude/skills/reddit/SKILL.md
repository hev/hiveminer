---
name: reddit
description: Browse Reddit from the command line. Use when the user wants to search Reddit, find posts on a subreddit, view trending content, or explore Reddit discussions.
allowed-tools: Bash(./threadminer *)
---

# Reddit CLI Skill

Browse Reddit directly from the terminal using the threadminer CLI.

## Available Commands

### List posts from a subreddit
```bash
./threadminer ls <subreddit> --sort <sort> --limit <n> --nsfw=<bool>
```

Arguments:
- `subreddit`: Subreddit name (e.g., `golang`, `programming`, `webdev`)
- `--sort, -s`: Sort by `hot`, `new`, `rising`, `top`, or `controversial` (default: hot)
- `--limit, -l`: Number of posts to show (default: 10)
- `--nsfw`: Include NSFW posts (default: true, use `--nsfw=false` to filter)

### Search Reddit
```bash
./threadminer search "<query>" --subreddit <subreddit> --limit <n> --nsfw=<bool>
```

Arguments:
- `query`: Search term
- `--subreddit, -r`: Limit search to specific subreddit (default: all)
- `--limit, -l`: Number of results (default: 10)
- `--nsfw`: Include NSFW posts (default: true)

### View/Search Thread Comments
```bash
./threadminer thread <permalink> --search "<query>" --limit <n>
```

Arguments:
- `permalink`: Thread URL or permalink (e.g., `/r/golang/comments/abc123/title`)
- `--search, -s`: Filter comments containing this text
- `--limit, -l`: Number of comments to fetch (default: 25)

## Examples

**Browse hot posts on r/programming:**
```bash
./threadminer ls programming
```

**Get top posts from r/golang:**
```bash
./threadminer ls golang --sort top --limit 15
```

**Search for discussions:**
```bash
./threadminer search "machine learning tutorials"
```

**Search within a specific subreddit:**
```bash
./threadminer search "async await" --subreddit javascript
```

**Filter out NSFW content:**
```bash
./threadminer ls askreddit --nsfw=false
```

**View comments on a thread:**
```bash
./threadminer thread /r/golang/comments/1pdzpbh
```

**Search within thread comments:**
```bash
./threadminer thread /r/golang/comments/1pdzpbh --search "errors.Is"
```

## Output

Each post shows:
- Title (with [NSFW] tag if applicable)
- Score (upvotes) and comment count
- Subreddit
- Domain/source
- Direct link to the post

Each comment shows:
- Score and author
- Comment body (truncated if long)
