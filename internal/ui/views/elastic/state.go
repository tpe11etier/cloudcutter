package elastic

import (
	"context"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
	"sort"
	"sync"
)

type State struct {
	pagination PaginationState
	ui         UIState
	data       DataState
	search     SearchState
	misc       MiscState
	mu         sync.RWMutex
}

type PaginationState struct {
	currentPage int
	totalPages  int
	pageSize    int
}

type UIState struct {
	showRowNumbers   bool
	isLoading        bool
	fieldListFilter  string
	fieldListVisible bool
}

type DataState struct {
	activeFields     map[string]bool
	filters          []string
	currentResults   []*DocEntry
	fieldOrder       []string
	originalFields   []string
	fieldMatches     []string
	filteredResults  []*DocEntry
	displayedResults []*DocEntry
	columnCache      map[string][]string
	currentFilter    string
	fieldCache       *FieldCache
}

type SearchState struct {
	currentIndex    string
	matchingIndices []string
	numResults      int
	timeframe       string
	indexStats      *elastic.IndexStats
	cancelCurrentOp context.CancelFunc
}

type MiscState struct {
	visibleRows       int
	lastDisplayHeight int
	spinner           *spinner.Spinner
	rateLimit         *RateLimiter
}

func (d *DataState) ResetFields() {
	d.originalFields = nil
	d.fieldOrder = nil
	d.activeFields = make(map[string]bool)
	d.columnCache = make(map[string][]string)
}

func (d *DataState) IsFieldActive(field string) bool {
	return d.activeFields[field]
}

func (d *DataState) SetFieldActive(field string, active bool) {
	d.activeFields[field] = active
}

func (d *DataState) UpdateFieldsFromSet(newFields map[string]struct{}) {
	fields := make([]string, 0, len(newFields))
	for field := range newFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	d.originalFields = fields
	d.fieldOrder = make([]string, len(fields))
	copy(d.fieldOrder, fields)
}
