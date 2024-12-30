package elastic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DocEntry struct {
	data    map[string]any
	ID      string   `json:"_id"`
	Index   string   `json:"_index"`
	Type    string   `json:"_type"`
	Score   *float64 `json:"_score"`
	Version *int64   `json:"_version"`
}

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

func (de *DocEntry) GetAvailableFields() []string {
	fields := de.GetMetadataFields()
	de.getFieldsRecursive(de.data, "", &fields)
	return fields
}

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
		}

		return fmt.Sprintf("%v", value)
	}
}
func (de *DocEntry) GetValue(path string) any {
	parts := strings.Split(path, ".")
	current := de.data

	for i, part := range parts {
		if current == nil {
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

func (de *DocEntry) getFieldsRecursive(data any, prefix string, fields *[]string) {
	if data == nil {
		return
	}

	// Add metadata fields if we're at the root level (empty prefix)
	if prefix == "" {
		metaFields := []string{"_id", "_index", "_type"}
		if de.Score != nil {
			metaFields = append(metaFields, "_score")
		}
		if de.Version != nil {
			metaFields = append(metaFields, "_version")
		}
		*fields = append(*fields, metaFields...)
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
