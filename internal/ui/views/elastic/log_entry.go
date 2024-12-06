package elastic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LogEntry struct {
	data map[string]any
}

func NewLogEntry(jsonData []byte) (*LogEntry, error) {
	var data map[string]any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}
	return &LogEntry{data: data}, nil
}

func (l *LogEntry) GetValue(path string) any {
	parts := strings.Split(path, ".")
	current := l.data

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

func (l *LogEntry) GetAvailableFields() []string {
	fields := make([]string, 0)
	l.getFieldsRecursive(l.data, "", &fields)
	return fields
}

func (l *LogEntry) getFieldsRecursive(data any, prefix string, fields *[]string) {
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
				l.getFieldsRecursive(value, newPrefix, fields)
			}
		}
	case []any:
		if prefix != "" {
			*fields = append(*fields, prefix)
		}
	}
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

func (l *LogEntry) GetFormattedValue(field string) string {
	value := l.GetValue(field) // Use existing GetValue instead of getNestedValue
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
