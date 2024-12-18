package dynamodb

import (
	"context"
	"fmt"
	"github.com/tpelletiersophos/cloudcutter/internal/services"
	components2 "github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/types"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
)

var _ services.Reinitializer = (*View)(nil)

type View struct {
	name       string
	manager    *manager.Manager
	service    *dynamodb.Service
	leftPanel  *tview.List
	dataTable  *tview.Table
	tableCache map[string]*dynamodbtypes.TableDescription

	originalItems   []map[string]dynamodbtypes.AttributeValue
	filteredItems   []map[string]dynamodbtypes.AttributeValue
	leftPanelFilter string
	dataPanelFilter string

	layout tview.Primitive

	ctx context.Context
	mu  sync.Mutex
	wg  sync.WaitGroup
}

func NewView(manager *manager.Manager, dynamoService *dynamodb.Service) *View {
	view := &View{
		name:       "dynamodb",
		manager:    manager,
		service:    dynamoService,
		tableCache: make(map[string]*dynamodbtypes.TableDescription),
		ctx:        manager.ViewContext(),
	}

	view.setupLayout()
	view.initializeTableCache()
	return view
}

func (v *View) Name() string {
	return v.name
}

func (v *View) Content() tview.Primitive {
	return v.layout
}

func (v *View) Show() {
	if v.leftPanelFilter != "" {
		v.filterLeftPanel(v.leftPanelFilter)
	} else {
		v.fetchTables()
	}
	v.manager.App().SetFocus(v.leftPanel)
}

func (v *View) Hide() {}

func (v *View) fetchTables() {
	v.leftPanel.Clear()

	tableNames, err := v.service.ListAllTables(v.ctx)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching DynamoDB tables: %v", err))
		return
	}

	for _, tableName := range tableNames {
		v.leftPanel.AddItem(tableName, "", 0, nil)
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
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching table details: %v", err))
		v.updateTableSummary(nil)
		return
	}

	v.updateTableSummary(table)
}

func (v *View) updateTableSummary(table *dynamodbtypes.TableDescription) {
	if table == nil {
		v.manager.UpdateHeader(nil)
		return
	}

	summary := []types.SummaryItem{
		{Key: "Table Name", Value: aws.ToString(table.TableName)},
		{Key: "Status", Value: string(table.TableStatus)},
		{Key: "Item Count", Value: fmt.Sprintf("%d", aws.ToInt64(table.ItemCount))},
		{Key: "Size", Value: fmt.Sprintf("%d bytes", aws.ToInt64(table.TableSizeBytes))},
	}

	v.manager.UpdateHeader(summary)
}

func extractSortedHeaders(items []map[string]dynamodbtypes.AttributeValue) []string {
	headersSet := make(map[string]struct{})
	for _, item := range items {
		for key := range item {
			headersSet[key] = struct{}{}
		}
	}

	headers := make([]string, 0, len(headersSet))
	for key := range headersSet {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	return headers
}

func calculateColumnWidths(items []map[string]dynamodbtypes.AttributeValue, headers []string) map[string]int {
	widths := make(map[string]int)
	for _, header := range headers {
		widths[header] = len(header)
	}

	for _, item := range items {
		for header, value := range item {
			width := len(attributeValueToString(value))
			if width > widths[header] {
				widths[header] = width
			}
		}
	}

	for header := range widths {
		if widths[header] > 50 {
			widths[header] = 50
		}
	}

	return widths
}

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

func (v *View) ActiveField() string {
	currentFocus := v.manager.App().GetFocus()
	switch currentFocus {
	case v.leftPanel:
		return "leftPanel"
	case v.dataTable:
		return "dataTable"
	default:
		return ""
	}
}

func (v *View) setupLayout() {
	layoutCfg := types.LayoutConfig{
		Title:     "DynamoDB",
		Direction: tview.FlexColumn,
		Components: []types.Component{
			{
				ID:         "leftPanel",
				Type:       types.ComponentList,
				FixedSize:  30,
				Proportion: 0,
				Focus:      true,
				Style: types.Style{
					Border:      true,
					Title:       " DynamoDB ",
					TitleAlign:  tview.AlignCenter,
					TitleColor:  tcell.ColorMediumTurquoise,
					BorderColor: tcell.ColorMediumTurquoise,
					TextColor:   tcell.ColorBeige,
				},
				Properties: map[string]any{
					"items":                   []string{},
					"selectedBackgroundColor": tcell.ColorDarkCyan,
					"selectedTextColor":       tcell.ColorLightYellow,
					"onFocus": func(list *tview.List) {
						list.SetBorderColor(tcell.ColorMediumTurquoise)
					},
					"onBlur": func(list *tview.List) {
						list.SetBorderColor(tcell.ColorBeige)
					},
					"onChanged": func(index int, mainText string, secondaryText string, shortcut rune) {
						v.fetchTableDetails(mainText)
					},
					"onSelected": func(index int, mainText, secondaryText string, shortcut rune) {
						v.showTableItems(mainText)
					},
				},
			},
			{
				ID:         "dataTable",
				Type:       types.ComponentTable,
				Proportion: 1,
				Style: types.Style{
					Border:      true,
					BorderColor: tcell.ColorBeige,
				},
				Properties: map[string]any{
					"selectedBackgroundColor": tcell.ColorDarkCyan,
					"selectedTextColor":       tcell.ColorLightYellow,
					"onFocus": func(table *tview.Table) {
						table.SetBorderColor(tcell.ColorMediumTurquoise)
					},
					"onBlur": func(table *tview.Table) {
						table.SetBorderColor(tcell.ColorBeige)
					},
				},
			},
		},
	}

	v.layout = v.manager.CreateLayout(layoutCfg)
	//v.manager.pages.AddPage("dynamodb", layout, true, true)

	v.leftPanel = v.manager.GetPrimitiveByID("leftPanel").(*tview.List)
	v.dataTable = v.manager.GetPrimitiveByID("dataTable").(*tview.Table)
	v.dataTable.SetSelectable(true, false)
}

func (v *View) updateDataTableForItems(items []map[string]dynamodbtypes.AttributeValue) {
	v.dataTable.Clear()

	if len(items) == 0 {
		v.dataTable.SetCell(0, 0,
			tview.NewTableCell("No items found").
				SetTextColor(tcell.ColorBeige).
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
		return
	}

	// Get and sort headers
	headers := extractSortedHeaders(items)
	columnWidths := calculateColumnWidths(items, headers)

	// Set header row
	for col, header := range headers {
		v.dataTable.SetCell(0, col,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter).
				SetMaxWidth(columnWidths[header]).
				SetExpansion(1).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold))
	}

	// Set data rows
	for row, item := range items {
		for col, header := range headers {
			value := attributeValueToString(item[header])
			v.dataTable.SetCell(row+1, col,
				tview.NewTableCell(strings.TrimSpace(value)).
					SetTextColor(tcell.ColorBeige).
					SetAlign(tview.AlignLeft).
					SetMaxWidth(columnWidths[header]).
					SetExpansion(1).
					SetSelectable(true))
		}
	}

	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) filterLeftPanel(filter string) {
	if filter == "" {
		v.fetchTables()
		return
	}

	filter = strings.ToLower(filter)
	v.leftPanel.Clear()

	for tableName := range v.tableCache {
		if strings.Contains(strings.ToLower(tableName), filter) {
			v.leftPanel.AddItem(tableName, "", 0, nil)
		}
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing tables matching '%s'", filter))
}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			v.manager.HideAllModals()
			v.manager.SetFocus(v.leftPanel)
			return nil
		case tcell.KeyTab:
			currentFocus := v.manager.App().GetFocus()
			if currentFocus == v.leftPanel {
				v.manager.SetFocus(v.dataTable)
			} else {
				v.manager.SetFocus(v.leftPanel)
			}
			return nil
		}
		return event
	}
}

func (v *View) HandleFilter(prompt *components2.Prompt, previousFocus tview.Primitive) {
	var opts components2.PromptOptions

	switch previousFocus {
	case v.leftPanel:
		opts = components2.PromptOptions{
			Title:      " Filter Tables ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.leftPanelFilter = text
				v.filterLeftPanel(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.leftPanelFilter = ""
				v.filterLeftPanel("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterLeftPanel(text)
			},
		}
	case v.dataTable:
		opts = components2.PromptOptions{
			Title:      " Filter Items ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.dataPanelFilter = text
				v.filterItems(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.dataPanelFilter = ""
				v.filterItems("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterItems(text)
			},
		}
	}

	prompt.Configure(opts)
	v.manager.ShowFilterPrompt(prompt.Show())
	v.manager.App().SetFocus(prompt.InputField)
}

func (v *View) filterItems(filter string) {
	if v.originalItems == nil || len(v.originalItems) == 0 {
		return
	}

	if filter == "" {
		v.filteredItems = v.originalItems
		v.updateDataTableForItems(v.originalItems)
		v.manager.UpdateStatusBar("Showing all items")
		return
	}

	filter = strings.ToLower(filter)
	filtered := make([]map[string]dynamodbtypes.AttributeValue, 0)

	for _, item := range v.originalItems {
		matches := false
		for _, value := range item {
			if strings.Contains(strings.ToLower(attributeValueToString(value)), filter) {
				matches = true
				break
			}
		}
		if matches {
			filtered = append(filtered, item)
		}
	}

	v.filteredItems = filtered
	v.updateDataTableForItems(filtered)
	v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing %d of %d items", len(filtered), len(v.originalItems)))
}

func (v *View) showTableItems(tableName string) {
	items, err := v.service.ScanTable(v.ctx, tableName)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error scanning table %s: %v", tableName, err))
		return
	}

	v.originalItems = items
	v.filteredItems = items

	if v.dataPanelFilter != "" {
		v.filterItems(v.dataPanelFilter)
	} else {
		v.updateDataTableForItems(items)
	}
	v.manager.SetFocus(v.dataTable)
}

func (v *View) Reinitialize(cfg aws.Config) {
	v.service = dynamodb.NewService(cfg)
	v.tableCache = make(map[string]*dynamodbtypes.TableDescription)
	v.originalItems = nil
	v.filteredItems = nil
	v.leftPanel.Clear()
	v.dataTable.Clear()

	v.initializeTableCache()
	v.Show()
}
