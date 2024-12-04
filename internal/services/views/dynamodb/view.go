package dynamodb

import (
	"context"
	"fmt"
	"github.com/tpelletiersophos/ahtui/internal/services/aws/dynamodb"
	"github.com/tpelletiersophos/ahtui/internal/services/manager"
	"github.com/tpelletiersophos/ahtui/internal/types"
	"github.com/tpelletiersophos/ahtui/ui/components"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var _ types.ServiceView = (*View)(nil)

type View struct {
	name           string
	manager        *manager.Manager
	service        *dynamodb.Service
	contentFlex    *tview.Flex
	LeftPanel      *components.LeftPanel
	DataTable      *components.DataTable
	currentContext context.Context
	tableCache     map[string]*dynamodbtypes.TableDescription
}

func NewView(manager *manager.Manager, dynamoService *dynamodb.Service) *View {
	view := &View{
		name:        "dynamodb",
		manager:     manager,
		service:     dynamoService,
		contentFlex: tview.NewFlex(),
		tableCache:  make(map[string]*dynamodbtypes.TableDescription),
	}
	view.setupLayout()
	view.initializeTableCache()
	return view
}

func (v *View) Name() string {
	return v.name
}

func (v *View) GetContent() tview.Primitive {
	return v.contentFlex
}

func (v *View) Show() {
	v.fetchTables()
}

func (v *View) Hide() {}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			v.manager.SetFocus(v.LeftPanel)
			return nil
		}

		return event
	}
}

func (v *View) setupLayout() {
	// Initialize components
	v.LeftPanel = components.NewLeftPanel()
	v.DataTable = components.NewDataTable()

	// Style the components
	v.LeftPanel.SetBorder(true).SetBorderColor(tcell.ColorGray).
		SetTitle(" DynamoDB ").SetTitleColor(tcell.ColorMediumTurquoise).SetTitleAlign(tview.AlignCenter)

	v.DataTable.SetBorder(true).SetBorderColor(tcell.ColorGray)

	// Set up the content flex layout
	v.contentFlex = tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(v.LeftPanel, 40, 0, true).
		AddItem(v.DataTable, 0, 1, false)

	// Set selection styles
	v.LeftPanel.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorDarkCyan).
		Attributes(tcell.AttrBold))

	v.DataTable.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorDarkCyan).
		Attributes(tcell.AttrBold))

	// Set focus and blur functions
	v.LeftPanel.SetFocusFunc(func() {
		v.LeftPanel.SetBorderColor(tcell.ColorMediumTurquoise)
	})
	v.LeftPanel.SetBlurFunc(func() {
		v.LeftPanel.SetBorderColor(tcell.ColorGray)
	})

	v.DataTable.SetFocusFunc(func() {
		v.DataTable.SetBorderColor(tcell.ColorMediumTurquoise)
	})
	v.DataTable.SetBlurFunc(func() {
		v.DataTable.SetBorderColor(tcell.ColorGray)
	})

	// updates when row has focus
	v.LeftPanel.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		v.fetchTableDetails(mainText)
	})

	// Handle 'Enter' key press on the left panel
	v.LeftPanel.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		//v.fetchTableDetails(mainText)
		v.showTableItems(mainText)
	})
}

func (v *View) fetchTables() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	v.LeftPanel.Clear()

	tableNames, err := v.service.ListAllTables(ctx)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching DynamoDB tables: %v", err))
		return
	}

	for _, tableName := range tableNames {
		v.LeftPanel.AddItem(tableName, "", 0, nil)
	}

	v.manager.UpdateStatusBar("Select a table to view details or press Enter to view items")
}

func (v *View) fetchTableDetails(tableName string) {
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

	v.tableCache[tableName] = table
}

func (v *View) updateTableSummary(table *dynamodbtypes.TableDescription) {
	if table == nil {
		v.manager.UpdateHeader(nil)
		return
	}

	summary := []components.SummaryItem{
		{Key: "Table Name", Value: aws.ToString(table.TableName)},
		{Key: "Status", Value: string(table.TableStatus)},
		{Key: "Item Count", Value: fmt.Sprintf("%d", aws.ToInt64(table.ItemCount))},
		{Key: "Size", Value: fmt.Sprintf("%d bytes", aws.ToInt64(table.TableSizeBytes))},
	}
	v.manager.UpdateHeader(summary)
}

func (v *View) showTableItems(tableName string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	items, err := v.service.ScanTable(ctx, tableName)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error scanning table %s: %v", tableName, err))
		return
	}

	v.updateDataTableForItems(items)
	v.manager.SetFocus(v.DataTable)
}

func (v *View) updateDataTableForItems(items []map[string]dynamodbtypes.AttributeValue) {
	v.DataTable.Clear()
	v.DataTable.SetSelectable(true, false) // Make rows selectable

	// If there are no items, show a message
	if len(items) == 0 {
		v.DataTable.SetCell(0, 0, tview.NewTableCell("No items found").SetSelectable(false))
		return
	}

	// Get the attribute names (columns) from the items
	headersSet := make(map[string]struct{})
	for _, item := range items {
		for key := range item {
			headersSet[key] = struct{}{}
		}
	}

	var headers []string
	for key := range headersSet {
		headers = append(headers, key)
	}
	sort.Strings(headers) // Sort headers for consistent order

	// Set up the table headers
	for col, header := range headers {
		v.DataTable.SetCell(0, col, tview.NewTableCell(header).
			SetSelectable(false).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter))
	}

	// Add the items to the table
	for row, item := range items {
		for col, header := range headers {
			value := attributeValueToString(item[header])
			v.DataTable.SetCell(row+1, col, tview.NewTableCell(value))
		}
	}

	selectedItemText, _ := v.LeftPanel.GetItemText(v.LeftPanel.GetCurrentItem())
	v.manager.UpdateStatusBar(fmt.Sprintf("Showing items for table %s", selectedItemText))
}

// Updated helper function
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

func (v *View) initializeTableCache() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		tableNames, err := v.service.ListAllTables(ctx)
		if err != nil {
			v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching DynamoDB tables: %v", err))
			return
		}

		for _, tableName := range tableNames {
			table, err := v.service.DescribeTable(ctx, tableName)
			if err != nil {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching table details for %s: %v", tableName, err))
				continue
			}
			v.tableCache[tableName] = table
		}
	}()
}
