package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strings"
	"text/template"
	"unicode"

	claude "go-claude"

	"hiveminer/pkg/types"
)

// ClaudeRanker implements Ranker using algorithmic scoring + Claude agentic assessment
type ClaudeRanker struct {
	runner  Runner
	prompts fs.FS
	model   string
	logger  claude.EventHandler
}

// NewClaudeRanker creates a new ranker
func NewClaudeRanker(runner Runner, prompts fs.FS, model string, logger claude.EventHandler) *ClaudeRanker {
	return &ClaudeRanker{
		runner:  runner,
		prompts: prompts,
		model:   model,
		logger:  logger,
	}
}

// RankEntries scores entries algorithmically, then sends to Claude for quality assessment
func (r *ClaudeRanker) RankEntries(ctx context.Context, form *types.Form, entries []RankInput) ([]RankOutput, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	// Step 1: Algorithmic scoring
	outputs := r.ScoreAlgorithmic(form, entries)

	// Step 2: Diversity penalty — penalize duplicate primary values
	applyDiversityPenalty(form, entries, outputs)

	// Step 3: Thread saturation penalty — penalize multiple entries from same thread
	applyThreadSaturation(entries, outputs)

	// Step 4: Agentic assessment
	assessed, err := r.AssessWithClaude(ctx, form, entries, outputs)
	if err != nil {
		// If Claude assessment fails, return algorithmic scores only
		fmt.Printf("  Warning: agentic assessment failed: %v\n", err)
		fmt.Println("  Using algorithmic scores only")
		return outputs, nil
	}

	return assessed, nil
}

// ScoreAlgorithmic computes pure algorithmic scores for entries (no Claude needed)
func (r *ClaudeRanker) ScoreAlgorithmic(form *types.Form, entries []RankInput) []RankOutput {
	outputs := make([]RankOutput, len(entries))

	for i, input := range entries {
		// Confidence component (40%): average confidence across non-null fields
		var confSum float64
		var confCount int
		for _, fv := range input.Entry.Fields {
			if fv.Value != nil {
				confSum += fv.Confidence
				confCount++
			}
		}
		var confidenceScore float64
		if confCount > 0 {
			confidenceScore = (confSum / float64(confCount)) * 100
		}

		// Completeness component (25%): non-null fields / total, required weighted 2x
		var totalWeight float64
		var filledWeight float64
		fieldMap := make(map[string]types.FieldValue)
		for _, fv := range input.Entry.Fields {
			fieldMap[fv.ID] = fv
		}
		for _, field := range form.Fields {
			weight := 1.0
			if field.Required {
				weight = 2.0
			}
			totalWeight += weight
			if fv, ok := fieldMap[field.ID]; ok && fv.Value != nil {
				filledWeight += weight
			}
		}
		var completenessScore float64
		if totalWeight > 0 {
			completenessScore = (filledWeight / totalWeight) * 100
		}

		// Upvotes component (20%): log-scaled, caps at ~1000
		var upvoteScore float64
		if input.ThreadScore > 0 {
			upvoteScore = math.Min(math.Log2(float64(input.ThreadScore)+1)/math.Log2(1001), 1.0) * 100
		}

		// Comments component (15%): log-scaled, caps at ~500
		var commentScore float64
		if input.NumComments > 0 {
			commentScore = math.Min(math.Log2(float64(input.NumComments)+1)/math.Log2(501), 1.0) * 100
		}

		// Weighted sum
		algoScore := confidenceScore*0.40 + completenessScore*0.25 + upvoteScore*0.20 + commentScore*0.15

		// Clamp to 0-100
		algoScore = math.Max(0, math.Min(100, algoScore))

		outputs[i] = RankOutput{
			ThreadPostID: input.ThreadPostID,
			EntryIndex:   input.EntryIndex,
			AlgoScore:    algoScore,
			FinalScore:   algoScore,
		}
	}

	return outputs
}

type indexedEntry struct {
	idx       int
	rawValue  string
	normValue string
	algoScore float64
}

// applyDiversityPenalty groups entries by normalized primary value and penalizes
// all but the best entry in each group. This catches obvious duplicates like
// "Walt Disney World" vs "Walt Disney World (Magic Kingdom, EPCOT, ...)"
// without relying on the LLM.
func applyDiversityPenalty(form *types.Form, entries []RankInput, outputs []RankOutput) {
	// Find the primary field ID (first required field, or just first field)
	primaryID := ""
	for _, f := range form.Fields {
		if f.Required {
			primaryID = f.ID
			break
		}
	}
	if primaryID == "" && len(form.Fields) > 0 {
		primaryID = form.Fields[0].ID
	}
	if primaryID == "" {
		return
	}

	// Extract and normalize primary values
	var items []indexedEntry
	for i, input := range entries {
		raw := primaryFieldString(input.Entry, primaryID)
		if raw == "" {
			continue
		}
		items = append(items, indexedEntry{
			idx:       i,
			rawValue:  raw,
			normValue: normalizePrimary(raw),
			algoScore: outputs[i].AlgoScore,
		})
	}

	// Group by normalized value using prefix containment
	// Two entries match if one normalized value contains the other,
	// or if they share a long common prefix (>= 70% of shorter string)
	groups := groupBySimlarity(items)

	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}

		// Sort group by algo score descending — best entry first
		sort.Slice(group, func(i, j int) bool {
			return group[i].algoScore > group[j].algoScore
		})

		// Penalize all but the best
		for rank, item := range group {
			if rank == 0 {
				continue // best entry — no penalty
			}

			idx := item.idx
			// Escalating penalty: -15 for 2nd, -25 for 3rd, -35 for 4th+
			penalty := -15.0 - float64(rank-1)*10.0
			if penalty < -50 {
				penalty = -50
			}

			outputs[idx].Penalty += penalty
			outputs[idx].FinalScore = math.Max(0, outputs[idx].AlgoScore+outputs[idx].Penalty)
			outputs[idx].Flags = appendUnique(outputs[idx].Flags, "duplicate")
			outputs[idx].Reason = fmt.Sprintf("Similar to higher-scored entry: %s", group[0].rawValue)
		}
	}
}

// applyThreadSaturation penalizes entries when too many come from the same thread.
// A single thread with 20 entries shouldn't dominate the top results. The best
// entry from each thread is untouched; the 2nd gets -5, the 3rd -10, etc.
func applyThreadSaturation(entries []RankInput, outputs []RankOutput) {
	// Group output indices by thread, sorted by current FinalScore descending
	type scored struct {
		idx        int
		finalScore float64
	}
	threadGroups := map[string][]scored{}
	for i, input := range entries {
		threadGroups[input.ThreadPostID] = append(threadGroups[input.ThreadPostID], scored{i, outputs[i].FinalScore})
	}

	for _, group := range threadGroups {
		if len(group) <= 1 {
			continue
		}

		// Sort by current score descending — best entry first
		sort.Slice(group, func(i, j int) bool {
			return group[i].finalScore > group[j].finalScore
		})

		// Penalize 2nd+ entries from this thread
		for rank := 1; rank < len(group); rank++ {
			idx := group[rank].idx
			penalty := -5.0 * float64(rank) // -5, -10, -15, -20, ...
			if penalty < -30 {
				penalty = -30 // cap at -30
			}

			outputs[idx].Penalty += penalty
			outputs[idx].FinalScore = math.Max(0, outputs[idx].AlgoScore+outputs[idx].Penalty)
		}
	}
}

// primaryFieldString extracts the string value of the primary field from an entry
func primaryFieldString(entry types.Entry, fieldID string) string {
	for _, fv := range entry.Fields {
		if fv.ID == fieldID && fv.Value != nil {
			switch v := fv.Value.(type) {
			case string:
				return v
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

// normalizePrimary reduces a primary value to a canonical form for comparison.
// "Walt Disney World (Magic Kingdom, EPCOT, ...)" → "walt disney world"
// "Alaska Cruise via Princess Cruises" → "alaska cruise"
func normalizePrimary(s string) string {
	s = strings.ToLower(s)

	// Strip parenthetical suffixes: "foo (bar, baz)" → "foo"
	if idx := strings.Index(s, "("); idx > 0 {
		s = s[:idx]
	}

	// Strip "via ..." / "- ..." suffixes
	for _, sep := range []string{" via ", " - ", " -- "} {
		if idx := strings.Index(s, sep); idx > 0 {
			s = s[:idx]
		}
	}

	// Strip common noise words and punctuation
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return -1
	}, s)

	// Collapse whitespace
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// groupBySimlarity clusters entries whose normalized primary values are similar.
// Two entries match if one is a prefix/substring of the other, or if they share
// a long common prefix (>= 70% of the shorter string).
func groupBySimlarity(items []indexedEntry) [][]indexedEntry {
	n := len(items)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if areSimilar(items[i].normValue, items[j].normValue) {
				union(i, j)
			}
		}
	}

	groupMap := map[int][]indexedEntry{}
	for i := range items {
		root := find(i)
		groupMap[root] = append(groupMap[root], items[i])
	}

	groups := make([][]indexedEntry, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, g)
	}
	return groups
}

// areSimilar returns true if two normalized strings refer to the same thing.
func areSimilar(a, b string) bool {
	if a == b {
		return true
	}

	// One contains the other entirely
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}

	// Long common prefix: if the shorter string is >=4 chars and they share
	// >= 70% of the shorter string as a prefix, they're likely the same
	shorter, longer := a, b
	if len(a) > len(b) {
		shorter, longer = b, a
	}
	if len(shorter) < 4 {
		return false
	}

	commonLen := 0
	for i := 0; i < len(shorter) && i < len(longer); i++ {
		if shorter[i] != longer[i] {
			break
		}
		commonLen++
	}

	return float64(commonLen) >= float64(len(shorter))*0.7
}

// appendUnique appends s to slice if not already present
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// rankPromptData holds data for the rank.md template
type rankPromptData struct {
	FormTitle       string
	FormDescription string
	Fields          []types.Field
	Entries         []rankPromptEntry
}

type rankPromptEntry struct {
	Index     int
	AlgoScore float64
	Fields    []rankPromptField
}

type rankPromptField struct {
	ID         string
	Value      any
	Confidence float64
}

// claudeAssessment represents Claude's response for a single flagged entry
type claudeAssessment struct {
	Index   int      `json:"index"`
	Flags   []string `json:"flags"`
	Penalty float64  `json:"penalty"`
	Reason  string   `json:"reason"`
}

// AssessWithClaude sends all entries to Claude for quality/spam assessment
func (r *ClaudeRanker) AssessWithClaude(ctx context.Context, form *types.Form, inputs []RankInput, outputs []RankOutput) ([]RankOutput, error) {
	// Build prompt data
	promptEntries := make([]rankPromptEntry, len(inputs))
	for i, input := range inputs {
		fields := make([]rankPromptField, 0, len(input.Entry.Fields))
		for _, fv := range input.Entry.Fields {
			fields = append(fields, rankPromptField{
				ID:         fv.ID,
				Value:      fv.Value,
				Confidence: fv.Confidence,
			})
		}
		promptEntries[i] = rankPromptEntry{
			Index:     i,
			AlgoScore: outputs[i].AlgoScore,
			Fields:    fields,
		}
	}

	data := rankPromptData{
		FormTitle:       form.Title,
		FormDescription: form.Description,
		Fields:          form.Fields,
		Entries:         promptEntries,
	}

	// Render prompt
	prompt, err := r.renderPrompt(data)
	if err != nil {
		return nil, fmt.Errorf("rendering rank prompt: %w", err)
	}

	// Call Claude
	opts := []claude.RunOption{claude.WithModel(r.model)}
	if r.logger != nil {
		opts = append(opts, claude.WithEventHandler(r.logger))
	}
	result, err := r.runner.Run(ctx, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("calling claude: %w", err)
	}

	// Parse response
	assessments, err := parseAssessments(result.Text)
	if err != nil {
		return nil, fmt.Errorf("parsing assessment: %w", err)
	}

	// Apply penalties
	scored := make([]RankOutput, len(outputs))
	copy(scored, outputs)

	for _, a := range assessments {
		if a.Index < 0 || a.Index >= len(scored) {
			continue
		}
		penalty := a.Penalty
		if penalty > 0 {
			penalty = -penalty // Ensure penalty is negative
		}
		if penalty < -50 {
			penalty = -50
		}
		if penalty > -10 && len(a.Flags) > 0 {
			penalty = -10 // Minimum penalty if flagged
		}

		scored[a.Index].Penalty = penalty
		scored[a.Index].FinalScore = math.Max(0, scored[a.Index].AlgoScore+penalty)
		scored[a.Index].Flags = a.Flags
		scored[a.Index].Reason = a.Reason
	}

	return scored, nil
}

func (r *ClaudeRanker) renderPrompt(data rankPromptData) (string, error) {
	funcMap := template.FuncMap{
		"json": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(b)
		},
	}

	pt, err := claude.LoadPromptTemplate(r.prompts, "rank.md", funcMap)
	if err != nil {
		return "", fmt.Errorf("loading rank template: %w", err)
	}

	return pt.Render(data)
}

func parseAssessments(response string) ([]claudeAssessment, error) {
	var assessments []claudeAssessment
	err := claude.ExtractJSONArray(response, &assessments)
	if err != nil {
		// No flagged entries — that's valid (everything is clean)
		if err == claude.ErrNoJSON {
			return nil, nil
		}
		return nil, fmt.Errorf("parsing assessments: %w", err)
	}

	return assessments, nil
}
