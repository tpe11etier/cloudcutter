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

	// Gather all discovered fields from the hits
	discoveredFields := make(map[string]struct{})
	for _, entry := range entries {
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			discoveredFields[field] = struct{}{}
		}
	}

	// Check if we need metadata for these discovered fields
	needMetadata := false
	for field := range discoveredFields {
		if _, ok := v.state.data.fieldCache.Get(field); !ok {
			needMetadata = true
			break
		}
	}

	// Fetch metadata if needed
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

		// Provide some default metadata, in case these fields aren't in the response
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

		// Populate our cache from the FieldCaps response
		for f, types := range fieldCaps.Fields {
			for typeName, meta := range types {
				v.state.data.fieldCache.Set(f, &FieldMetadata{
					Type:         typeName,
					Searchable:   meta.Searchable,
					Aggregatable: meta.Aggregatable,
					Active:       false, // active in ES
				})
				break
			}
		}
	}

	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	allFields := make([]string, 0, len(discoveredFields))
	for field := range discoveredFields {
		allFields = append(allFields, field)
	}
	sort.Strings(allFields)

	v.state.data.originalFields = allFields
	v.state.data.fieldOrder = make([]string, len(allFields))
	copy(v.state.data.fieldOrder, allFields)

	v.state.data.activeFields = make(map[string]bool)
	if len(allFields) > 0 {
		firstField := allFields[0]
		v.state.data.activeFields[firstField] = true
	}
	
	v.manager.Logger().Debug("Fields loaded",
		"originalFields", v.state.data.originalFields,
		"fieldOrder", v.state.data.fieldOrder,
	)

	// Mark these fields as "found in ES" (meta.Active = true if you want)
	for field := range discoveredFields {
		if meta, ok := v.state.data.fieldCache.Get(field); ok {
			meta.Active = true
			v.manager.Logger().Debug("Set field meta.Active=true", "field", field)
		}
	}

	// preserve old selections and ensure `_id` is selected
	oldActive := v.state.data.activeFields
	newActive := make(map[string]bool)

	// preserve any old active fields, but only if they're still discovered
	for field := range oldActive {
		if _, ok := discoveredFields[field]; ok {
			newActive[field] = true
		}
	}

	v.state.data.activeFields = newActive

	v.manager.Logger().Debug("TUI active fields updated",
		"activeFields", v.state.data.activeFields,
	)

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
	v.components.selectedList.Clear()

	// Rebuild both lists.
	for _, field := range v.state.data.fieldOrder {
		if v.state.data.activeFields[field] {
			// Field is active => goes to the selected list
			v.components.selectedList.AddItem(field, "", 0, func() {
				v.toggleField(field)
			})
		} else {
			// Field is inactive => goes to the unselected list
			v.components.fieldList.AddItem(field, "", 0, func() {
				v.toggleField(field)
			})
		}
	}

	v.manager.Logger().Debug("Field list rebuilt successfully")
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
	var matchedFields []string // Changed name to avoid conflict

	// Add debug logging
	v.manager.Logger().Debug("Filtering fields",
		"filter", filter,
		"originalFields", v.state.data.originalFields,
		"activeFields", v.state.data.activeFields)

	// Only show fields that match the filter AND are not currently active
	for _, field := range v.state.data.originalFields {
		isActive := v.state.data.activeFields[field]
		containsFilter := strings.Contains(strings.ToLower(field), filter)

		v.manager.Logger().Debug("Field filter check",
			"field", field,
			"isActive", isActive,
			"containsFilter", containsFilter)

		if containsFilter && !isActive {
			matchedFields = append(matchedFields, field)
		}
	}

	v.state.data.fieldMatches = matchedFields

	for _, field := range matchedFields {
		displayText := field
		fieldName := field
		v.components.fieldList.AddItem(displayText, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing available fields matching '%s' (%d matches)", filter, len(matchedFields)))
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
