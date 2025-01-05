package elastic

import (
	"strconv"
	"strings"
)

// BuildQuery combines multiple filters into one Elasticsearch bool-query.
func BuildQuery(filters []string, size int) map[string]interface{} {
	// If no filters, match all
	if len(filters) == 0 {
		return map[string]interface{}{
			"query": map[string]interface{}{
				"match_all": map[string]interface{}{},
			},
			"size": size,
		}
	}

	var mustClauses []map[string]interface{}
	for _, f := range filters {
		clause := ParseFilter(f)
		mustClauses = append(mustClauses, clause)
	}

	// Return a bool query with "must" conditions
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": mustClauses,
			},
		},
		"size": size,
	}
}

func ParseFilter(filter string) map[string]interface{} {
	filter = strings.TrimSpace(filter)

	// 1) Handle range queries (>, >=, <, <=)
	if clause := parseRangeSyntax(filter); clause != nil {
		return clause
	}

	// 2) Handle "field=value"
	if strings.Contains(filter, "=") {
		parts := strings.SplitN(filter, "=", 2)
		fieldName := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Try numeric
		if numericClause := parseNumericTerm(fieldName, value); numericClause != nil {
			return numericClause
		}
		// Try boolean
		if boolClause := parseBooleanTerm(fieldName, value); boolClause != nil {
			return boolClause
		}
		// Try wildcard
		if wildcardClause := parseWildcardTerm(fieldName, value); wildcardClause != nil {
			return wildcardClause
		}

		// Otherwise, fallback to match query
		return map[string]interface{}{
			"match": map[string]interface{}{
				fieldName: value,
			},
		}
	}

	// 3) If it doesn't match any known syntax, return match_all
	return map[string]interface{}{
		"match_all": map[string]interface{}{},
	}
}

func parseRangeSyntax(filter string) map[string]interface{} {
	// Look for first occurrence of < or >
	idx := strings.IndexAny(filter, "><")
	if idx == -1 {
		return nil
	}

	fieldName := strings.TrimSpace(filter[:idx])
	opAndValue := strings.TrimSpace(filter[idx:])

	var op, val string
	switch {
	case strings.HasPrefix(opAndValue, ">="):
		op = "gte"
		val = strings.TrimPrefix(opAndValue, ">=")
	case strings.HasPrefix(opAndValue, "<="):
		op = "lte"
		val = strings.TrimPrefix(opAndValue, "<=")
	case strings.HasPrefix(opAndValue, ">"):
		op = "gt"
		val = strings.TrimPrefix(opAndValue, ">")
	case strings.HasPrefix(opAndValue, "<"):
		op = "lt"
		val = strings.TrimPrefix(opAndValue, "<")
	default:
		return nil
	}

	val = strings.TrimSpace(val)
	if num, err := strconv.ParseFloat(val, 64); err == nil {
		return map[string]interface{}{
			"range": map[string]interface{}{
				fieldName: map[string]interface{}{
					op: num,
				},
			},
		}
	}
	return nil
}

// parseNumericTerm tries to parse value as a float and build a term query
func parseNumericTerm(fieldName, value string) map[string]interface{} {
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return map[string]interface{}{
			"term": map[string]interface{}{
				fieldName: num,
			},
		}
	}
	return nil
}

// parseBooleanTerm tries to parse value as "true" or "false" and build a term query
func parseBooleanTerm(fieldName, value string) map[string]interface{} {
	if strings.EqualFold(value, "true") {
		return map[string]interface{}{
			"term": map[string]interface{}{
				fieldName: true,
			},
		}
	} else if strings.EqualFold(value, "false") {
		return map[string]interface{}{
			"term": map[string]interface{}{
				fieldName: false,
			},
		}
	}
	return nil
}

// parseWildcardTerm checks if the value contains * or ? and returns a wildcard query
func parseWildcardTerm(fieldName, value string) map[string]interface{} {
	if strings.ContainsAny(value, "*?") {
		return map[string]interface{}{
			"wildcard": map[string]interface{}{
				fieldName: value,
			},
		}
	}
	return nil
}
