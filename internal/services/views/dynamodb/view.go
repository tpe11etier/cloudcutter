package dynamodb

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpelletiersophos/cloudcutter/internal/services/manager"
	"github.com/tpelletiersophos/cloudcutter/ui/components"
)

// View represents the DynamoDB view in your application.
// It implements the ServiceView interface defined in the manager package.
type View struct {
	name       string
	manager    *manager.Manager
	service    *dynamodb.Service
	pages      *tview.Pages
	leftPanel  *components.LeftPanel
	dataTable  *components.DataTable
	tableCache map[string]*dynamodbtypes.TableDescription
	ctx        context.Context
	mu         sync.Mutex
	wg         sync.WaitGroup
}

// NewView creates a new instance of the DynamoDB view.
// It initializes the UI components and sets up the layout and event handlers.
func NewView(manager *manager.Manager, dynamoService *dynamodb.Service) *View {
	view := &View{
		name:       "dynamodb",
		manager:    manager,
		service:    dynamoService,
		pages:      tview.NewPages(),
		leftPanel:  components.NewLeftPanel(),
		dataTable:  components.NewDataTable(),
		tableCache: make(map[string]*dynamodbtypes.TableDescription),
		ctx:        manager.ViewContext(),
	}

	view.configureComponents()
	view.setupLayout()
	view.initializeTableCache()
	return view
}

// Name returns the name of the view.
// This is required by the ServiceView interface.
func (v *View) Name() string {
	return v.name
}

// GetContent returns the main primitive (UI element) of the view.
// This is used by the manager to display the view.
// It can be considered for removal if it's not being used,
// but in this case, it seems to be used by the manager when switching views.
func (v *View) GetContent() tview.Primitive {
	return v.pages
}

// Show is called when the view is activated.
// It fetches the list of DynamoDB tables and sets the focus to the left panel.
func (v *View) Show() {
	v.fetchTables()
	v.manager.App.SetFocus(v.leftPanel)
}

// Hide is called when the view is deactivated.
// Currently, it does nothing but is required by the ServiceView interface.
// You might consider whether you need to perform any cleanup here.
func (v *View) Hide() {}

// InputHandler returns a function that handles key events for the view.
// In this case, pressing 'Esc' will set the focus back to the left panel.
func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			v.manager.SetFocus(v.leftPanel)
			return nil
		}
		return event
	}
}

// configureComponents sets up the visual appearance and styles of the UI components.
func (v *View) configureComponents() {
	// Configure the left panel (list of tables)
	v.leftPanel.SetBorder(true).
		SetTitle(" DynamoDB ").
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetTitleAlign(tview.AlignCenter)

	// Configure the data table (table details and items)
	v.dataTable.SetBorder(true).
		SetBorderColor(tcell.ColorGray)

	// Set the selection styles for the left panel and data table
	v.leftPanel.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorDarkCyan).
		Attributes(tcell.AttrBold))

	v.dataTable.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorDarkCyan).
		Attributes(tcell.AttrBold))

	// Set up event handlers for focus and selection changes
	v.setupComponentHandlers()
}

// setupComponentHandlers sets up the event handlers for the UI components.
func (v *View) setupComponentHandlers() {
	// Change border color when the left panel gains or loses focus
	v.leftPanel.SetFocusFunc(func() {
		v.leftPanel.SetBorderColor(tcell.ColorMediumTurquoise)
	})
	v.leftPanel.SetBlurFunc(func() {
		v.leftPanel.SetBorderColor(tcell.ColorGray)
	})

	// Change border color when the data table gains or loses focus
	v.dataTable.SetFocusFunc(func() {
		v.dataTable.SetBorderColor(tcell.ColorMediumTurquoise)
	})
	v.dataTable.SetBlurFunc(func() {
		v.dataTable.SetBorderColor(tcell.ColorGray)
	})

	// Fetch table details when the selection in the left panel changes
	v.leftPanel.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		v.fetchTableDetails(mainText)
	})

	// Show table items when a table is selected in the left panel
	v.leftPanel.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		v.showTableItems(mainText)
	})
}

// setupLayout creates the layout of the view using the manager's CreateLayout function.
func (v *View) setupLayout() {
	// Create the layout configuration
	layout := v.manager.CreateLayout(manager.LayoutConfig{
		Title:       "DynamoDB",
		Components:  []tview.Primitive{v.leftPanel, v.dataTable},
		Direction:   tview.FlexColumn,
		FixedSizes:  []int{30, 0}, // Left panel has a fixed width of 30
		Proportions: []int{0, 1},  // Data table takes the remaining space
	})

	// Add the layout to the pages (this allows for multiple views)
	v.pages.AddPage("dynamodb", layout, true, true)
}

// fetchTables retrieves the list of DynamoDB tables and populates the left panel.
func (v *View) fetchTables() {
	// Clear the left panel before adding new items
	v.leftPanel.Clear()

	// Fetch all table names
	tableNames, err := v.service.ListAllTables(v.ctx)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching DynamoDB tables: %v", err))
		return
	}

	for _, tableName := range tableNames {
		v.leftPanel.AddItem(tableName, "", 0, nil)
	}

	// Update the status bar with instructions
	v.manager.UpdateStatusBar("Select a table to view details or press Enter to view items")
}

// fetchTableDetails retrieves the details of a specific table and updates the summary.
func (v *View) fetchTableDetails(tableName string) {
	// Check if the table details are already cached
	if table, found := v.tableCache[tableName]; found {
		v.updateTableSummary(table)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	table, err := v.service.DescribeTable(ctx, tableName)
	v.updateTableSummary(table)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching table details: %v", err))
		return
	}

}

// updateTableSummary updates the header with summary information about the table.
func (v *View) updateTableSummary(table *dynamodbtypes.TableDescription) {
	if table == nil {
		v.manager.UpdateHeader(nil)
		return
	}

	// Create a summary of the table details
	summary := []components.SummaryItem{
		{Key: "Table Name", Value: aws.ToString(table.TableName)},
		{Key: "Status", Value: string(table.TableStatus)},
		{Key: "Item Count", Value: fmt.Sprintf("%d", aws.ToInt64(table.ItemCount))},
		{Key: "Size", Value: fmt.Sprintf("%d bytes", aws.ToInt64(table.TableSizeBytes))},
	}

	// Update the header component with the summary
	v.manager.UpdateHeader(summary)
}

// showTableItems retrieves and displays the items of a selected table.
func (v *View) showTableItems(tableName string) {
	items, err := v.service.ScanTable(v.ctx, tableName)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error scanning table %s: %v", tableName, err))
		return
	}

	v.updateDataTableForItems(items)
	v.manager.SetFocus(v.dataTable)
}

// updateDataTableForItems populates the data table with items from the DynamoDB table.
func (v *View) updateDataTableForItems(items []map[string]dynamodbtypes.AttributeValue) {
	v.dataTable.Clear()
	v.dataTable.SetSelectable(true, false) // Make rows selectable

	if len(items) == 0 {
		v.dataTable.SetCell(0, 0, tview.NewTableCell("No items found").SetSelectable(false))
		return
	}

	// Extract all attribute names (columns) from the items
	headersSet := make(map[string]struct{})
	for _, item := range items {
		for key := range item {
			headersSet[key] = struct{}{}
		}
	}

	// Convert the set of headers to a sorted slice
	var headers []string
	for key := range headersSet {
		headers = append(headers, key)
	}
	sort.Strings(headers) // Sort headers for consistent order

	// Set up the table headers in the data table
	for col, header := range headers {
		v.dataTable.SetCell(0, col, tview.NewTableCell(header).
			SetSelectable(false).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter))
	}

	// Add each item to the data table
	for row, item := range items {
		for col, header := range headers {
			value := attributeValueToString(item[header])
			v.dataTable.SetCell(row+1, col, tview.NewTableCell(value))
		}
	}

	selectedItemText, _ := v.leftPanel.GetItemText(v.leftPanel.GetCurrentItem())
	v.manager.UpdateStatusBar(fmt.Sprintf("Showing items for table %s", selectedItemText))
}

// attributeValueToString converts a DynamoDB AttributeValue to a string for display.
func attributeValueToString(av dynamodbtypes.AttributeValue) string {
	if av == nil {
		return "<nil>"
	}
	switch v := av.(type) {
	case *dynamodbtypes.AttributeValueMemberS:
		return v.Value
	case *dynamodbtypes.AttributeValueMemberN:
		return v.Value
	case *dynamodbtypes.AttributeValueMemberBOOL:
		return fmt.Sprintf("%t", v.Value)
	case *dynamodbtypes.AttributeValueMemberSS:
		return fmt.Sprintf("%v", v.Value)
	case *dynamodbtypes.AttributeValueMemberNS:
		return fmt.Sprintf("%v", v.Value)
	case *dynamodbtypes.AttributeValueMemberB:
		return fmt.Sprintf("%v", v.Value)
	case *dynamodbtypes.AttributeValueMemberBS:
		return fmt.Sprintf("%v", v.Value)
	case *dynamodbtypes.AttributeValueMemberM:
		return fmt.Sprintf("%v", v.Value)
	case *dynamodbtypes.AttributeValueMemberL:
		return fmt.Sprintf("%v", v.Value)
	case *dynamodbtypes.AttributeValueMemberNULL:
		return "NULL"
	default:
		return "[Unsupported Type]"
	}
}

// initializeTableCache preloads the table details into the cache.
func (v *View) initializeTableCache() {

	tableNames, err := v.service.ListAllTables(v.ctx)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching DynamoDB tables: %v", err))
		return
	}

	v.wg.Add(len(tableNames))

	for _, tableName := range tableNames {
		go func(name string) {
			defer v.wg.Done()

			table, err := v.service.DescribeTable(v.ctx, name)
			if err != nil {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching table details for %s: %v", name, err))
				return
			}

			v.mu.Lock()
			v.tableCache[name] = table
			v.mu.Unlock()
		}(tableName)
	}
	v.wg.Wait()
}
