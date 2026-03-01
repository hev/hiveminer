package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"hiveminer/internal/search"
	"hiveminer/pkg/types"
)

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	subreddit := fs.String("subreddit", "", "Limit search to specific subreddit")
	rShort := fs.String("r", "", "Limit search to specific subreddit (shorthand)")
	limit := fs.Int("limit", 10, "Number of results")
	lShort := fs.Int("l", 10, "Number of results (shorthand)")
	nsfw := fs.Bool("nsfw", true, "Include NSFW posts")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	fs.Usage = func() {
		fmt.Println(`Search Reddit for posts

Usage:
  hiveminer search "<query>" [options]

Options:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("query is required")
	}

	query := fs.Arg(0)
	sub := *subreddit
	if sub == "" {
		sub = *rShort
	}
	lim := *limit
	if lim == 10 && *lShort != 10 {
		lim = *lShort
	}

	searcher := search.NewRedditSearcher()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var posts []types.Post
	var err error

	if sub == "" {
		sub = "all"
	}
	posts, err = searcher.Search(ctx, query, sub, lim)

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	filtered := filterNSFW(posts, *nsfw)
	if *jsonOut {
		return printJSON(filtered)
	}

	for _, p := range filtered {
		nsfwTag := ""
		if p.NSFW {
			nsfwTag = " [NSFW]"
		}
		fmt.Printf("%s%s\n", p.Title, nsfwTag)
		fmt.Printf("  ↑ %d  💬 %d  r/%s  (%s)\n", p.Score, p.NumComments, p.Subreddit, p.Domain)
		fmt.Printf("  https://reddit.com%s\n\n", p.Permalink)
	}

	if len(filtered) == 0 {
		fmt.Println("No results found.")
	}

	return nil
}

func cmdLs(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	sort := fs.String("sort", "hot", "Sort by: hot, new, rising, top, controversial")
	sShort := fs.String("s", "hot", "Sort (shorthand)")
	limit := fs.Int("limit", 10, "Number of posts")
	lShort := fs.Int("l", 10, "Number of posts (shorthand)")
	nsfw := fs.Bool("nsfw", true, "Include NSFW posts")
	jsonOut := fs.Bool("json", false, "Output results as JSON")

	fs.Usage = func() {
		fmt.Println(`List posts from a subreddit

Usage:
  hiveminer ls <subreddit> [options]

Options:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("subreddit name is required")
	}

	subreddit := fs.Arg(0)
	sortBy := *sort
	if sortBy == "hot" && *sShort != "hot" {
		sortBy = *sShort
	}
	lim := *limit
	if lim == 10 && *lShort != 10 {
		lim = *lShort
	}

	searcher := search.NewRedditSearcher()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := searcher.ListSubreddit(ctx, subreddit, sortBy, lim)
	if err != nil {
		return fmt.Errorf("failed to list subreddit: %w", err)
	}

	filtered := filterNSFW(results, *nsfw)
	if *jsonOut {
		return printJSON(filtered)
	}

	for _, p := range filtered {
		nsfwTag := ""
		if p.NSFW {
			nsfwTag = " [NSFW]"
		}
		fmt.Printf("%s%s\n", p.Title, nsfwTag)
		fmt.Printf("  ↑ %d  💬 %d  r/%s  (%s)\n", p.Score, p.NumComments, p.Subreddit, p.Domain)
		fmt.Printf("  https://reddit.com%s\n\n", p.Permalink)
	}

	if len(filtered) == 0 {
		fmt.Println("No posts found.")
	}

	return nil
}

func cmdThread(args []string) error {
	fs := flag.NewFlagSet("thread", flag.ExitOnError)
	searchQuery := fs.String("search", "", "Filter comments containing this text")
	sShort := fs.String("s", "", "Filter comments (shorthand)")
	limit := fs.Int("limit", 25, "Number of comments to fetch")
	lShort := fs.Int("l", 25, "Number of comments (shorthand)")
	jsonOut := fs.Bool("json", false, "Output thread JSON")

	fs.Usage = func() {
		fmt.Println(`View thread comments

Usage:
  hiveminer thread <permalink> [options]

Options:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("permalink is required")
	}

	permalink := fs.Arg(0)
	filter := *searchQuery
	if filter == "" {
		filter = *sShort
	}
	lim := *limit
	if lim == 25 && *lShort != 25 {
		lim = *lShort
	}

	searcher := search.NewRedditSearcher()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	thread, err := searcher.GetThread(ctx, permalink, lim)
	if err != nil {
		return fmt.Errorf("failed to fetch thread: %w", err)
	}

	if *jsonOut {
		return printJSON(thread)
	}

	fmt.Printf("%s\n", thread.Post.Title)
	fmt.Printf("  ↑ %d  💬 %d  r/%s  by u/%s\n", thread.Post.Score, thread.Post.NumComments, thread.Post.Subreddit, thread.Post.Author)
	if thread.Post.Selftext != "" {
		text := thread.Post.Selftext
		if len(text) > 500 {
			text = text[:500] + "..."
		}
		fmt.Printf("\n%s\n", text)
	}
	fmt.Println("\n---")

	printCommentList(thread.Comments, filter)

	return nil
}

func filterNSFW(posts []types.Post, includeNSFW bool) []types.Post {
	if includeNSFW {
		return posts
	}
	filtered := make([]types.Post, 0, len(posts))
	for _, p := range posts {
		if !p.NSFW {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func printCommentList(comments []*types.Comment, filter string) {
	for _, c := range comments {
		if filter != "" && !strings.Contains(strings.ToLower(c.Body), strings.ToLower(filter)) {
			printCommentList(c.Replies, filter)
			continue
		}

		indent := strings.Repeat("  ", c.Depth)
		body := c.Body
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		fmt.Printf("%s↑ %d  u/%s\n", indent, c.Score, c.Author)
		for _, line := range strings.Split(body, "\n") {
			fmt.Printf("%s  %s\n", indent, line)
		}
		fmt.Println()

		printCommentList(c.Replies, filter)
	}
}
