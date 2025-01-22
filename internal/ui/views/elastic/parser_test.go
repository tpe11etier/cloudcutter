package elastic

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildQuery(t *testing.T) {
	fixedTime := time.Date(2024, 10, 4, 1, 0, 17, 0, time.UTC)
	fieldCache := newTestFieldCache()
	tests := []struct {
		name        string
		filters     []string
		size        int
		timeframe   string
		want        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:      "No filters returns match_all query with timeframe",
			filters:   []string{},
			size:      10,
			timeframe: "12h",
			want: map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
							{
								"bool": map[string]any{
									"should": []map[string]any{
										{
											"range": map[string]any{
												"unixTime": map[string]any{
													"gte": fixedTime.Unix() - 12*60*60,
													"lte": fixedTime.Unix(),
												},
											},
										},
										{
											"range": map[string]any{
												"detectionGeneratedTime": map[string]any{
													"gte": fixedTime.UnixMilli() - 12*60*60*1000,
													"lte": fixedTime.UnixMilli(),
												},
											},
										},
									},
									"minimum_should_match": 1,
								},
							},
						},
					},
				},
				"size": 10,
			},
		},
		{
			name:      "No filters returns match_all query without timeframe",
			filters:   []string{},
			size:      10,
			timeframe: "",
			want: map[string]any{
				"query": map[string]any{
					"match_all": map[string]any{},
				},
				"size": 10,
			},
		},
		{
			name:        "Negative size returns error",
			filters:     []string{},
			size:        -1,
			timeframe:   "12h",
			wantErr:     true,
			errContains: "size must be non-negative",
		},
		{
			name:      "Single valid filter with timeframe",
			filters:   []string{"status=active"},
			size:      20,
			timeframe: "12h",
			want: map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
							{
								"bool": map[string]any{
									"should": []map[string]any{
										{
											"range": map[string]any{
												"unixTime": map[string]any{
													"gte": fixedTime.Unix() - 12*60*60,
													"lte": fixedTime.Unix(),
												},
											},
										},
										{
											"range": map[string]any{
												"detectionGeneratedTime": map[string]any{
													"gte": fixedTime.UnixMilli() - 12*60*60*1000,
													"lte": fixedTime.UnixMilli(),
												},
											},
										},
									},
									"minimum_should_match": 1,
								},
							},
							{
								"match": map[string]any{
									"status": "active",
								},
							},
						},
					},
				},
				"size": 20,
			},
		},
		{
			name:        "Invalid filter in multiple filters",
			filters:     []string{"status=active", "age>", "role=admin"},
			wantErr:     true,
			size:        10,
			timeframe:   "12h",
			errContains: "missing value in range query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildQueryWithTime(tt.filters, tt.size, tt.timeframe, fixedTime, fieldCache)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("BuildQuery() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
				t.Errorf("BuildQuery() = \n%s\nwant\n%s", gotJSON, wantJSON)
			}
		})
	}
}

func TestParseFilter(t *testing.T) {
	fieldCache := newTestFieldCache()
	tests := []struct {
		name        string
		filter      string
		want        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:        "Empty filter",
			filter:      "",
			wantErr:     true,
			errContains: "empty filter",
		},
		{
			name:        "Invalid field name with special characters",
			filter:      "@field=value",
			wantErr:     true,
			errContains: "invalid field name",
		},
		{
			name:   "Valid numeric term",
			filter: "age=25",
			want: map[string]any{
				"term": map[string]any{
					"age": float64(25),
				},
			},
		},
		{
			name:   "Valid boolean term - true",
			filter: "active=true",
			want: map[string]any{
				"term": map[string]any{
					"active": true,
				},
			},
		},
		{
			name:   "Valid boolean term - false",
			filter: "deleted=FALSE",
			want: map[string]any{
				"term": map[string]any{
					"deleted": false,
				},
			},
		},
		{
			name:   "Valid null value query",
			filter: "status=null",
			want: map[string]any{
				"bool": map[string]any{
					"must_not": map[string]any{
						"exists": map[string]any{
							"field": "status",
						},
					},
				},
			},
		},
		{
			name:        "Invalid wildcard query with leading asterisk",
			filter:      "name=*john",
			wantErr:     true,
			errContains: "wildcard query cannot start with *",
		},
		{
			name:   "Valid wildcard query",
			filter: "name=john*",
			want: map[string]any{
				"wildcard": map[string]any{
					"name": "john*",
				},
			},
		},
		{
			name:   "Valid range query - greater than",
			filter: "price>100",
			want: map[string]any{
				"range": map[string]any{
					"price": map[string]any{
						"gt": float64(100),
					},
				},
			},
		},
		{
			name:   "Valid range query - less than or equal",
			filter: "stock<=50",
			want: map[string]any{
				"range": map[string]any{
					"stock": map[string]any{
						"lte": float64(50),
					},
				},
			},
		},
		{
			name:        "Invalid range query - missing value",
			filter:      "price>",
			wantErr:     true,
			errContains: "missing value in range query",
		},
		{
			name:        "Invalid range query - non-numeric value",
			filter:      "price>abc",
			wantErr:     true,
			errContains: "invalid numeric value in range query",
		},
		{
			name:   "Match query with escaped wildcard",
			filter: "description=test\\*product",
			want: map[string]any{
				"match": map[string]any{
					"description": "test*product",
				},
			},
		},
		{
			name:   "Wildcard query with unescaped wildcard",
			filter: "description=test*product",
			want: map[string]any{
				"wildcard": map[string]any{
					"description": "test*product",
				},
			},
		},
		{
			name:   "Match query with multiple escaped characters",
			filter: "description=test\\*product\\?",
			want: map[string]any{
				"match": map[string]any{
					"description": "test*product?",
				},
			},
		},
		{
			name:   "Wildcard query with mixed escaped and unescaped",
			filter: "description=test\\**product",
			want: map[string]any{
				"wildcard": map[string]any{
					"description": "test**product",
				},
			},
		},
		{
			name:   "Valid field name with dots",
			filter: "user.name=john",
			want: map[string]any{
				"match": map[string]any{
					"user.name": "john",
				},
			},
		},
		{
			name:        "Invalid filter format",
			filter:      "invalid_filter",
			wantErr:     true,
			errContains: "invalid filter format",
		},
		{
			name:   "ID query",
			filter: "_id=2091402405ee09645b3a985deb2d0c8b6c37d324",
			want: map[string]any{
				"ids": map[string]any{
					"values": []string{"2091402405ee09645b3a985deb2d0c8b6c37d324"},
				},
			},
		},
		{
			name:   "Detection ID query",
			filter: "detection_id_dedup=2091402405ee09645b3a985deb2d0c8b6c37d324",
			want: map[string]any{
				"term": map[string]any{
					"detection_id_dedup": "2091402405ee09645b3a985deb2d0c8b6c37d324",
				},
			},
		},
		{
			name:   "Wildcard with escaped characters",
			filter: "name=john\\*doe\\?",
			want: map[string]any{
				"match": map[string]any{
					"name": "john*doe?",
				},
			},
		},
		{
			name:        "Complex wildcard",
			filter:      "email=*@*.com",
			wantErr:     true,
			errContains: "wildcard query cannot start with *",
		},
		{
			name:   "Compound field name",
			filter: "user.profile.email=test@example.com",
			want: map[string]any{
				"match": map[string]any{
					"user.profile.email": "test@example.com",
				},
			},
		},
		{
			name:   "Range query with decimal",
			filter: "price>=99.99",
			want: map[string]any{
				"range": map[string]any{
					"price": map[string]any{
						"gte": float64(99.99),
					},
				},
			},
		},
		{
			name:   "Range query with negative number",
			filter: "temperature<-10",
			want: map[string]any{
				"range": map[string]any{
					"temperature": map[string]any{
						"lt": float64(-10),
					},
				},
			},
		},
		{
			name:   "Multiple dots in field name",
			filter: "data.user.preferences.theme=dark",
			want: map[string]any{
				"match": map[string]any{
					"data.user.preferences.theme": "dark",
				},
			},
		},
		{
			name:   "Field name with underscore and numbers",
			filter: "custom_field_123=value",
			want: map[string]any{
				"match": map[string]any{
					"custom_field_123": "value",
				},
			},
		},
		{
			name:   "Value with spaces",
			filter: "description=This is a test",
			want: map[string]any{
				"match": map[string]any{
					"description": "This is a test",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilter(tt.filter, fieldCache)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseFilter() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
				t.Errorf("ParseFilter() = \n%s\nwant\n%s", gotJSON, wantJSON)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	validFieldNames := []string{
		"name",
		"user.name",
		"user_name",
		"userName123",
		"name-with-hyphens",
		"field.with.dots",
		"a.b.c",
		"user.profile.firstName",
		"very.deep.nested.field.name",
		"mix.of_styles-in.one.field",
	}

	invalidFieldNames := []string{
		"",
		"123field",
		"@field",
		"field@invalid",
		"field name",
		".fieldname",
		"fieldname.",
		"field..name",
		"field.123name",
		"field.-name",
		"field._name",
		"field.name.",
		".field.name",
		"user..name",
		"field.5.name",
		"field.*",
		"field.@name",
	}

	for _, name := range validFieldNames {
		t.Run("Valid field name: "+name, func(t *testing.T) {
			if !isValidFieldName(name) {
				t.Errorf("isValidFieldName(%q) = false, want true", name)
			}
		})
	}

	for _, name := range invalidFieldNames {
		t.Run("Invalid field name: "+name, func(t *testing.T) {
			if isValidFieldName(name) {
				t.Errorf("isValidFieldName(%q) = true, want false", name)
			}
		})
	}

	// Test unescapeValue
	escapeTests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"escaped\\*star", "escaped*star"},
		{"escaped\\\\backslash", "escaped\\backslash"},
		{"escaped\\=equals", "escaped=equals"},
		{"multiple\\*\\?\\=", "multiple*?="},
		{"trailing\\", "trailing\\"},
	}

	for _, tt := range escapeTests {
		t.Run("Unescape: "+tt.input, func(t *testing.T) {
			got := unescapeValue(tt.input)
			if got != tt.want {
				t.Errorf("unescapeValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimeframe(t *testing.T) {

	tests := []struct {
		name      string
		timeframe string
		want      time.Duration
		wantErr   bool
	}{
		// Standard numeric durations
		{
			name:      "12 hours",
			timeframe: "12h",
			want:      12 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "24 hours",
			timeframe: "24h",
			want:      24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "7 days",
			timeframe: "7d",
			want:      7 * 24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "1 week",
			timeframe: "1w",
			want:      7 * 24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "30 days",
			timeframe: "30d",
			want:      30 * 24 * time.Hour,
			wantErr:   false,
		},

		// Special keywords
		//{
		//	name:      "today keyword",
		//	timeframe: "today",
		//	want:      15*time.Hour + 30*time.Minute, // Since start of day (15:30 - 00:00)
		//	wantErr:   false,
		//},
		{
			name:      "week keyword",
			timeframe: "week",
			want:      7 * 24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "month keyword",
			timeframe: "month",
			want:      30 * 24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "quarter keyword",
			timeframe: "quarter",
			want:      90 * 24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "year keyword",
			timeframe: "year",
			want:      365 * 24 * time.Hour,
			wantErr:   false,
		},

		// Case variations
		{
			name:      "uppercase hour",
			timeframe: "24H",
			want:      24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "uppercase week keyword",
			timeframe: "WEEK",
			want:      7 * 24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "mixed case month",
			timeframe: "MoNtH",
			want:      30 * 24 * time.Hour,
			wantErr:   false,
		},

		// Error cases
		{
			name:      "invalid unit",
			timeframe: "24x",
			wantErr:   true,
		},
		{
			name:      "empty timeframe",
			timeframe: "",
			wantErr:   true,
		},
		{
			name:      "invalid format",
			timeframe: "abc",
			wantErr:   true,
		},
		{
			name:      "negative value",
			timeframe: "-24h",
			wantErr:   true,
		},
		{
			name:      "invalid keyword",
			timeframe: "fortnight",
			wantErr:   true,
		},
		{
			name:      "zero value",
			timeframe: "0h",
			wantErr:   false,
			want:      0,
		},

		// Whitespace handling
		{
			name:      "leading space",
			timeframe: " 24h",
			want:      24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "trailing space",
			timeframe: "24h ",
			want:      24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "surrounding spaces",
			timeframe: " 24h ",
			want:      24 * time.Hour,
			wantErr:   false,
		},
		{
			name:      "spaces with keyword",
			timeframe: " week ",
			want:      7 * 24 * time.Hour,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeframe(tt.timeframe)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimeframe(%q) error = %v, wantErr %v", tt.timeframe, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseTimeframe(%q) = %v, want %v", tt.timeframe, got, tt.want)
			}
		})
	}
}

func TestBuildTimeQuery(t *testing.T) {
	// Fixed time for consistent testing
	fixedTime := time.Date(2024, 10, 4, 1, 0, 17, 0, time.UTC)

	tests := []struct {
		name      string
		timeframe string
		now       time.Time
		want      map[string]any
		wantErr   bool
	}{
		{
			name:      "12 hour timeframe",
			timeframe: "12h",
			now:       fixedTime,
			want: map[string]any{
				"bool": map[string]any{
					"should": []map[string]any{
						{
							"range": map[string]any{
								"unixTime": map[string]any{
									"gte": int64(1728003617 - (12 * 3600)),
									"lte": int64(1728003617),
								},
							},
						},
						{
							"range": map[string]any{
								"detectionGeneratedTime": map[string]any{
									"gte": int64(1728003617000 - (12 * 3600 * 1000)),
									"lte": int64(1728003617000),
								},
							},
						},
					},
					"minimum_should_match": 1,
				},
			},
			wantErr: false,
		},
		{
			name:      "week keyword",
			timeframe: "week",
			now:       fixedTime,
			want: map[string]any{
				"bool": map[string]any{
					"should": []map[string]any{
						{
							"range": map[string]any{
								"unixTime": map[string]any{
									"gte": int64(1728003617 - (7 * 24 * 3600)),
									"lte": int64(1728003617),
								},
							},
						},
						{
							"range": map[string]any{
								"detectionGeneratedTime": map[string]any{
									"gte": int64(1728003617000 - (7 * 24 * 3600 * 1000)),
									"lte": int64(1728003617000),
								},
							},
						},
					},
					"minimum_should_match": 1,
				},
			},
			wantErr: false,
		},
		{
			name:      "empty timeframe",
			timeframe: "",
			now:       fixedTime,
			want:      nil,
			wantErr:   false,
		},
		{
			name:      "invalid timeframe",
			timeframe: "invalid",
			now:       fixedTime,
			wantErr:   true,
		},
		//{
		//	name:      "today keyword",
		//	timeframe: "today",
		//	now:       fixedTime,
		//	want: map[string]any{
		//		"bool": map[string]any{
		//			"should": []map[string]any{
		//				{
		//					"range": map[string]any{
		//						"unixTime": map[string]any{
		//							"gte": int64(1728003617 - int64(fixedTime.Sub(time.Date(fixedTime.Year(), fixedTime.Month(), fixedTime.Day(), 0, 0, 0, 0, fixedTime.Location())).Seconds())),
		//							"lte": int64(1728003617),
		//						},
		//					},
		//				},
		//				{
		//					"range": map[string]any{
		//						"detectionGeneratedTime": map[string]any{
		//							"gte": int64(1728003617000 - int64(fixedTime.Sub(time.Date(fixedTime.Year(), fixedTime.Month(), fixedTime.Day(), 0, 0, 0, 0, fixedTime.Location())).Milliseconds())),
		//							"lte": int64(1728003617000),
		//						},
		//					},
		//				},
		//			},
		//			"minimum_should_match": 1,
		//		},
		//	},
		//	wantErr: false,
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildTimeQuery(tt.timeframe, tt.now)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildTimeQuery(%q) error = %v, wantErr %v", tt.timeframe, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareMaps(got, tt.want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
				t.Errorf("BuildTimeQuery(%q) =\n%s\nwant\n%s", tt.timeframe, gotJSON, wantJSON)
			}
		})
	}
}

// Helper function to compare nested maps
func compareMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v1 := range a {
		v2, ok := b[k]
		if !ok {
			return false
		}

		switch val1 := v1.(type) {
		case map[string]any:
			val2, ok := v2.(map[string]any)
			if !ok || !compareMaps(val1, val2) {
				return false
			}
		case []any:
			val2, ok := v2.([]any)
			if !ok || !compareSlices(val1, val2) {
				return false
			}
		case time.Time:
			val2, ok := v2.(time.Time)
			if !ok || val1.Sub(val2).Seconds() > 1 {
				return false
			}
		default:
			if !reflect.DeepEqual(v1, v2) {
				return false
			}
		}
	}
	return true
}

func compareSlices(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		switch val1 := a[i].(type) {
		case map[string]any:
			val2, ok := b[i].(map[string]any)
			if !ok || !compareMaps(val1, val2) {
				return false
			}
		case []any:
			val2, ok := b[i].([]any)
			if !ok || !compareSlices(val2, val2) {
				return false
			}
		default:
			if !reflect.DeepEqual(a[i], b[i]) {
				return false
			}
		}
	}
	return true
}

func newTestFieldCache() *FieldCache {
	fc := NewFieldCache()
	fc.Set("status", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	fc.Set("age", &FieldMetadata{Type: "long", Searchable: true, Aggregatable: true})
	fc.Set("price", &FieldMetadata{Type: "float", Searchable: true, Aggregatable: true})
	fc.Set("temperature", &FieldMetadata{Type: "float", Searchable: true, Aggregatable: true})
	fc.Set("stock", &FieldMetadata{Type: "long", Searchable: true, Aggregatable: true})
	fc.Set("deleted", &FieldMetadata{Type: "boolean", Searchable: true, Aggregatable: true})
	fc.Set("active", &FieldMetadata{Type: "boolean", Searchable: true, Aggregatable: true})
	fc.Set("name", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	fc.Set("description", &FieldMetadata{Type: "text", Searchable: true, Aggregatable: false})
	fc.Set("user.name", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	fc.Set("user.profile.email", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	fc.Set("data.user.preferences.theme", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	fc.Set("custom_field_123", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	fc.Set("_id", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: false})
	fc.Set("detection_id_dedup", &FieldMetadata{Type: "keyword", Searchable: true, Aggregatable: true})
	return fc
}
