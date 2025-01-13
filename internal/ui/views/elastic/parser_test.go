package elastic

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBuildQuery(t *testing.T) {
	tests := []struct {
		name        string
		filters     []string
		size        int
		want        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:    "No filters returns match_all query",
			filters: []string{},
			size:    10,
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
			wantErr:     true,
			errContains: "size must be non-negative",
		},
		{
			name:    "Single valid filter",
			filters: []string{"status=active"},
			size:    20,
			want: map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
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
			errContains: "missing value in range query",
		},
		{
			name: "Complex query combining multiple types",
			filters: []string{
				"_id=2091402405ee09645b3a985deb2d0c8b6c37d324",
				"status=active",
				"age>=21",
				"name=john*",
				"is_deleted=false",
				"last_login=null",
			},
			size: 20,
			want: map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
							{
								"ids": map[string]any{
									"values": []string{"2091402405ee09645b3a985deb2d0c8b6c37d324"},
								},
							},
							{
								"match": map[string]any{
									"status": "active",
								},
							},
							{
								"range": map[string]any{
									"age": map[string]any{
										"gte": float64(21),
									},
								},
							},
							{
								"wildcard": map[string]any{
									"name": "john*",
								},
							},
							{
								"term": map[string]any{
									"is_deleted": false,
								},
							},
							{
								"bool": map[string]any{
									"must_not": map[string]any{
										"exists": map[string]any{
											"field": "last_login",
										},
									},
								},
							},
						},
					},
				},
				"size": 20,
			},
		},
		{
			name: "Query with escaped characters",
			filters: []string{
				"description=test\\*product",
				"category=tools\\?misc",
			},
			size: 10,
			want: map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
							{
								"match": map[string]any{
									"description": "test*product",
								},
							},
							{
								"match": map[string]any{
									"category": "tools?misc",
								},
							},
						},
					},
				},
				"size": 10,
			},
		},
		{
			name: "Query with all numeric comparisons",
			filters: []string{
				"price>100",
				"price<=200",
				"stock>=10",
				"rating<5",
			},
			size: 15,
			want: map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
							{
								"range": map[string]any{
									"price": map[string]any{
										"gt": float64(100),
									},
								},
							},
							{
								"range": map[string]any{
									"price": map[string]any{
										"lte": float64(200),
									},
								},
							},
							{
								"range": map[string]any{
									"stock": map[string]any{
										"gte": float64(10),
									},
								},
							},
							{
								"range": map[string]any{
									"rating": map[string]any{
										"lt": float64(5),
									},
								},
							},
						},
					},
				},
				"size": 15,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildQuery(tt.filters, tt.size)
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
			got, err := ParseFilter(tt.filter)
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
	// Test isValidFieldName
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
