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

// Custom UnmarshalJSON for ESTotal to handle both formats
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

// Helper function to get total hits regardless of format
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
