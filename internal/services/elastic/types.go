package elastic

import (
	"encoding/json"
	"fmt"
)

// ESSearchResult represents the response from Elasticsearch
type ESSearchResult struct {
	Took     int          `json:"took"`
	TimedOut bool         `json:"timed_out"`
	Hits     ESSearchHits `json:"hits"`
	ScrollID string       `json:"_scroll_id,omitempty"`
	Error    *ESError     `json:"error,omitempty"`
}

// ESError holds the error body ES returns on non-2xx responses.
type ESError struct {
	Type         string      `json:"type"`
	Reason       string      `json:"reason"`
	CausedBy     *ESError    `json:"caused_by,omitempty"`
	FailedShards []ESShardError `json:"failed_shards,omitempty"`
}

type ESShardError struct {
	Reason *ESError `json:"reason,omitempty"`
}

func (e *ESError) rootCause() *ESError {
	// Prefer the first failed shard's cause — it has the real reason.
	if len(e.FailedShards) > 0 && e.FailedShards[0].Reason != nil {
		return e.FailedShards[0].Reason.rootCause()
	}
	if e.CausedBy != nil {
		return e.CausedBy.rootCause()
	}
	return e
}

func (e *ESError) Error() string {
	root := e.rootCause()
	if root != e {
		return fmt.Sprintf("%s: %s", e.Type, root.Error())
	}
	if e.Type != "" {
		return fmt.Sprintf("%s: %s", e.Type, e.Reason)
	}
	return e.Reason
}

// ESSearchHits contains the hits part of the response
type ESSearchHits struct {
	Total    ESTotal       `json:"total"`
	MaxScore *float64      `json:"max_score"`
	Hits     []ESSearchHit `json:"hits"`
}

// ESTotal represents the total hits structure in ES7+
type ESTotal struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}

// ESSearchHit represents a single hit in the search results
type ESSearchHit struct {
	Index   string          `json:"_index"`
	Type    string          `json:"_type"`
	ID      string          `json:"_id"`
	Score   *float64        `json:"_score"`
	Source  json.RawMessage `json:"_source"`
	Version *int64          `json:"_version,omitempty"`
}

func (t *ESTotal) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a simple number (ES6 and earlier)
	var value int
	if err := json.Unmarshal(data, &value); err == nil {
		t.Value = value
		t.Relation = "eq"
		return t.Validate()
	}

	// If that fails, try to unmarshal as an object (ES7+)
	var obj struct {
		Value    int    `json:"value"`
		Relation string `json:"relation"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("failed to unmarshal ESTotal as either number or object: %v", err)
	}
	t.Value = obj.Value
	t.Relation = obj.Relation
	return t.Validate()
}

func (h *ESSearchHits) GetTotalHits() int {
	return h.Total.Value
}

func (t *ESTotal) Validate() error {
	if t.Value < 0 {
		return fmt.Errorf("total value cannot be negative: %d", t.Value)
	}
	validRelations := map[string]bool{"eq": true, "gte": true}
	if !validRelations[t.Relation] {
		return fmt.Errorf("invalid relation: %s", t.Relation)
	}
	return nil
}
