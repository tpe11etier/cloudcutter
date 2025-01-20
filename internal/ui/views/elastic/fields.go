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

type FieldMetadata struct {
	Type         string
	Searchable   bool
	Aggregatable bool
	Active       bool
}

type FieldCache struct {
	cache sync.Map
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

func (v *View) resetFieldState() {
	v.state.mu.Lock()
	v.state.data.ResetFields()
	v.state.mu.Unlock()

	v.components.fieldList.Clear()
	v.components.selectedList.Clear()

	go func() {
		if err := v.loadFields(); err != nil {
			v.manager.Logger().Error("Failed to load fields for new index", "error", err)
			v.manager.App().QueueUpdateDraw(func() {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
			})
		}
	}()
}

func (v *View) loadFields() error {
	query := map[string]interface{}{
		"size": 10,
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
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

	activeFields := make(map[string]struct{})
	for _, entry := range entries {
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			activeFields[field] = struct{}{}
		}
	}

	// Only fetch field metadata if it isn't cached
	needMetadata := false
	for field := range activeFields {
		if _, ok := v.state.data.fieldCache.Get(field); !ok {
			needMetadata = true
			break
		}
	}

	if needMetadata {
		// Fetch metadata for all fields
		res, err := v.service.Client.FieldCaps(
			v.service.Client.FieldCaps.WithIndex(v.state.search.currentIndex),
			v.service.Client.FieldCaps.WithBody(strings.NewReader(`{"fields": "*"}`)),
		)
		if err != nil {
			return fmt.Errorf("field caps error: %v", err)
		}
		defer res.Body.Close()

		var fieldCaps struct {
			Fields map[string]map[string]struct {
				Type         string `json:"type"`
				Searchable   bool   `json:"searchable"`
				Aggregatable bool   `json:"aggregatable"`
			} `json:"fields"`
		}

		if err := json.NewDecoder(res.Body).Decode(&fieldCaps); err != nil {
			return fmt.Errorf("error decoding field caps: %v", err)
		}

		for field, types := range fieldCaps.Fields {
			for typeName, meta := range types {
				v.state.data.fieldCache.Set(field, &FieldMetadata{
					Type:         typeName,
					Searchable:   meta.Searchable,
					Aggregatable: meta.Aggregatable,
					Active:       false,
				})
				break
			}
		}
	}

	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	fields := make([]string, 0, len(activeFields))
	for field := range activeFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	v.state.data.originalFields = fields
	v.state.data.fieldOrder = make([]string, len(fields))
	copy(v.state.data.fieldOrder, fields)

	for field := range activeFields {
		if meta, ok := v.state.data.fieldCache.Get(field); ok {
			meta.Active = true
		}
	}

	// Preserve existing active field selections that are still valid
	newActiveFields := make(map[string]bool)
	for field := range v.state.data.activeFields {
		if _, ok := activeFields[field]; ok {
			newActiveFields[field] = true
		}
	}
	v.state.data.activeFields = newActiveFields

	return nil
}

func (v *View) updateFieldsFromResults(results []*DocEntry) {
	if len(results) == 0 {
		return
	}

	newFields := make(map[string]struct{})
	for _, entry := range results {
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			newFields[field] = struct{}{}
		}
	}

	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	if len(newFields) == len(v.state.data.originalFields) {
		allMatch := true
		for _, field := range v.state.data.originalFields {
			if _, ok := newFields[field]; !ok {
				allMatch = false
				break
			}
		}
		if allMatch {
			return // No changes needed
		}
	}

	fields := make([]string, 0, len(newFields))
	for field := range newFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	// Update state atomically
	v.state.data.originalFields = fields
	v.state.data.fieldOrder = make([]string, len(fields))
	copy(v.state.data.fieldOrder, fields)

	// Update field metadata
	for field := range newFields {
		if meta, ok := v.state.data.fieldCache.Get(field); ok {
			meta.Active = true
		}
	}

	newActiveFields := make(map[string]bool)
	for field := range v.state.data.activeFields {
		if _, ok := newFields[field]; ok {
			newActiveFields[field] = true
		}
	}
	v.state.data.activeFields = newActiveFields

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
	for _, field := range v.state.data.fieldOrder {
		if !v.state.data.activeFields[field] { // Only show non-selected fields
			v.components.fieldList.AddItem(field, "", 0, func() {
				v.toggleField(field)
			})
		}
	}
}

func (v *View) toggleField(field string) {
	isActive := v.state.data.activeFields[field]
	v.state.data.activeFields[field] = !isActive

	if !isActive {
		v.components.selectedList.AddItem(field, "", 0, func() {
			v.toggleField(field)
		})

		for i := 0; i < v.components.fieldList.GetItemCount(); i++ {
			if text, _ := v.components.fieldList.GetItemText(i); text == field {
				v.components.fieldList.RemoveItem(i)
				break
			}
		}
	} else {
		for i := 0; i < v.components.selectedList.GetItemCount(); i++ {
			if text, _ := v.components.selectedList.GetItemText(i); text == field {
				v.components.selectedList.RemoveItem(i)
				break
			}
		}
		v.components.fieldList.AddItem(field, "", 0, func() {
			v.toggleField(field)
		})
	}

	// Refresh results to show new column order
	v.refreshResults()
}

func (v *View) filterFieldList(filter string) {
	v.state.data.currentFilter = filter
	v.components.fieldList.Clear()

	if filter == "" {
		v.state.data.fieldMatches = nil
		v.rebuildFieldList()
		v.manager.UpdateStatusBar("Showing all fields")
		return
	}

	filter = strings.ToLower(filter)
	var matches []string

	// Only show fields that match the filter AND are not currently active
	for _, field := range v.state.data.originalFields {
		if strings.Contains(strings.ToLower(field), filter) && !v.state.data.activeFields[field] {
			matches = append(matches, field)
		}
	}

	v.state.data.fieldMatches = matches

	for _, field := range matches {
		displayText := field
		fieldName := field
		v.components.fieldList.AddItem(displayText, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing available fields matching '%s' (%d matches)", filter, len(matches)))
}

func (v *View) moveFieldPosition(field string, moveUp bool) {
	// Get current list of fields in selected list
	var selectedFields []string
	for i := 0; i < v.components.selectedList.GetItemCount(); i++ {
		text, _ := v.components.selectedList.GetItemText(i)
		selectedFields = append(selectedFields, text)
	}

	// Find current position
	currentPos := -1
	for i, f := range selectedFields {
		if f == field {
			currentPos = i
			break
		}
	}
	if currentPos == -1 {
		return
	}

	// Calculate new position
	newPos := currentPos
	if moveUp && currentPos > 0 {
		newPos = currentPos - 1
	} else if !moveUp && currentPos < len(selectedFields)-1 {
		newPos = currentPos + 1
	}

	// If no movement needed, return early
	if newPos == currentPos {
		return
	}

	// Do the swap
	selectedFields[currentPos], selectedFields[newPos] = selectedFields[newPos], selectedFields[currentPos]

	// Rebuild just the selected list
	v.components.selectedList.Clear()
	for _, f := range selectedFields {
		field := f // Capture for closure
		v.components.selectedList.AddItem(field, "", 0, func() {
			v.toggleField(field)
		})
	}

	// Set focus back to the moved item
	v.components.selectedList.SetCurrentItem(newPos)

	// Refresh the results table since order changed
	v.displayCurrentPage()
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
	// get order from selected list
	var headers []string
	for i := 0; i < v.components.selectedList.GetItemCount(); i++ {
		text, _ := v.components.selectedList.GetItemText(i)
		headers = append(headers, text)
	}
	return headers
}
