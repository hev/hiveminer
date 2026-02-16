package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"threadminer/internal/session"
	"threadminer/pkg/types"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorWhite  = "\033[37m"
	colorMag    = "\033[35m"
	colorBgDim  = "\033[48;5;236m"
)

func cmdRuns(args []string) error {
	if len(args) < 1 {
		printRunsUsage()
		return nil
	}

	switch args[0] {
	case "ls", "list":
		return cmdRunsLs(args[1:])
	case "show":
		return cmdRunsShow(args[1:])
	case "help", "-h", "--help":
		printRunsUsage()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown runs subcommand: %s\n", args[0])
		printRunsUsage()
		return fmt.Errorf("unknown runs subcommand: %s", args[0])
	}
}

func printRunsUsage() {
	fmt.Println(`threadminer runs - View extraction runs and results

Usage:
  threadminer runs <command> [options]

Commands:
  ls       List all runs in the output directory
  show     Show extraction results for a run

Examples:
  threadminer runs ls
  threadminer runs ls -o ./output
  threadminer runs show family-vacation-20260214-045927
  threadminer runs show family-vacation -n 0       # show all results
  threadminer runs show ./output/family-vacation-20260214-045927`)
}

type sessionInfo struct {
	Dir      string
	Name     string
	Manifest *types.Manifest
}

func cmdRunsLs(args []string) error {
	fs := flag.NewFlagSet("runs ls", flag.ExitOnError)
	outputDir := fs.String("output", "./output", "Output directory to scan")
	fs.StringVar(outputDir, "o", "./output", "Output directory (shorthand)")
	fs.Parse(args)

	entries, err := os.ReadDir(*outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No output directory found. Run an extraction first.")
			return nil
		}
		return fmt.Errorf("reading output directory: %w", err)
	}

	var sessions []sessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(*outputDir, entry.Name())
		manifest, err := session.LoadManifest(dir)
		if err != nil || manifest == nil {
			continue
		}
		sessions = append(sessions, sessionInfo{
			Dir:      dir,
			Name:     entry.Name(),
			Manifest: manifest,
		})
	}

	if len(sessions) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	// Sort by created_at descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Manifest.CreatedAt.After(sessions[j].Manifest.CreatedAt)
	})

	fmt.Printf("\n%s%s Runs %s\n", colorBold, colorCyan, colorReset)
	fmt.Println(strings.Repeat("─", 80))

	for idx := len(sessions) - 1; idx >= 0; idx-- {
		s := sessions[idx]
		m := s.Manifest
		counts := session.CountByStatus(m)

		// Status indicator
		statusColor := colorGreen
		statusIcon := "done"
		if len(m.Runs) > 0 {
			lastRun := m.Runs[len(m.Runs)-1]
			switch lastRun.Status {
			case "running":
				statusColor = colorYellow
				statusIcon = "running"
			case "interrupted":
				statusColor = colorYellow
				statusIcon = "interrupted"
			case "failed":
				statusColor = colorRed
				statusIcon = "failed"
			}
		}

		fmt.Printf("\n %s%s#%d%s  %s%s%s\n", colorBold, colorDim, idx+1, colorReset, colorBold, s.Name, colorReset)
		fmt.Printf("     %sForm:%s  %s\n", colorCyan, colorReset, m.Form.Title)
		if m.Query != "" {
			fmt.Printf("     %sQuery:%s %s\n", colorCyan, colorReset, m.Query)
		}
		if len(m.Subreddits) > 0 {
			subs := m.Subreddits
			display := strings.Join(subs, ", ")
			if len(display) > 60 {
				display = strings.Join(subs[:3], ", ") + fmt.Sprintf(" (+%d more)", len(subs)-3)
			}
			fmt.Printf("     %sSubs:%s  %s\n", colorCyan, colorReset, display)
		}

		threadSummary := fmt.Sprintf("%d total", len(m.Threads))
		parts := []string{}
		if counts["ranked"] > 0 {
			parts = append(parts, fmt.Sprintf("%s%d ranked%s", colorGreen, counts["ranked"], colorReset))
		}
		if counts["extracted"] > 0 {
			parts = append(parts, fmt.Sprintf("%s%d extracted%s", colorGreen, counts["extracted"], colorReset))
		}
		if counts["collected"] > 0 {
			parts = append(parts, fmt.Sprintf("%s%d collected%s", colorCyan, counts["collected"], colorReset))
		}
		if counts["pending"] > 0 {
			parts = append(parts, fmt.Sprintf("%s%d pending%s", colorYellow, counts["pending"], colorReset))
		}
		if counts["skipped"] > 0 {
			parts = append(parts, fmt.Sprintf("%s%d skipped%s", colorDim, counts["skipped"], colorReset))
		}
		if counts["failed"] > 0 {
			parts = append(parts, fmt.Sprintf("%s%d failed%s", colorRed, counts["failed"], colorReset))
		}
		if len(parts) > 0 {
			threadSummary += " (" + strings.Join(parts, ", ") + ")"
		}
		fmt.Printf("     %sThreads:%s %s\n", colorCyan, colorReset, threadSummary)

		fmt.Printf("     %sStatus:%s  %s%s%s", colorCyan, colorReset, statusColor, statusIcon, colorReset)
		fmt.Printf("  %s%s%s\n", colorDim, m.CreatedAt.Format("Jan 02 15:04"), colorReset)
	}

	fmt.Println()
	return nil
}

func cmdRunsShow(args []string) error {
	fs := flag.NewFlagSet("runs show", flag.ExitOnError)
	outputDir := fs.String("output", "./output", "Output directory")
	showInternal := fs.Bool("all", false, "Show internal fields")
	maxResults := fs.Int("n", 10, "Maximum number of results to show (0 for all)")
	fs.StringVar(outputDir, "o", "./output", "Output directory (shorthand)")
	fs.BoolVar(showInternal, "a", false, "Show internal fields (shorthand)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: run ID required")
		fmt.Fprintln(os.Stderr, "Usage: threadminer runs show <run-id>")
		fmt.Fprintln(os.Stderr, "  Run 'threadminer runs ls' to see available runs")
		return fmt.Errorf("run ID required")
	}

	target := fs.Arg(0)

	// Resolve session directory - accept full path or just directory name
	sessionDir := target
	if _, err := os.Stat(filepath.Join(target, "manifest.json")); os.IsNotExist(err) {
		// Try as a subdirectory of output
		sessionDir = filepath.Join(*outputDir, target)
		if _, err := os.Stat(filepath.Join(sessionDir, "manifest.json")); os.IsNotExist(err) {
			// Try prefix match
			matched := findSessionByPrefix(*outputDir, target)
			if matched == "" {
				fmt.Fprintf(os.Stderr, "Error: no run found matching %q\n", target)
				fmt.Fprintln(os.Stderr, "  Run 'threadminer runs ls' to see available runs")
				return fmt.Errorf("run not found: %s", target)
			}
			sessionDir = matched
		}
	}

	manifest, err := session.LoadManifest(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading manifest: %v\n", err)
		return err
	}
	if manifest == nil {
		fmt.Fprintf(os.Stderr, "Error: no manifest found in %s\n", sessionDir)
		return fmt.Errorf("no manifest found")
	}

	// Load the form to get field metadata
	form, err := loadFormFromManifest(manifest)
	if err != nil {
		// Fall back to deriving fields from the extraction data
		form = deriveFormFromManifest(manifest)
	}

	// Filter to extracted or ranked threads
	var extracted []types.ThreadState
	for _, t := range manifest.Threads {
		if (t.Status == "extracted" || t.Status == "ranked") && len(t.Entries) > 0 {
			extracted = append(extracted, t)
		}
	}

	if len(extracted) == 0 {
		fmt.Printf("\n%s%s%s\n", colorBold, manifest.Form.Title, colorReset)
		fmt.Println("No extracted results yet.")
		return nil
	}

	// Build visible fields list
	var fields []types.Field
	for _, f := range form.Fields {
		if f.Internal && !*showInternal {
			continue
		}
		fields = append(fields, f)
	}

	// Print header
	fmt.Printf("\n%s%s %s %s\n", colorBold, colorCyan, manifest.Form.Title, colorReset)
	if manifest.Query != "" {
		fmt.Printf(" %sQuery: %s%s\n", colorDim, manifest.Query, colorReset)
	}
	fmt.Printf(" %s%d threads extracted%s\n", colorDim, len(extracted), colorReset)
	fmt.Println()

	// Collect all entries for sorting
	type rankedEntry struct {
		entry  types.Entry
		thread types.ThreadState
	}
	var allEntries []rankedEntry
	for _, thread := range extracted {
		for _, entry := range thread.Entries {
			allEntries = append(allEntries, rankedEntry{entry: entry, thread: thread})
		}
	}

	// Sort by rank score descending (highest first), unscored entries last
	sort.Slice(allEntries, func(i, j int) bool {
		si := allEntries[i].entry.RankScore
		sj := allEntries[j].entry.RankScore
		if si == nil && sj == nil {
			return false
		}
		if si == nil {
			return false
		}
		if sj == nil {
			return true
		}
		return *si > *sj
	})

	// Limit displayed results
	totalEntries := len(allEntries)
	truncated := false
	if *maxResults > 0 && totalEntries > *maxResults {
		allEntries = allEntries[:*maxResults]
		truncated = true
	}

	// Display entries in reverse so #1 appears at the bottom (closest to prompt)
	for i := len(allEntries) - 1; i >= 0; i-- {
		re := allEntries[i]
		entryNum := i
		entry := re.entry
		thread := re.thread

		// Build field map for quick lookup
		fieldMap := make(map[string]types.FieldValue)
		for _, fv := range entry.Fields {
			fieldMap[fv.ID] = fv
		}

		// Entry header with rank score and thread source
		title := thread.Title
		if len(title) > 72 {
			title = title[:72] + "..."
		}
		scoreLabel := ""
		if entry.RankScore != nil {
			scoreLabel = fmt.Sprintf(" %s%.0fpts%s", colorGreen, *entry.RankScore, colorReset)
		}
		fmt.Printf("%s%s %-3s%s %s%s\n", colorBold, colorMag, fmt.Sprintf("[%d]", entryNum+1), scoreLabel, title, colorReset)

		// Show flags if present
		if len(entry.RankFlags) > 0 {
			var flagParts []string
			for _, f := range entry.RankFlags {
				flagColor := colorYellow
				switch f {
				case "spam", "off_topic":
					flagColor = colorRed
				case "joke", "outdated":
					flagColor = colorRed
				case "duplicate", "low_effort":
					flagColor = colorYellow
				}
				flagParts = append(flagParts, fmt.Sprintf("%s[%s]%s", flagColor, f, colorReset))
			}
			fmt.Printf("    %s\n", strings.Join(flagParts, " "))
		}
		fmt.Printf("    %sr/%s  ↑%d pts  %d comments%s\n",
			colorDim, thread.Subreddit, thread.Score, thread.NumComments, colorReset)
		fmt.Println()

		// Field values
		for _, field := range fields {
			fv, ok := fieldMap[field.ID]
			label := formatFieldLabel(field.ID)

			if !ok || fv.Value == nil {
				fmt.Printf("    %s%-20s%s %s—%s\n", colorCyan, label, colorReset, colorDim, colorReset)
				continue
			}

			valueStr := formatValue(fv.Value)
			confColor := confidenceColor(fv.Confidence)

			// Confidence badge
			confBadge := fmt.Sprintf("%s%.0f%%%s", confColor, fv.Confidence*100, colorReset)

			// Check if value is multiline (arrays)
			lines := strings.Split(valueStr, "\n")
			if len(lines) > 1 {
				fmt.Printf("    %s%-20s%s %s\n", colorCyan, label, colorReset, confBadge)
				for _, line := range lines {
					fmt.Printf("      %s%s%s\n", colorWhite, line, colorReset)
				}
			} else {
				fmt.Printf("    %s%-20s%s %s  %s\n", colorCyan, label, colorReset, valueStr, confBadge)
			}
		}

		// Sources: collect unique comment evidence across all fields
		type commentSource struct {
			Author string
			Quote  string
			Link   string
		}
		seen := make(map[string]bool)
		var sources []commentSource
		for _, fv := range entry.Fields {
			for i, ev := range fv.Evidence {
				if ev.CommentID == "" || ev.CommentID == "post_content" {
					continue
				}
				if seen[ev.CommentID] {
					continue
				}
				seen[ev.CommentID] = true
				link := ""
				if i < len(fv.Links) {
					link = fv.Links[i]
				}
				quote := ev.Text
				if len(quote) > 60 {
					quote = quote[:60] + "..."
				}
				sources = append(sources, commentSource{
					Author: ev.Author,
					Quote:  quote,
					Link:   link,
				})
			}
		}
		if len(sources) > 0 {
			fmt.Printf("\n    %sSources:%s\n", colorDim, colorReset)
			for _, src := range sources {
				author := src.Author
				if author != "" && !strings.HasPrefix(author, "u/") {
					author = "u/" + author
				}
				if author != "" {
					fmt.Printf("      %s%s%s  %s\"%s\"%s\n", colorCyan, author, colorReset, colorWhite, src.Quote, colorReset)
				} else {
					fmt.Printf("      %s\"%s\"%s\n", colorWhite, src.Quote, colorReset)
				}
				if src.Link != "" {
					fullURL := "https://reddit.com" + src.Link
					fmt.Printf("      %s%s%s\n", colorDim, hyperlink(fullURL, fullURL), colorReset)
				}
			}
		}

		fmt.Printf("\n  %s%s%s\n\n", colorDim, strings.Repeat("·", 76), colorReset)
	}

	if truncated {
		fmt.Printf(" %sShowing top %d of %d results. Run %sruns show <id> -n 0%s%s to see all.%s\n\n",
			colorDim, *maxResults, totalEntries, colorReset, colorBold, colorDim, colorReset)
	}

	fmt.Println()
	return nil
}

// findSessionByPrefix finds a session directory matching a prefix
func findSessionByPrefix(outputDir, prefix string) string {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}

	prefix = strings.ToLower(prefix)
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, filepath.Join(outputDir, entry.Name()))
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		// Return most recent (last alphabetically since names contain timestamps)
		sort.Strings(matches)
		return matches[len(matches)-1]
	}
	return ""
}

// loadFormFromManifest attempts to load the original form file
func loadFormFromManifest(manifest *types.Manifest) (*types.Form, error) {
	if manifest.Form.Path == "" {
		return nil, fmt.Errorf("no form path in manifest")
	}

	data, err := os.ReadFile(manifest.Form.Path)
	if err != nil {
		return nil, err
	}

	var form types.Form
	if err := json.Unmarshal(data, &form); err != nil {
		return nil, err
	}
	return &form, nil
}

// deriveFormFromManifest creates a minimal form from extraction data
func deriveFormFromManifest(manifest *types.Manifest) *types.Form {
	seen := make(map[string]bool)
	var fields []types.Field

	for _, t := range manifest.Threads {
		for _, entry := range t.Entries {
			for _, fv := range entry.Fields {
				if !seen[fv.ID] {
					seen[fv.ID] = true
					fields = append(fields, types.Field{
						ID:   fv.ID,
						Type: types.FieldTypeString,
					})
				}
			}
		}
	}

	return &types.Form{
		Title:  manifest.Form.Title,
		Fields: fields,
	}
}

// formatFieldLabel converts a field ID like "best_age_range" to "Best Age Range"
func formatFieldLabel(id string) string {
	parts := strings.Split(id, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// formatValue renders an extracted value as a display string
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%.1f", val)
	case []any:
		if len(val) == 0 {
			return "—"
		}
		var lines []string
		for _, item := range val {
			lines = append(lines, fmt.Sprintf("• %v", item))
		}
		return strings.Join(lines, "\n")
	case map[string]any:
		if len(val) == 0 {
			return "—"
		}
		// Find max key length for alignment
		maxKey := 0
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
			if len(k) > maxKey {
				maxKey = len(k)
			}
		}
		sort.Strings(keys)
		var lines []string
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("%-*s  %v", maxKey, k, val[k]))
		}
		return strings.Join(lines, "\n")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// hyperlink renders an OSC 8 terminal hyperlink
func hyperlink(url, text string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// confidenceColor returns an ANSI color based on confidence level
func confidenceColor(conf float64) string {
	switch {
	case conf >= 0.8:
		return colorGreen
	case conf >= 0.5:
		return colorYellow
	default:
		return colorRed
	}
}

// timeAgo returns a human-readable relative time string
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
