package elastic

import (
	"context"
	"sync"

	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
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
	fieldCache *FieldCache
	fieldState *FieldState

	filters       []string
	currentFilter string

	currentResults   []*DocEntry
	filteredResults  []*DocEntry
	displayedResults []*DocEntry
	columnCache      map[string][]string
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
	d.fieldCache = NewFieldCache()
	d.fieldState = NewFieldState(d.fieldCache)
	d.columnCache = make(map[string][]string)
}

func (d *DataState) IsFieldActive(field string) bool {
	return d.fieldState.IsFieldSelected(field)
}

func (d *DataState) SetFieldActive(field string, active bool) {
	if active {
		d.fieldState.SelectField(field)
	} else {
		d.fieldState.UnselectField(field)
	}
}
