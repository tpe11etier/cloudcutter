package elastic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DocEntry represents a single document entry with metadata and dynamic fields.
type DocEntry struct {
	data    map[string]any // The main document data
	ID      string         `json:"_id"`      // Document ID
	Index   string         `json:"_index"`   // Index name
	Type    string         `json:"_type"`    // Document type
	Score   *float64       `json:"_score"`   // Relevance score (optional)
	Version *int64         `json:"_version"` // Document version (optional)
}

// NewDocEntry creates a new DocEntry instance by unmarshalling the source data and setting metadata fields.
func NewDocEntry(source []byte, id, index, docType string, score *float64, version *int64) (*DocEntry, error) {
	var data map[string]any
	if err := json.Unmarshal(source, &data); err != nil {
		return nil, err
	}

	return &DocEntry{
		data:    data,
		ID:      id,
		Index:   index,
		Type:    docType,
		Score:   score,
		Version: version,
	}, nil
}

// GetMetadataFields returns a list of metadata fields available for the document.
func (de *DocEntry) GetMetadataFields() []string {
	fields := []string{"_id", "_index", "_type"}
	if de.Score != nil {
		fields = append(fields, "_score")
	}
	if de.Version != nil {
		fields = append(fields, "_version")
	}
	return fields
}

// GetAvailableFields retrieves all fields in the document, including nested ones.
func (de *DocEntry) GetAvailableFields() []string {
	fields := de.GetMetadataFields()
	de.getFieldsRecursive(de.data, "", &fields)
	sort.Strings(fields)
	return fields
}

// isLeafNode determines if a given value is a leaf node (not a map or slice).
func isLeafNode(v any) bool {
	if v == nil {
		return true
	}

	switch v.(type) {
	case map[string]any, []any:
		return false
	default:
		return true
	}
}

// formatSeverity formats severity values to a string representation.
func formatSeverity(sev any) string {
	switch v := sev.(type) {
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// getFieldsRecursive recursively retrieves all field paths in a nested structure.
func (de *DocEntry) getFieldsRecursive(data any, prefix string, fields *[]string) {
	if data == nil {
		return
	}

	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}

			if isLeafNode(value) {
				*fields = append(*fields, newPrefix)
			} else {
				de.getFieldsRecursive(value, newPrefix, fields)
			}
		}
	case []any:
		if prefix != "" {
			*fields = append(*fields, prefix)
		}
	}
}

// GetFormattedValue retrieves and formats a field value based on its name.
func (de *DocEntry) GetFormattedValue(field string) string {
	switch field {
	case "_id":
		return de.ID
	case "_index":
		return de.Index
	case "_type":
		return de.Type
	case "_score":
		if de.Score != nil {
			return fmt.Sprintf("%v", *de.Score)
		}
		return ""
	case "_version":
		if de.Version != nil {
			return fmt.Sprintf("%v", *de.Version)
		}
		return ""
	default:
		value := de.GetValue(field)
		if value == nil {
			return ""
		}

		switch field {
		case "unixTime":
			if ts, ok := value.(float64); ok {
				return time.Unix(int64(ts), 0).Format(time.RFC3339)
			}
		case "severity":
			return formatSeverity(value)
		case "title", "content":
			if str, ok := value.(string); ok {
				return str
			}
		}

		return fmt.Sprintf("%v", value)
	}
}

// GetValue retrieves the value for a specified field path in the document.
func (de *DocEntry) GetValue(path string) any {
	parts := strings.Split(path, ".")
	current := de.data

	for i, part := range parts {
		if current == nil {
			return nil
		}

		if strings.HasSuffix(part, "]") {
			arrayKey, index := parseArrayAccess(part)
			if index >= 0 {
				if arr, ok := current[arrayKey].([]any); ok && index < len(arr) {
					if i == len(parts)-1 {
						return arr[index]
					}
					if mapVal, ok := arr[index].(map[string]any); ok {
						current = mapVal
						continue
					}
				}
			}
			return nil
		}

		if i == len(parts)-1 {
			return current[part]
		}

		switch v := current[part].(type) {
		case map[string]any:
			current = v
		default:
			return nil
		}
	}
	return nil
}

// parseArrayAccess parses an array field path and extracts the key and index.
func parseArrayAccess(field string) (string, int) {
	start := strings.Index(field, "[")
	end := strings.Index(field, "]")
	if start < 0 || end < 0 || end <= start {
		return field, -1
	}

	key := field[:start]
	indexStr := field[start+1 : end]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return field, -1
	}
	return key, index
}
