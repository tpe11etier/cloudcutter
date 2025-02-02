package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
	"sort"
	"strings"
	"sync"
)

type FieldCaps struct {
	Fields map[string]map[string]FieldMetadata `json:"fields"`
}

type FieldMetadata struct {
	Type         string `json:"type"`
	Searchable   bool   `json:"searchable"`
	Aggregatable bool   `json:"aggregatable"`
	Active       bool   `json:"active"`
}

type FieldCache struct {
	cache sync.Map
}

type FieldState struct {
	// Fields discovered from documents
	discoveredFields map[string]struct{}

	// Fields that are selected for display
	selectedFields map[string]struct{}

	// Ordered list of selected fields (maintains display order)
	fieldOrder []string

	// Current filter being applied
	currentFilter string

	// Fields matching current filter
	filteredFields []string

	// Reference to field cache for metadata access
	fieldCache *FieldCache

	mu sync.RWMutex
}

func NewFieldCache() *FieldCache {
	return &FieldCache{}
}

func (fc *FieldCache) Get(field string) (*FieldMetadata, bool) {
	if val, ok := fc.cache.Load(field); ok {
		return val.(*FieldMetadata), true
	}
	return nil, false
}

func (fc *FieldCache) Set(field string, metadata *FieldMetadata) {
	fc.cache.Store(field, metadata)
}

func (v *View) resetFieldState() error {
	v.state.mu.Lock()
	v.state.data.ResetFields()
	v.state.mu.Unlock()

	v.components.fieldList.Clear()
	v.components.selectedList.Clear()

	if err := v.loadFields(); err != nil {
		v.manager.Logger().Error("Failed to load fields for new index", "error", err)
		v.manager.App().QueueUpdateDraw(func() {
			v.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
		})
		return err
	}

	return nil
}

func (v *View) loadFields() error {
	// Initialize if needed
	if v.state.data.fieldCache == nil {
		v.state.data.fieldCache = NewFieldCache()
		v.state.data.fieldState = NewFieldState(v.state.data.fieldCache)
	}

	query := map[string]any{
		"size": 10,
		"query": map[string]any{
			"match_all": map[string]any{},
		},
	}

	result, err := v.executeSearch(query)
	if err != nil {
		return err
	}

	entries, err := v.processSearchResults(result.Hits.Hits)
	if err != nil {
		return err
	}

	// Update field state with discovered fields
	v.state.data.fieldState.UpdateFromDocuments(entries)

	needMetadata := false
	discoveredFields := v.state.data.fieldState.GetDiscoveredFields()
	for _, field := range discoveredFields {
		if _, ok := v.state.data.fieldCache.Get(field); !ok {
			needMetadata = true
			break
		}
	}

	if needMetadata {
		res, err := v.service.Client.FieldCaps(
			v.service.Client.FieldCaps.WithIndex(v.state.search.currentIndex),
			v.service.Client.FieldCaps.WithBody(strings.NewReader(`{"fields": "*"}`)),
		)
		if err != nil {
			return fmt.Errorf("field caps error: %v", err)
		}
		defer res.Body.Close()

		var fieldCaps FieldCaps
		if err := json.NewDecoder(res.Body).Decode(&fieldCaps); err != nil {
			return fmt.Errorf("error decoding field caps: %v", err)
		}

		defaultFields := map[string]FieldMetadata{
			"_id": {
				Type:         "keyword",
				Searchable:   true,
				Aggregatable: true,
				Active:       false,
			},
			"_index": {
				Type:         "keyword",
				Searchable:   true,
				Aggregatable: true,
				Active:       false,
			},
		}
		for f, meta := range defaultFields {
			v.state.data.fieldCache.Set(f, &meta)
		}

		for f, types := range fieldCaps.Fields {
			for typeName, meta := range types {
				v.state.data.fieldCache.Set(f, &FieldMetadata{
					Type:         typeName,
					Searchable:   meta.Searchable,
					Aggregatable: meta.Aggregatable,
					Active:       false,
				})
				break
			}
		}
	}

	if !v.state.data.fieldState.IsFieldSelected("_id") {
		v.state.data.fieldState.SelectField("_id")
	}

	v.manager.App().QueueUpdateDraw(func() {
		v.rebuildFieldList()
	})

	return nil
}

func (v *View) updateFieldsFromResults(results []*DocEntry) {
	if len(results) == 0 {
		return
	}

	// Update the field state with new document fields
	v.state.data.fieldState.UpdateFromDocuments(results)

	v.manager.App().QueueUpdateDraw(func() {
		v.rebuildFieldList()
	})
}

func (v *View) toggleFieldList() {
	v.state.ui.fieldListVisible = !v.state.ui.fieldListVisible
	v.updateResultsLayout()

	if !v.state.ui.fieldListVisible {
		v.manager.App().SetFocus(v.components.resultsTable)
	}
}

func (v *View) rebuildFieldList() {
	v.components.fieldList.Clear()
	v.components.selectedList.Clear()

	// Get all discovered and selected fields
	discoveredFields := v.state.data.fieldState.GetDiscoveredFields()
	selectedFields := v.state.data.fieldState.GetOrderedSelectedFields()

	// Build selected fields list maintaining order
	for _, field := range selectedFields {
		fieldName := field
		v.components.selectedList.AddItem(field, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	// Build available fields list (excluding selected fields)
	selectedMap := make(map[string]struct{})
	for _, field := range selectedFields {
		selectedMap[field] = struct{}{}
	}

	for _, field := range discoveredFields {
		if _, isSelected := selectedMap[field]; !isSelected {
			fieldName := field
			v.components.fieldList.AddItem(field, "", 0, func() {
				v.toggleField(fieldName)
			})
		}
	}
}

func (v *View) toggleField(field string) {
	if v.state.data.fieldState.IsFieldSelected(field) {
		v.state.data.fieldState.UnselectField(field)
	} else {
		v.state.data.fieldState.SelectField(field)
	}

	v.rebuildFieldList()

	v.refreshResults()
}

func (v *View) filterFieldList(filter string) {
	// Get filtered fields from FieldState
	filteredFields := v.state.data.fieldState.ApplyFilter(filter)

	v.components.fieldList.Clear()

	for _, field := range filteredFields {
		fieldName := field
		v.components.fieldList.AddItem(field, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	if filter != "" {
		v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing available fields matching '%s' (%d matches)",
			filter, len(filteredFields)))
	} else {
		v.manager.UpdateStatusBar("Showing all available fields")
	}
}

func (v *View) moveFieldPosition(field string, moveUp bool) {
	if moved := v.state.data.fieldState.MoveField(field, moveUp); moved {
		v.rebuildFieldList()

		selectedFields := v.state.data.fieldState.GetOrderedSelectedFields()
		newPos := 0
		for i, f := range selectedFields {
			if f == field {
				newPos = i
				break
			}
		}

		v.components.selectedList.SetCurrentItem(newPos)

		v.displayCurrentPage()
	}
}

func (v *View) initFieldsSync() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		indices, err := v.service.ListIndices(context.Background(), "*")
		if err != nil {
			v.manager.Logger().Error("Failed to list indices", "error", err)
			errChan <- err
			return
		}
		v.state.mu.Lock()
		v.state.search.matchingIndices = indices
		v.state.mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := v.loadFields(); err != nil {
			errChan <- err
			return
		}

		v.manager.App().QueueUpdateDraw(func() {
			v.rebuildFieldList()
		})
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func getIndices(v *View) []help.Command {
	v.state.mu.RLock()
	indices := v.state.search.matchingIndices
	v.state.mu.RUnlock()

	indexCommands := make([]help.Command, 0, len(indices))
	for _, idx := range indices {
		indexCommands = append(indexCommands, help.Command{
			Key: idx,
		})
	}
	indexCommands = append(indexCommands, help.Command{})
	return indexCommands
}

func (v *View) getActiveHeaders() []string {
	return v.state.data.fieldState.GetOrderedSelectedFields()
}

func NewFieldState(fieldCache *FieldCache) *FieldState {
	return &FieldState{
		discoveredFields: make(map[string]struct{}),
		selectedFields:   make(map[string]struct{}),
		fieldOrder:       make([]string, 0),
		fieldCache:       fieldCache,
	}
}

func (fs *FieldState) Reset() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.discoveredFields = make(map[string]struct{})
	fs.selectedFields = make(map[string]struct{})
	fs.fieldOrder = make([]string, 0)
	fs.currentFilter = ""
	fs.filteredFields = nil
}

// UpdateFromDocuments updates discovered fields from document results
func (fs *FieldState) UpdateFromDocuments(docs []*DocEntry) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	newFields := make(map[string]struct{})
	for _, doc := range docs {
		fields := doc.GetAvailableFields()
		for _, field := range fields {
			newFields[field] = struct{}{}
		}
	}

	// Check if fields have changed
	if len(newFields) == len(fs.discoveredFields) {
		allMatch := true
		for field := range fs.discoveredFields {
			if _, ok := newFields[field]; !ok {
				allMatch = false
				break
			}
		}
		if allMatch {
			return
		}
	}

	fs.discoveredFields = newFields

	// Clean up selected fields that no longer exist
	for field := range fs.selectedFields {
		if _, ok := newFields[field]; !ok {
			delete(fs.selectedFields, field)
			fs.fieldOrder = removeString(fs.fieldOrder, field)
		}
	}
}

// GetDiscoveredFields returns a sorted list of all discovered fields
func (fs *FieldState) GetDiscoveredFields() []string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	fields := make([]string, 0, len(fs.discoveredFields))
	for field := range fs.discoveredFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

// IsFieldSelected checks if a field is currently selected
func (fs *FieldState) IsFieldSelected(field string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	_, ok := fs.selectedFields[field]
	return ok
}

// SelectField adds a field to selected fields and maintains order
func (fs *FieldState) SelectField(field string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.discoveredFields[field]; !exists {
		return
	}

	if _, alreadySelected := fs.selectedFields[field]; !alreadySelected {
		fs.selectedFields[field] = struct{}{}
		fs.fieldOrder = append(fs.fieldOrder, field)
	}
}

// UnselectField removes a field from selected fields
func (fs *FieldState) UnselectField(field string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	delete(fs.selectedFields, field)
	fs.fieldOrder = removeString(fs.fieldOrder, field)
}

// MoveField changes the position of a field in the order
func (fs *FieldState) MoveField(field string, moveUp bool) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	currentPos := -1
	for i, f := range fs.fieldOrder {
		if f == field {
			currentPos = i
			break
		}
	}

	if currentPos == -1 {
		return false
	}

	newPos := currentPos
	if moveUp && currentPos > 0 {
		newPos = currentPos - 1
	} else if !moveUp && currentPos < len(fs.fieldOrder)-1 {
		newPos = currentPos + 1
	}

	if newPos == currentPos {
		return false
	}

	// Perform the swap
	fs.fieldOrder[currentPos], fs.fieldOrder[newPos] = fs.fieldOrder[newPos], fs.fieldOrder[currentPos]
	return true
}

// GetOrderedSelectedFields returns selected fields in display order
func (fs *FieldState) GetOrderedSelectedFields() []string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	result := make([]string, len(fs.fieldOrder))
	copy(result, fs.fieldOrder)
	return result
}

// ApplyFilter updates filtered fields based on search string
func (fs *FieldState) ApplyFilter(filter string) []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.currentFilter = filter

	if filter == "" {
		fs.filteredFields = nil
		discovered := make([]string, 0, len(fs.discoveredFields))
		for field := range fs.discoveredFields {
			if _, isSelected := fs.selectedFields[field]; !isSelected {
				discovered = append(discovered, field)
			}
		}
		sort.Strings(discovered)
		return discovered
	}

	filter = strings.ToLower(filter)
	var matched []string

	for field := range fs.discoveredFields {
		if _, isSelected := fs.selectedFields[field]; !isSelected {
			if strings.Contains(strings.ToLower(field), filter) {
				matched = append(matched, field)
			}
		}
	}

	sort.Strings(matched)
	fs.filteredFields = matched
	return matched
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
