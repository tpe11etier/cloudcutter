package dynamodb

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/common"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/views"
)

var _ views.Reinitializer = (*View)(nil)

type View struct {
	name         string
	content      *tview.Flex
	manager      *manager.Manager
	service      dynamodb.Interface
	leftPanel    *tview.List
	dataTable    *tview.Table
	filterPrompt *components.Prompt
	layout       tview.Primitive

	// Shared event bus
	sharedEventBus  *common.EventBus
	componentMapper *DynamoDBComponentMapper

	state viewState
	ctx   context.Context
	mu    sync.Mutex
	wg    sync.WaitGroup
}

type viewState struct {
	isLoading         bool
	tableCache        map[string]*dynamodbtypes.TableDescription
	originalItems     []map[string]dynamodbtypes.AttributeValue
	filteredItems     []map[string]dynamodbtypes.AttributeValue
	leftPanelFilter   string
	dataPanelFilter   string
	showRowNumbers    bool
	visibleRows       int
	lastDisplayHeight int
	currentPage       int
	pageSize          int
	totalPages        int
	spinner           *spinner.Spinner
}

func NewView(manager *manager.Manager, dynamoService dynamodb.Interface) *View {
	view := &View{
		name:         "dynamodb",
		manager:      manager,
		service:      dynamoService,
		filterPrompt: components.NewPrompt(),
		state: viewState{
			tableCache:        make(map[string]*dynamodbtypes.TableDescription),
			showRowNumbers:    true,
			visibleRows:       0,
			lastDisplayHeight: 0,
			currentPage:       1,
			pageSize:          50,
			totalPages:        1,
		},
		ctx: manager.ViewContext(),
	}

	view.setupLayout()

	// Initialize component mapper (after layout setup so components exist)
	view.componentMapper = NewDynamoDBComponentMapper(view)

	// Initialize shared event bus
	logger := common.NewLoggerAdapter(
		view.manager.Logger().Debug,
		view.manager.Logger().Info,
		view.manager.Logger().Error,
	)
	errorHandler := common.NewSimpleErrorHandler(view.manager, logger)
	globalHandler := common.NewDefaultGlobalShortcutHandler()

	view.sharedEventBus = common.NewEventBus(&common.EventBusConfig{
		View:          view,
		ErrorHandler:  errorHandler,
		Logger:        logger,
		GlobalHandler: globalHandler,
	})

	// Register dynamodb-specific handlers
	view.sharedEventBus.RegisterHandler(DataTableComponent, NewDynamoDBDataTableHandler(view))
	view.sharedEventBus.RegisterHandler(LeftPanelComponent, NewDynamoDBLeftPanelHandler(view))
	view.sharedEventBus.RegisterHandler(FilterPromptComponent, NewDynamoDBFilterPromptHandler(view))

	view.initializeTableCache()
	return view
}

func (v *View) Name() string {
	return v.name
}

func (v *View) Content() tview.Primitive {
	return v.content
}

func (v *View) Show() {
	v.manager.App().SetFocus(v.leftPanel)
	if v.leftPanel.GetItemCount() == 0 {
		v.fetchTables()
	}
}

func (v *View) Hide() {}

// ViewInterface implementation methods

// GetManager returns the manager for this view
func (v *View) GetManager() common.ManagerInterface {
	return v.manager
}

// GetComponentMapper returns the component mapper for this view
func (v *View) GetComponentMapper() common.ComponentMapper {
	return v.componentMapper
}

// GetName returns the view name for logging (alias to Name for consistency)
func (v *View) GetName() string {
	return v.Name()
}

func (v *View) fetchTables() {
	tableNames, err := v.service.ListTables(v.ctx)
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
	if table, found := v.state.tableCache[tableName]; found {
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

	v.state.tableCache[tableName] = table
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
	tableNames, err := v.service.ListTables(v.ctx)
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
			v.state.tableCache[name] = table
			v.mu.Unlock()
		}(tableName)
	}
	v.wg.Wait()
}

func (v *View) setupLayout() {
	layoutCfg := types.LayoutConfig{
		Title:     "DynamoDB",
		Direction: tview.FlexColumn,
		Components: []types.Component{
			{
				ID:        "leftPanel",
				Type:      types.ComponentList,
				FixedSize: 30,
				Focus:     true,
				Style: types.ListStyle{
					BaseStyle: types.BaseStyle{
						Border:      true,
						Title:       " DynamoDB ",
						TitleAlign:  tview.AlignCenter,
						TitleColor:  tcell.ColorMediumTurquoise,
						BorderColor: tcell.ColorMediumTurquoise,
						TextColor:   tcell.ColorBeige,
					},
					SelectedTextColor:       tcell.ColorLightYellow,
					SelectedBackgroundColor: tcell.ColorDarkCyan,
				},
				Properties: types.ListProperties{
					Items: []string{},
					OnFocus: func(list *tview.List) {
						list.SetBorderColor(tcell.ColorMediumTurquoise)
					},
					OnBlur: func(list *tview.List) {
						list.SetBorderColor(tcell.ColorBeige)
					},
					OnChanged: func(index int, mainText string, secondaryText string, shortcut rune) {
						v.fetchTableDetails(mainText)
					},
					OnSelected: func(index int, mainText, secondaryText string, shortcut rune) {
						v.showTableItems(mainText)
					},
				},
			},
			{
				ID:         "dataTable",
				Type:       types.ComponentTable,
				Proportion: 1,
				Style: types.TableStyle{
					BaseStyle: types.BaseStyle{
						Border:      true,
						BorderColor: tcell.ColorBeige,
					},
					SelectedTextColor:       tcell.ColorLightYellow,
					SelectedBackgroundColor: tcell.ColorDarkCyan,
				},
				Properties: types.TableProperties{
					OnFocus: func(table *tview.Table) {
						table.SetBorderColor(tcell.ColorMediumTurquoise)
					},
					OnBlur: func(table *tview.Table) {
						table.SetBorderColor(tcell.ColorBeige)
					},
				},
			},
		},
	}

	v.content = v.manager.CreateLayout(layoutCfg).(*tview.Flex)
	pages := v.manager.Pages()
	pages.AddPage("dynamodb", v.content, true, true)

	v.leftPanel = v.manager.GetPrimitiveByID("leftPanel").(*tview.List)
	v.dataTable = v.manager.GetPrimitiveByID("dataTable").(*tview.Table)
	v.dataTable.SetSelectable(true, false)
}

func (v *View) updateDataTableForItems(items []map[string]dynamodbtypes.AttributeValue) {
	v.dataTable.Clear()
	v.calculateVisibleRows()

	if len(items) == 0 {
		v.dataTable.SetCell(0, 0,
			tview.NewTableCell("No items found").
				SetTextColor(tcell.ColorBeige).
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
		return
	}

	totalItems := len(items)
	v.state.pageSize = v.state.visibleRows
	v.state.totalPages = (totalItems + v.state.pageSize - 1) / v.state.pageSize
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	start := (v.state.currentPage - 1) * v.state.pageSize
	end := start + v.state.pageSize
	if end > totalItems {
		end = totalItems
	}

	if start >= totalItems {
		start = totalItems - v.state.pageSize
		if start < 0 {
			start = 0
		}
		end = totalItems
	}

	pageItems := items[start:end]

	headers := extractSortedHeaders(items)
	columnWidths := calculateColumnWidths(items, headers)

	displayHeaders := headers
	if v.state.showRowNumbers {
		displayHeaders = append([]string{"#"}, headers...)
	}

	for col, header := range displayHeaders {
		cell := tview.NewTableCell(header).
			SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)

		if v.state.showRowNumbers && col == 0 {
			cell.SetMaxWidth(2).
				SetExpansion(0).
				SetText("#")
		} else {
			cell.SetMaxWidth(columnWidths[header]).
				SetExpansion(1)
		}
		v.dataTable.SetCell(0, col, cell)
	}

	for rowIdx, item := range pageItems {
		currentRow := rowIdx + 1
		currentCol := 0

		if v.state.showRowNumbers {
			v.dataTable.SetCell(currentRow, currentCol,
				tview.NewTableCell(fmt.Sprintf("%d", start+rowIdx+1)).
					SetTextColor(tcell.ColorGray).
					SetAlign(tview.AlignRight).
					SetMaxWidth(3).
					SetExpansion(0).
					SetSelectable(false))
			currentCol++
		}

		for _, header := range headers {
			value := attributeValueToString(item[header])
			v.dataTable.SetCell(currentRow, currentCol,
				tview.NewTableCell(strings.TrimSpace(value)).
					SetTextColor(tcell.ColorBeige).
					SetAlign(tview.AlignLeft).
					SetMaxWidth(columnWidths[header]).
					SetExpansion(1).
					SetSelectable(true))
			currentCol++
		}
	}

	v.dataTable.SetFixed(1, 0)
	if len(pageItems) > 0 {
		v.dataTable.Select(1, 0)
	}

	statusMsg := fmt.Sprintf("Page %d/%d | Showing %d of %d items",
		v.state.currentPage, v.state.totalPages,
		len(pageItems), totalItems)

	if v.state.showRowNumbers {
		statusMsg += fmt.Sprintf(" | [%s]Row numbers: on (press 'r' to toggle)[-]",
			style.GruvboxMaterial.Yellow)
	}

	v.manager.UpdateStatusBar(statusMsg)
}

func (v *View) filterLeftPanel(filter string) {
	tableNames := make([]string, 0, len(v.state.tableCache))

	filter = strings.ToLower(filter)

	for tableName := range v.state.tableCache {
		if filter == "" || strings.Contains(strings.ToLower(tableName), filter) {
			tableNames = append(tableNames, tableName)
		}
	}

	sort.Strings(tableNames)

	v.leftPanel.Clear()
	for _, tableName := range tableNames {
		v.leftPanel.AddItem(tableName, "", 0, nil)
	}

	if filter == "" {
		v.manager.UpdateStatusBar("Showing all tables")
	} else {
		v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing tables matching '%s'", filter))
	}
}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		currentFocus := v.manager.App().GetFocus()
		return v.sharedEventBus.ProcessEvent(event, currentFocus)
	}
}

func (v *View) HandleFilter(prompt *components.Prompt, previousFocus tview.Primitive) {
	var opts components.PromptOptions

	switch previousFocus {
	case v.leftPanel:
		opts = components.PromptOptions{
			Title:      " Filter Tables ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.state.leftPanelFilter = text
				v.filterLeftPanel(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.leftPanelFilter = ""
				v.filterLeftPanel("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterLeftPanel(text)
			},
		}
	case v.dataTable:
		opts = components.PromptOptions{
			Title:      " Filter Items ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.state.dataPanelFilter = text
				v.filterItems(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.dataPanelFilter = ""
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
	promptLayout := prompt.Layout()
	v.manager.Pages().AddPage(types.ModalFilter, promptLayout, true, true)
	v.manager.App().SetFocus(prompt.InputField)
}

func (v *View) showTableItems(tableName string) {
	v.showLoading(fmt.Sprintf("Fetching items for table %s...", tableName))
	go func() {
		items, err := v.service.ScanTable(v.ctx, tableName)

		v.manager.App().QueueUpdateDraw(func() {
			defer v.hideLoading()

			if err != nil {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error scanning table %s: %v", tableName, err))
				return
			}

			v.state.originalItems = items
			v.state.filteredItems = items

			if v.state.dataPanelFilter != "" {
				v.filterItems(v.state.dataPanelFilter)
			} else {
				v.updateDataTableForItems(items)
			}
			v.manager.SetFocus(v.dataTable)
		})
	}()
}
func (v *View) Reinitialize(cfg aws.Config) error {
	v.service = dynamodb.NewService(cfg)

	v.state.tableCache = make(map[string]*dynamodbtypes.TableDescription)
	v.state.originalItems = nil
	v.state.filteredItems = nil
	v.leftPanel.Clear()
	v.dataTable.Clear()

	v.manager.UpdateStatusBar("Reinitializing DynamoDB...")
	v.initializeTableCache()

	tableNames := make([]string, 0, len(v.state.tableCache))
	for tableName := range v.state.tableCache {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)
	for _, tName := range tableNames {
		v.leftPanel.AddItem(tName, "", 0, nil)
	}

	return nil
}

func (v *View) showLoading(message string) {
	if v.state.spinner == nil {
		v.state.spinner = spinner.NewSpinner(message)
		v.state.spinner.SetOnComplete(func() {
			pages := v.manager.Pages()
			pages.RemovePage("loading")
		})
	} else {
		v.state.spinner.SetMessage(message)
	}

	if !v.state.spinner.IsLoading() {
		modal := spinner.CreateSpinnerModal(v.state.spinner)
		pages := v.manager.Pages()
		pages.AddPage("loading", modal, true, true)
		v.state.spinner.Start(v.manager.App())
	}
}

func (v *View) hideLoading() {
	if v.state.spinner != nil {
		v.state.spinner.Stop()
	}
}

func (v *View) showFilterPrompt(source tview.Primitive) {
	previousFocus := source

	switch source {
	case v.leftPanel:
		v.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Tables ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.state.leftPanelFilter = text
				v.filterLeftPanel(text)
				v.manager.Pages().RemovePage(types.ModalFilter)
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.leftPanelFilter = ""
				v.filterLeftPanel("")
				v.manager.Pages().RemovePage(types.ModalFilter)
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterLeftPanel(text)
			},
		})

	case v.dataTable:
		v.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Items ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.state.dataPanelFilter = text
				v.filterItems(text)
				v.manager.Pages().RemovePage(types.ModalFilter)
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.dataPanelFilter = ""
				v.filterItems("")
				v.manager.Pages().RemovePage(types.ModalFilter)
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterItems(text)
			},
		})
	}

	v.filterPrompt.SetText("")
	promptLayout := v.filterPrompt.Layout()
	v.manager.Pages().AddPage(types.ModalFilter, promptLayout, true, true)
	v.manager.App().SetFocus(v.filterPrompt.InputField)
}

func (v *View) calculateVisibleRows() {
	// Get the current screen size
	_, _, _, height := v.dataTable.GetInnerRect()

	if height == v.state.lastDisplayHeight {
		return // No need to recalculate if height hasn't changed
	}

	v.state.lastDisplayHeight = height

	// Subtract 1 for header row and 1 for border
	v.state.visibleRows = height - 2

	// Ensure minimum of 1 row
	if v.state.visibleRows < 1 {
		v.state.visibleRows = 1
	}
}

func (v *View) toggleRowNumbers() {
	v.state.showRowNumbers = !v.state.showRowNumbers
	v.updateDataTableForItems(v.state.filteredItems)
}

func (v *View) nextPage() {
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	if v.state.currentPage < v.state.totalPages {
		v.state.currentPage++
		v.updateDataTableForItems(v.state.filteredItems)
	} else {
		v.manager.UpdateStatusBar("Already on the last page.")
	}
}

func (v *View) previousPage() {
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	if v.state.currentPage > 1 {
		v.state.currentPage--
		v.updateDataTableForItems(v.state.filteredItems)
	} else {
		v.manager.UpdateStatusBar("Already on the first page.")
	}
}

func (v *View) showItemDetails(item map[string]dynamodbtypes.AttributeValue) {
	table := tview.NewTable()
	table.SetSelectable(true, false).
		SetFixed(1, 0)

	attributes := make([]string, 0, len(item))
	for attr := range item {
		attributes = append(attributes, attr)
	}
	sort.Strings(attributes)

	// Determine the maximum attribute name and value length for dynamic sizing
	maxAttrLen := 0
	maxValLen := 0
	for _, attr := range attributes {
		if len(attr) > maxAttrLen {
			maxAttrLen = len(attr)
		}
		value := attributeValueToString(item[attr])
		if len(value) > maxValLen {
			maxValLen = len(value)
		}
	}

	const minModalWidth = 50
	const maxModalWidth = 120
	calculatedWidth := maxAttrLen + maxValLen + 4
	if calculatedWidth > maxModalWidth {
		calculatedWidth = maxModalWidth
	}
	if calculatedWidth < minModalWidth {
		calculatedWidth = minModalWidth
	}

	for row, attr := range attributes {
		value := attributeValueToString(item[attr])

		attrCell := tview.NewTableCell(attr).
			SetTextColor(tcell.ColorMediumTurquoise).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		table.SetCell(row+1, 0, attrCell)

		valueCell := tview.NewTableCell(value).
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorBeige).Background(tcell.ColorDarkCyan)).
			SetSelectable(true)
		table.SetCell(row+1, 1, valueCell)
	}

	table.SetBorder(true).
		SetTitle(" Item Details (ESC to close, 'y' to copy value) ").
		SetTitleColor(style.GruvboxMaterial.Yellow).
		SetBorderColor(tcell.ColorMediumTurquoise)

	// Calculate modal height based on the number of attributes
	// Add extra rows for padding and header
	modalHeight := len(attributes) + 4 // 1 for header, 2 for padding, 1 buffer

	const minModalHeight = 10
	const maxModalHeight = 30
	if modalHeight < minModalHeight {
		modalHeight = minModalHeight
	} else if modalHeight > maxModalHeight {
		modalHeight = maxModalHeight
	}

	v.showModal(table, types.ModalRowDetails, calculatedWidth, modalHeight, func() {
		v.manager.App().SetFocus(v.dataTable)
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			if event.Rune() == 'y' || event.Rune() == 'Y' {
				row, col := table.GetSelection()
				if row > 0 && col == 1 {
					cell := table.GetCell(row, col)
					value := cell.Text
					// Copy to clipboard
					if err := clipboard.WriteAll(value); err != nil {
						v.manager.UpdateStatusBar("Failed to copy value to clipboard")
					} else {
						v.manager.UpdateStatusBar("Value copied to clipboard")
					}
				}
				return nil
			}
		}
		return event
	})
}

func (v *View) showModal(modal tview.Primitive, name string, width, height int, onDismiss func()) {
	if v.manager.Pages().HasPage(name) {
		return
	}

	modalGrid := tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, width, 0).
		SetBorders(false).
		AddItem(modal, 1, 1, 1, 1, 0, 0, true)

	modalGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			v.manager.Pages().RemovePage(name)
			if onDismiss != nil {
				onDismiss()
			}
			return nil
		}
		return event
	})

	v.manager.Pages().AddPage(name, modalGrid, true, true)
	v.manager.App().SetFocus(modal)
}

func (v *View) filterItems(filter string) {
	if v.state.originalItems == nil || len(v.state.originalItems) == 0 {
		return
	}

	if filter == "" {
		v.state.filteredItems = v.state.originalItems
		v.updateDataTableForItems(v.state.originalItems)
		v.manager.UpdateStatusBar("Showing all items")
		return
	}

	filter = strings.ToLower(filter)
	filtered := make([]map[string]dynamodbtypes.AttributeValue, 0)

	for _, item := range v.state.originalItems {
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

	v.state.filteredItems = filtered
	v.updateDataTableForItems(filtered)

	filterText := filter
	statusMsg := fmt.Sprintf("Page %d/%d | Showing %d of %d items",
		v.state.currentPage,
		v.state.totalPages,
		len(filtered),
		len(v.state.originalItems))

	if filterText != "" {
		statusMsg += fmt.Sprintf(" (filtered: %q)", filterText)
	}

	if v.state.showRowNumbers {
		statusMsg += fmt.Sprintf(" | [%s]Row numbers: on (press 'r' to toggle)[-]",
			style.GruvboxMaterial.Yellow)
	}

	v.manager.UpdateStatusBar(statusMsg)

	if len(filtered) > 0 {
		v.dataTable.Select(1, 0)
		v.dataTable.SetSelectable(true, false)
	}
}
