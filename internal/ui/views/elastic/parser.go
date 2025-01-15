package elastic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// Compile regexes once for better performance
	validFieldNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(?:\.[a-zA-Z][a-zA-Z0-9_-]*)*$`)

	validOperatorRegex = regexp.MustCompile(`^(>=|<=|>|<|=)$`)
)

// Error types for better error handling
type ParseError struct {
	Field   string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error on field '%s': %s", e.Field, e.Message)
}

// BuildQuery combines multiple filters into one Elasticsearch bool-query with error handling
// BuildQuery combines multiple filters into one Elasticsearch bool-query with error handling
func BuildQuery(filters []string, size int, timeframe string) (map[string]any, error) {
	return BuildQueryWithTime(filters, size, timeframe, time.Now())
}

// BuildQueryWithTime is like BuildQuery but accepts a specific time for testing
func BuildQueryWithTime(filters []string, size int, timeframe string, now time.Time) (map[string]any, error) {
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
		clause, err := ParseFilter(f)
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

// ParseFilter parses a single filter with comprehensive error handling
func ParseFilter(filter string) (map[string]any, error) {
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

	// 1) Handle range queries (>, >=, <, <=)
	if clause, err := parseRangeSyntax(filter); err != nil {
		return nil, err
	} else if clause != nil {
		return clause, nil
	}

	// 2) Handle "field=value"
	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return nil, &ParseError{Field: filter, Message: "invalid filter format, expected 'field=value' or range query"}
	}

	fieldName := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Validate field name
	if !isValidFieldName(fieldName) {
		return nil, &ParseError{Field: fieldName, Message: "invalid field name"}
	}

	// Handle null values
	if isNullValue(value) {
		return buildNullQuery(fieldName), nil
	}

	// Try numeric
	if clause, err := parseNumericTerm(fieldName, value); err != nil {
		return nil, err
	} else if clause != nil {
		return clause, nil
	}

	// Try boolean
	if clause, err := parseBooleanTerm(fieldName, value); err != nil {
		return nil, err
	} else if clause != nil {
		return clause, nil
	}

	// Try wildcard
	if clause, err := parseWildcardTerm(fieldName, value); err != nil {
		return nil, err
	} else if clause != nil {
		return clause, nil
	}

	// Otherwise, use match query
	matchQuery := make(map[string]any)
	innerMatch := make(map[string]any)
	innerMatch[fieldName] = unescapeValue(value)
	matchQuery["match"] = innerMatch
	return matchQuery, nil
}

func parseRangeSyntax(filter string) (map[string]any, error) {
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

	// Parse the value as number
	num, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, &ParseError{Field: fieldName, Message: fmt.Sprintf("invalid numeric value in range query: %s", value)}
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

	rangeQuery := make(map[string]any)
	fieldQuery := make(map[string]any)
	opQuery := make(map[string]any)
	opQuery[rangeOp] = num
	fieldQuery[fieldName] = opQuery
	rangeQuery["range"] = fieldQuery

	return rangeQuery, nil
}

func parseNumericTerm(fieldName, value string) (map[string]any, error) {
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		termQuery := make(map[string]any)
		innerTerm := make(map[string]any)
		innerTerm[fieldName] = num
		termQuery["term"] = innerTerm
		return termQuery, nil
	}
	return nil, nil
}

func parseBooleanTerm(fieldName, value string) (map[string]any, error) {
	valueLower := strings.ToLower(value)
	switch valueLower {
	case "true", "false":
		termQuery := make(map[string]any)
		innerTerm := make(map[string]any)
		innerTerm[fieldName] = valueLower == "true"
		termQuery["term"] = innerTerm
		return termQuery, nil
	}
	return nil, nil
}

func parseWildcardTerm(fieldName, value string) (map[string]any, error) {
	// Check if the string contains unescaped wildcards
	hasUnescapedWildcard := false
	escaped := false

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
			if i == 0 {
				return nil, &ParseError{Field: fieldName, Message: "wildcard query cannot start with *"}
			}
			hasUnescapedWildcard = true
		}
	}

	if hasUnescapedWildcard {
		wildcardQuery := make(map[string]any)
		innerWildcard := make(map[string]any)
		innerWildcard[fieldName] = unescapeValue(value)
		wildcardQuery["wildcard"] = innerWildcard
		return wildcardQuery, nil
	}
	return nil, nil
}

// Helper functions

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

// ParseTimeframe converts a timeframe string (e.g., "12h", "7d") to time.Duration
func ParseTimeframe(timeframe string) (time.Duration, error) {
	timeframe = strings.TrimSpace(strings.ToLower(timeframe))
	if timeframe == "" {
		return 0, fmt.Errorf("empty timeframe")
	}

	// Handle special keywords
	switch timeframe {
	case "today":
		now := time.Now()
		// Set startOfDay to midnight, so the duration is how long it’s been since 00:00
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

	// Parse numeric value with unit
	length := len(timeframe)
	if length < 2 {
		return 0, fmt.Errorf("invalid timeframe format: %s", timeframe)
	}

	value, err := strconv.Atoi(timeframe[:length-1])
	if err != nil {
		return 0, fmt.Errorf("invalid timeframe number: %s", timeframe)
	}

	if value < 0 {
		return 0, fmt.Errorf("timeframe cannot be negative: %s", timeframe)
	}

	// Get the unit
	unit := timeframe[length-1:]
	switch unit {
	case "h", "H":
		return time.Duration(value) * time.Hour, nil
	case "d", "D":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w", "W":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid timeframe unit: %s (supported: h,d,w)", unit)
	}
}

// BuildTimeQuery creates an Elasticsearch time range query based on the timeframe
func BuildTimeQuery(timeframe string, now time.Time) (map[string]interface{}, error) {
	if timeframe == "" {
		return nil, nil
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
