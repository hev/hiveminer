package types

import "time"

// Post represents a Reddit post
type Post struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	Domain      string  `json:"domain"`
	Permalink   string  `json:"permalink"`
	Selftext    string  `json:"selftext"`
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	NSFW        bool    `json:"over_18"`
	Created     float64 `json:"created_utc"`
}

// Comment represents a Reddit comment
type Comment struct {
	ID        string     `json:"id"`
	Body      string     `json:"body"`
	Author    string     `json:"author"`
	Score     int        `json:"score"`
	Created   float64    `json:"created_utc"`
	Permalink string     `json:"permalink"`
	Replies   []*Comment `json:"replies,omitempty"`
	Depth     int        `json:"depth"`
}

// Thread represents a complete Reddit thread with post and comments
type Thread struct {
	Post     Post       `json:"post"`
	Comments []*Comment `json:"comments"`
}

// FieldType represents the type of a form field
type FieldType string

const (
	FieldTypeString  FieldType = "string"
	FieldTypeNumber  FieldType = "number"
	FieldTypeBoolean FieldType = "boolean"
	FieldTypeArray   FieldType = "array"
)

// Field represents a single field in a form schema
type Field struct {
	ID          string    `json:"id"`
	Type        FieldType `json:"type"`
	Question    string    `json:"question"`
	SearchHints []string  `json:"search_hints,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Internal    bool      `json:"internal,omitempty"` // Don't show in viewer
}

// Form represents a complete extraction form schema
type Form struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	SearchHints []string `json:"search_hints,omitempty"`
	Fields      []Field  `json:"fields"`
}

// Evidence represents a quote from a thread supporting an extracted value
type Evidence struct {
	Text      string `json:"text"`
	CommentID string `json:"comment_id,omitempty"`
	Author    string `json:"author,omitempty"`
	Score     int    `json:"score,omitempty"`
}

// FieldValue represents an extracted field value
type FieldValue struct {
	ID         string     `json:"id"`
	Value      any        `json:"value"`
	Confidence float64    `json:"confidence"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	Reasoning  string     `json:"reasoning,omitempty"`
}

// Entry represents a single distinct item extracted from a thread.
// For example, one destination recommendation with all its associated fields.
type Entry struct {
	Fields []FieldValue `json:"fields"`
}

// ExtractionResult holds all extracted entries for a thread.
// Each entry represents one distinct recommendation/item found in the thread.
type ExtractionResult struct {
	Entries []Entry `json:"entries"`
}

// ThreadState represents the extraction state of a single thread
type ThreadState struct {
	PostID      string        `json:"post_id"`
	Permalink   string        `json:"permalink"`
	Title       string        `json:"title"`
	Subreddit   string        `json:"subreddit"`
	Score       int           `json:"score"`
	NumComments int           `json:"num_comments"`
	Status      string        `json:"status"` // pending, collected, extracted, failed
	CollectedAt *time.Time    `json:"collected_at,omitempty"`
	ExtractedAt *time.Time    `json:"extracted_at,omitempty"`
	Entries     []Entry        `json:"entries,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// FormRef holds reference to the form used in a session
type FormRef struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Hash  string `json:"hash"`
}

// RunLog records metadata about a single extraction run
type RunLog struct {
	InvocationID     string    `json:"invocation_id"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at,omitempty"`
	Status           string    `json:"status"` // running, completed, interrupted, failed
	ThreadsProcessed int       `json:"threads_processed"`
}

// Manifest tracks the complete state of an extraction session
type Manifest struct {
	Version              int           `json:"version"`
	Form                 FormRef       `json:"form"`
	Query                string        `json:"query,omitempty"`
	Subreddits           []string      `json:"subreddits"`
	DiscoveredSubreddits bool          `json:"discovered_subreddits,omitempty"`
	Threads              []ThreadState `json:"threads"`
	Runs                 []RunLog      `json:"runs"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

// TokenUsage tracks API token usage
type TokenUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Model        string  `json:"model"`
	CostUSD      float64 `json:"cost_usd"`
}
