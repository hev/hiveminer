#!/usr/bin/env python3
"""Fetch Reddit posts from the command line."""

import argparse
import json
import urllib.request
import sys


def fetch_subreddit(subreddit: str, sort: str = "hot", limit: int = 10) -> dict:
    """Fetch posts from a subreddit."""
    url = f"https://www.reddit.com/r/{subreddit}/{sort}.json?limit={limit}"
    req = urllib.request.Request(
        url,
        headers={"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"}
    )
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode())


def fetch_search(query: str, subreddit: str = "all", limit: int = 10) -> dict:
    """Search Reddit for posts."""
    encoded_query = urllib.parse.quote(query)
    url = f"https://www.reddit.com/r/{subreddit}/search.json?q={encoded_query}&limit={limit}&restrict_sr=1"
    req = urllib.request.Request(
        url,
        headers={"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"}
    )
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.loads(response.read().decode())


def display_posts(data: dict, title: str):
    """Display posts in a formatted way."""
    print(f"\n{title}\n")
    print("-" * 70)

    children = data.get("data", {}).get("children", [])
    if not children:
        print("\nNo posts found.")
        return

    for i, child in enumerate(children, 1):
        post = child["data"]
        post_title = post["title"]
        if len(post_title) > 65:
            post_title = post_title[:65] + "..."

        print(f"\n{i:2}. {post_title}")
        print(f"    ⬆️  {post['score']:,} points | 💬 {post['num_comments']:,} comments")
        print(f"    🔗 {post['domain']}")
        print(f"    📎 https://reddit.com{post['permalink']}")


def main():
    import urllib.parse

    parser = argparse.ArgumentParser(description="Browse Reddit from the command line")
    subparsers = parser.add_subparsers(dest="command", help="Commands")

    # List command
    list_parser = subparsers.add_parser("ls", help="List posts from a subreddit")
    list_parser.add_argument("subreddit", nargs="?", default="programming", help="Subreddit name (default: programming)")
    list_parser.add_argument("--sort", "-s", choices=["hot", "new", "top", "rising", "controversial"], default="hot", help="Sort method")
    list_parser.add_argument("--limit", "-l", type=int, default=10, help="Number of posts")

    # Search command
    search_parser = subparsers.add_parser("search", help="Search Reddit")
    search_parser.add_argument("query", help="Search query")
    search_parser.add_argument("--subreddit", "-r", default="all", help="Subreddit to search in (default: all)")
    search_parser.add_argument("--limit", "-l", type=int, default=10, help="Number of results")

    args = parser.parse_args()

    if args.command == "ls":
        data = fetch_subreddit(args.subreddit, args.sort, args.limit)
        display_posts(data, f"📋 r/{args.subreddit} ({args.sort})")
    elif args.command == "search":
        data = fetch_search(args.query, args.subreddit, args.limit)
        display_posts(data, f"🔍 Search: '{args.query}' in r/{args.subreddit}")
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
