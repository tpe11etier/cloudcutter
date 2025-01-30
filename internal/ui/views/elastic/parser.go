package elastic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	validFieldNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(?:\.[a-zA-Z][a-zA-Z0-9_-]*)*$`)

	validOperatorRegex = regexp.MustCompile(`^(>=|<=|>|<|=)$`)
)

type ParseError struct {
	Field   string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error on field '%s': %s", e.Field, e.Message)
}

// BuildQuery combines multiple filters into one Elasticsearch bool-query with error handling
func BuildQuery(filters []string, size int, timeframe string, fieldCache *FieldCache) (map[string]any, error) {
	return BuildQueryWithTime(filters, size, timeframe, time.Now(), fieldCache)
}

// BuildQueryWithTime is like BuildQuery but accepts a specific time for testing
func BuildQueryWithTime(filters []string, size int, timeframe string, now time.Time, fieldCache *FieldCache) (map[string]any, error) {
	if size < 0 {
		return nil, fmt.Errorf("size must be non-negative, got %d", size)
	}

	// We'll store timeframe + any user filters in mustClauses
	var mustClauses []map[string]any

	// If timeframe is set, build a timeframe clause
	if timeframe != "" {
		timeQuery, err := BuildTimeQuery(timeframe, now)
		if err != nil {
			return nil, fmt.Errorf("error building time query: %v", err)
		}
		if timeQuery != nil {
			mustClauses = append(mustClauses, timeQuery)
		}
	}

	// Process user filters
	var parseErrors []string
	for i, f := range filters {
		clause, err := ParseFilter(f, fieldCache)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("filter[%d]: %v", i, err))
			continue
		}
		if clause != nil {
			mustClauses = append(mustClauses, clause)
		}
	}
	if len(parseErrors) > 0 {
		return nil, fmt.Errorf("failed to parse filters: %s", strings.Join(parseErrors, "; "))
	}

	// If we ended up with zero clauses (no timeframe, no filters), match all
	if len(mustClauses) == 0 {
		return map[string]any{
			"query": map[string]any{
				"match_all": map[string]any{},
			},
			"size": size,
		}, nil
	}

	// Otherwise, build a bool query with everything in "must"
	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": mustClauses,
			},
		},
		"size": size,
	}, nil
}

func ParseFilter(filter string, fieldCache *FieldCache) (map[string]any, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil, &ParseError{Field: "", Message: "empty filter"}
	}

	// Handle special case for _id field
	if strings.HasPrefix(filter, "_id=") {
		value := strings.TrimSpace(strings.TrimPrefix(filter, "_id="))
		return map[string]any{
			"ids": map[string]any{
				"values": []string{value},
			},
		}, nil
	}

	// Handle special case for detection_id_dedup field
	if strings.HasPrefix(filter, "detection_id_dedup=") {
		value := strings.TrimSpace(strings.TrimPrefix(filter, "detection_id_dedup="))
		return map[string]any{
			"term": map[string]any{
				"detection_id_dedup": value,
			},
		}, nil
	}

	// Handle range queries first
	if clause, err := parseRangeSyntax(filter, fieldCache); err != nil {
		return nil, err
	} else if clause != nil {
		return clause, nil
	}

	// Split into field and value
	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return nil, &ParseError{Field: filter, Message: "invalid filter format"}
	}

	fieldName := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if !isValidFieldName(fieldName) {
		return nil, &ParseError{Field: fieldName, Message: "invalid field name"}
	}

	metadata, exists := fieldCache.Get(fieldName)
	if !exists {
		metadata = &FieldMetadata{
			Type:         "keyword",
			Searchable:   true,
			Aggregatable: true,
		}
	}

	if !metadata.Searchable {
		return nil, &ParseError{Field: fieldName, Message: "field is not searchable"}
	}

	switch metadata.Type {
	case "long", "integer", "float", "double":
		if num, err := strconv.ParseFloat(value, 64); err == nil {
			return map[string]any{
				"term": map[string]any{
					fieldName: num,
				},
			}, nil
		}
		return nil, &ParseError{Field: fieldName, Message: "invalid numeric value"}

	case "date":
		return parseDateValue(fieldName, value)

	case "boolean":
		valueLower := strings.ToLower(value)
		switch valueLower {
		case "true", "false":
			return map[string]any{
				"term": map[string]any{
					fieldName: valueLower == "true",
				},
			}, nil
		}
		return nil, &ParseError{Field: fieldName, Message: "invalid boolean value (must be 'true' or 'false')"}
	default:
		if isNullValue(value) {
			return buildNullQuery(fieldName), nil
		}

		hasWildcard, startsWithWildcard := hasWildcard(value)
		if hasWildcard {
			if startsWithWildcard {
				return nil, &ParseError{Field: fieldName, Message: "wildcard query cannot start with *"}
			}
			return map[string]any{
				"wildcard": map[string]any{
					fieldName: unescapeValue(value),
				},
			}, nil
		}

		return map[string]any{
			"match": map[string]any{
				fieldName: unescapeValue(value),
			},
		}, nil
	}
}

func isValidFieldName(field string) bool {
	return validFieldNameRegex.MatchString(field)
}

func isNullValue(value string) bool {
	valueLower := strings.ToLower(value)
	return valueLower == "null" || valueLower == "nil"
}

func buildNullQuery(fieldName string) map[string]any {
	nullQuery := make(map[string]any)
	boolQuery := make(map[string]any)
	existsQuery := make(map[string]any)
	fieldQuery := make(map[string]any)

	fieldQuery["field"] = fieldName
	existsQuery["exists"] = fieldQuery
	boolQuery["must_not"] = existsQuery
	nullQuery["bool"] = boolQuery

	return nullQuery
}

func unescapeValue(value string) string {
	if !strings.ContainsRune(value, '\\') {
		return value
	}

	var result strings.Builder
	escaped := false

	for _, ch := range value {
		if escaped {
			if ch == '\\' || ch == '*' || ch == '?' || ch == '=' {
				result.WriteRune(ch)
			} else {
				result.WriteRune('\\')
				result.WriteRune(ch)
			}
			escaped = false
		} else if ch == '\\' {
			escaped = true
		} else {
			result.WriteRune(ch)
		}
	}

	if escaped {
		result.WriteRune('\\')
	}

	return result.String()
}

// ParseTimeframe assumes input has already been validated by ValidateTimeframe
func ParseTimeframe(timeframe string) (time.Duration, error) {
	timeframe = strings.TrimSpace(strings.ToLower(timeframe))

	switch timeframe {
	case "today":
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return now.Sub(startOfDay), nil
	case "week":
		return 7 * 24 * time.Hour, nil
	case "month":
		return 30 * 24 * time.Hour, nil
	case "quarter":
		return 90 * 24 * time.Hour, nil
	case "year":
		return 365 * 24 * time.Hour, nil
	}

	length := len(timeframe)
	value, _ := strconv.Atoi(timeframe[:length-1])
	unit := timeframe[length-1:]

	switch unit {
	case "h", "H":
		return time.Duration(value) * time.Hour, nil
	case "d", "D":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w", "W":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("internal error: invalid unit %s passed validation", unit)
	}
}

// BuildTimeQuery creates an Elasticsearch time range query based on the timeframe
func BuildTimeQuery(timeframe string, now time.Time) (map[string]interface{}, error) {
	if timeframe == "" {
		return nil, nil
	}

	if err := ValidateTimeframe(timeframe); err != nil {
		return nil, err
	}

	duration, err := ParseTimeframe(timeframe)
	if err != nil {
		return nil, err
	}

	unixTime := now.Unix()
	unixTimeMs := now.UnixMilli()

	return map[string]interface{}{
		"bool": map[string]interface{}{
			"should": []map[string]interface{}{
				{
					"range": map[string]interface{}{
						"unixTime": map[string]interface{}{
							"gte": unixTime - int64(duration.Seconds()),
							"lte": unixTime,
						},
					},
				},
				{
					"range": map[string]interface{}{
						"detectionGeneratedTime": map[string]interface{}{
							"gte": unixTimeMs - int64(duration.Milliseconds()),
							"lte": unixTimeMs,
						},
					},
				},
			},
			"minimum_should_match": 1,
		},
	}, nil
}

func parseDateValue(fieldName, value string) (map[string]any, error) {
	// Try parse as unix timestamp first
	if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
		var timestamp int64
		if ts > 1000000000000 { // Likely milliseconds
			timestamp = ts
		} else {
			timestamp = ts * 1000 // Convert to milliseconds
		}
		return map[string]any{
			"term": map[string]any{
				fieldName: timestamp,
			},
		}, nil
	}

	// Try as ISO date
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return map[string]any{
			"term": map[string]any{
				fieldName: value,
			},
		}, nil
	}

	return nil, &ParseError{Field: fieldName, Message: "invalid date format"}
}

func parseRangeSyntax(filter string, fieldCache *FieldCache) (map[string]any, error) {
	// Find the operator
	var opStart int = -1
	for i, c := range filter {
		if c == '>' || c == '<' {
			opStart = i
			break
		}
	}
	if opStart == -1 {
		return nil, nil
	}

	fieldName := strings.TrimSpace(filter[:opStart])
	if !isValidFieldName(fieldName) {
		return nil, &ParseError{Field: fieldName, Message: "invalid field name in range query"}
	}

	// Get field metadata
	metadata, exists := fieldCache.Get(fieldName)
	if !exists {
		metadata = &FieldMetadata{
			Type:         "keyword",
			Searchable:   true,
			Aggregatable: true,
			Active:       false,
		}
	}

	// Only allow range queries on numeric and date fields
	if !strings.Contains(metadata.Type, "int") &&
		!strings.Contains(metadata.Type, "long") &&
		!strings.Contains(metadata.Type, "float") &&
		!strings.Contains(metadata.Type, "double") &&
		metadata.Type != "date" {
		return nil, &ParseError{Field: fieldName, Message: "range queries only supported on numeric and date fields"}
	}

	// Find the operator end
	opEnd := opStart + 1
	if opEnd < len(filter) && (filter[opEnd] == '=' || filter[opEnd] == '>') {
		opEnd++
	}

	operator := filter[opStart:opEnd]
	if !validOperatorRegex.MatchString(operator) {
		return nil, &ParseError{Field: fieldName, Message: "invalid range operator"}
	}

	value := strings.TrimSpace(filter[opEnd:])
	if value == "" {
		return nil, &ParseError{Field: fieldName, Message: "missing value in range query"}
	}

	var rangeValue interface{}
	if metadata.Type == "date" {
		// Try parse as timestamp or ISO date
		if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
			if ts > 1000000000000 {
				rangeValue = ts
			} else {
				rangeValue = ts * 1000
			}
		} else if _, err := time.Parse(time.RFC3339, value); err == nil {
			rangeValue = value
		} else {
			return nil, &ParseError{Field: fieldName, Message: "invalid date format in range query"}
		}
	} else {
		// Parse as number
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, &ParseError{Field: fieldName, Message: fmt.Sprintf("invalid numeric value in range query: %s", value)}
		}
		rangeValue = num
	}

	var rangeOp string
	switch operator {
	case ">=":
		rangeOp = "gte"
	case "<=":
		rangeOp = "lte"
	case ">":
		rangeOp = "gt"
	case "<":
		rangeOp = "lt"
	default:
		return nil, &ParseError{Field: fieldName, Message: "invalid range operator"}
	}

	return map[string]any{
		"range": map[string]any{
			fieldName: map[string]any{
				rangeOp: rangeValue,
			},
		},
	}, nil
}

func hasWildcard(value string) (bool, bool) {
	escaped := false
	hasWildcard := false
	startsWithWildcard := false

	for i, ch := range value {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '*' || ch == '?' {
			hasWildcard = true
			if i == 0 {
				startsWithWildcard = true
			}
		}
	}
	return hasWildcard, startsWithWildcard
}

func ValidateTimeframe(timeframe string) error {
	timeframe = strings.TrimSpace(strings.ToLower(timeframe))
	if timeframe == "" {
		return fmt.Errorf("empty timeframe")
	}

	// Check valid keywords first
	switch timeframe {
	case "today", "week", "month", "quarter", "year":
		return nil
	}

	// Check for invalid variations of keywords
	keywords := []string{"today", "week", "month", "quarter", "year"}
	for _, keyword := range keywords {
		if strings.HasPrefix(timeframe, keyword) && timeframe != keyword {
			return fmt.Errorf("invalid timeframe: did you mean '%s'?", keyword)
		}
	}

	// Must be at least 2 chars for numeric format
	if len(timeframe) < 2 {
		return fmt.Errorf("invalid timeframe format (must be at least 2 characters)")
	}

	// Find the last digit position
	lastDigitPos := -1
	for i := len(timeframe) - 1; i >= 0; i-- {
		if timeframe[i] >= '0' && timeframe[i] <= '9' {
			lastDigitPos = i
			break
		}
	}

	if lastDigitPos == -1 {
		return fmt.Errorf("invalid timeframe: no numeric value found")
	}

	// Split into value and unit
	valueStr := timeframe[:lastDigitPos+1]
	unit := timeframe[lastDigitPos+1:]

	// Validate number
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return fmt.Errorf("invalid timeframe number: %s", valueStr)
	}

	if value < 0 {
		return fmt.Errorf("timeframe cannot be negative")
	}

	// Validate unit
	switch unit {
	case "h", "H", "d", "D", "w", "W":
		return nil
	default:
		return fmt.Errorf("invalid timeframe unit: %s (supported: h,d,w)", unit)
	}
}
