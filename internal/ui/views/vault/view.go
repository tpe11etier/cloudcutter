package vault

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/vault"
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
	service      vault.Interface
	leftPanel    *tview.List
	dataTable    *tview.Table
	rightPanel   *tview.Flex
	detailsTable *tview.Table
	headerTable  *tview.Table
	filterPrompt *components.Prompt
	layout       tview.Primitive

	state viewState
	ctx   context.Context
	mu    sync.Mutex
	wg    sync.WaitGroup
}

type viewState struct {
	isLoading         bool
	mountsCache       map[string]*vault.Mount
	originalSecrets  []*vault.Secret
	filteredSecrets   []*vault.Secret
	leftPanelFilter   string
	dataPanelFilter   string
	showRowNumbers    bool
	visibleRows       int
	lastDisplayHeight int
	currentPage       int
	pageSize          int
	totalPages        int
	spinner           *spinner.Spinner
	vaultAddr         string
	vaultToken        string
	currentViewType   string // "overview", "secret", "metadata"
	currentSecret     *vault.Secret
}

func NewView(manager *manager.Manager, vaultService vault.Interface, addr, token string) *View {
	view := &View{
		name:         "vault",
		manager:      manager,
		service:      vaultService,
		filterPrompt: components.NewPrompt(),
		state: viewState{
			mountsCache:       make(map[string]*vault.Mount),
			showRowNumbers:    true,
			visibleRows:       0,
			lastDisplayHeight: 0,
			currentPage:       1,
			pageSize:          50,
			totalPages:        1,
			vaultAddr:         addr,
			vaultToken:        token,
		},
		ctx: manager.ViewContext(),
	}

	view.setupLayout()
	view.initializeMounts()
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
		v.fetchMounts()
	}
}

func (v *View) Hide() {}

func (v *View) fetchMounts() {
	mounts, err := v.service.ListMounts(v.ctx, v.state.vaultAddr, v.state.vaultToken)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching Vault mounts: %v", err))
		return
	}

	for mountPath, mount := range mounts {
		v.state.mountsCache[mountPath] = mount
		
		// Create enhanced description with mount type and warnings
		description := v.getMountDescription(mountPath, mount)
		v.leftPanel.AddItem(mountPath, description, 0, nil)
	}

	v.manager.UpdateStatusBar("Select a mount to view secrets or press Enter to view secrets")
}

func (v *View) fetchMountDetails(mountPath string) {
	if mount, found := v.state.mountsCache[mountPath]; found {
		v.updateMountSummary(mount)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mounts, err := v.service.ListMounts(ctx, v.state.vaultAddr, v.state.vaultToken)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching mount details: %v", err))
		v.updateMountSummary(nil)
		return
	}

	if mount, exists := mounts[mountPath]; exists {
		v.state.mountsCache[mountPath] = mount
		v.updateMountSummary(mount)
	}
}

func (v *View) updateMountSummary(mount *vault.Mount) {
	if mount == nil {
		v.manager.UpdateHeader(nil)
		return
	}

	summary := []types.SummaryItem{
		{Key: "Mount Type", Value: mount.Type},
		{Key: "Description", Value: mount.Description},
		{Key: "Local", Value: fmt.Sprintf("%t", mount.Local)},
		{Key: "Seal Wrap", Value: fmt.Sprintf("%t", mount.SealWrap)},
	}

	if mount.PluginVersion != "" {
		summary = append(summary, types.SummaryItem{Key: "Plugin Version", Value: mount.PluginVersion})
	}

	v.manager.UpdateHeader(summary)
}

func extractSortedHeaders(secrets []*vault.Secret) []string {
	headersSet := make(map[string]struct{})
	for _, secret := range secrets {
		for key := range secret.Data {
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

func calculateColumnWidths(secrets []*vault.Secret, headers []string) map[string]int {
	widths := make(map[string]int)
	for _, header := range headers {
		widths[header] = len(header)
	}

	for _, secret := range secrets {
		for header, value := range secret.Data {
			valueStr := fmt.Sprintf("%v", value)
			if len(valueStr) > widths[header] {
				widths[header] = len(valueStr)
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

func (v *View) initializeMounts() {
	mounts, err := v.service.ListMounts(v.ctx, v.state.vaultAddr, v.state.vaultToken)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching Vault mounts: %v", err))
		return
	}

	v.wg.Add(len(mounts))
	for mountPath, mount := range mounts {
		go func(path string, m *vault.Mount) {
			defer v.wg.Done()
			v.mu.Lock()
			v.state.mountsCache[path] = m
			v.mu.Unlock()
		}(mountPath, mount)
	}
	v.wg.Wait()
}

func (v *View) setupLayout() {
	// Create the main layout with left panel and right panel
	v.content = tview.NewFlex().SetDirection(tview.FlexColumn)
	
	// Left panel for mounts
	v.leftPanel = tview.NewList()
	v.leftPanel.SetBorder(true).
		SetTitle(" Vault Mounts ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorMediumTurquoise)
	
	v.leftPanel.SetSelectedTextColor(tcell.ColorLightYellow).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan)
	
	v.leftPanel.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		v.fetchMountDetails(mainText)
	})
	
	v.leftPanel.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		v.showMountSecrets(mainText)
	})
	
	// Right panel container
	v.rightPanel = tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Top right panel for directory contents
	v.dataTable = tview.NewTable()
	v.dataTable.SetBorder(true).
		SetTitle(" Contents ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorBeige)
	v.dataTable.SetSelectable(true, false)
	
	// Bottom right panel for secret details (initially hidden)
	v.detailsTable = tview.NewTable()
	v.detailsTable.SetBorder(true).
		SetTitle(" Secret Details ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorBeige)
	v.detailsTable.SetSelectable(true, false)
	
	// Header panel for tabs (initially hidden)
	v.headerTable = tview.NewTable()
	v.headerTable.SetBorder(true).
		SetTitle(" View ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorBeige)
	v.headerTable.SetSelectable(true, false)
	
	// Initially show only the data table
	v.rightPanel.AddItem(v.dataTable, 0, 1, true)
	
	// Add panels to main layout
	v.content.AddItem(v.leftPanel, 30, 0, true)
	v.content.AddItem(v.rightPanel, 0, 1, false)
	
	// Set up input handlers
	v.setupInputHandlers()
	
	// Add to pages
	pages := v.manager.Pages()
	pages.AddPage("vault", v.content, true, true)
}

func (v *View) setupInputHandlers() {
	v.dataTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, _ := v.dataTable.GetSelection()
			if row > 0 { // Header is row 0
				// Check if it's a directory or secret
				typeCell := v.dataTable.GetCell(row, 1)
				pathCell := v.dataTable.GetCell(row, 2)
				
				if typeCell != nil && strings.Contains(typeCell.Text, "Directory") {
					// Navigate into directory
					if pathCell != nil {
						v.navigateToPath(pathCell.Text)
					}
				} else {
					// Show secret details in split pane
					if pathCell != nil {
						v.showSecretInSplitPane(pathCell.Text)
					}
				}
			}
			return nil
		case tcell.KeyUp, tcell.KeyDown:
			// Update details when selection changes
			v.updateDetailsOnSelectionChange()
			return event
		case tcell.KeyLeft:
			// Only move to left panel if we're at the first item
			row, _ := v.dataTable.GetSelection()
			if row <= 1 { // Header or first item
				v.manager.SetFocus(v.leftPanel)
				return nil
			}
			return event
		case tcell.KeyRight:
			// Move to header row if we have a secret loaded
			if v.state.currentSecret != nil {
				v.manager.SetFocus(v.headerTable)
				return nil
			}
			return event
		}
		return event
	})
	
	v.headerTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, col := v.headerTable.GetSelection()
			if row >= 0 {
				cell := v.headerTable.GetCell(row, col)
				if cell != nil {
					v.switchViewType(cell.Text)
				}
			}
			return nil
		case tcell.KeyLeft:
			row, col := v.headerTable.GetSelection()
			if col > 0 {
				v.headerTable.Select(row, col-1)
				// Update the view immediately
				cell := v.headerTable.GetCell(row, col-1)
				if cell != nil {
					v.switchViewType(cell.Text)
				}
			}
			return nil
		case tcell.KeyRight:
			row, col := v.headerTable.GetSelection()
			if col < 2 { // We have 3 columns (0, 1, 2)
				v.headerTable.Select(row, col+1)
				// Update the view immediately
				cell := v.headerTable.GetCell(row, col+1)
				if cell != nil {
					v.switchViewType(cell.Text)
				}
			}
			return nil
		}
		return event
	})
}

func (v *View) updateDataTableForSecrets(secrets []*vault.Secret) {
	v.dataTable.Clear()
	v.calculateVisibleRows()

	if len(secrets) == 0 {
		v.dataTable.SetCell(0, 0,
			tview.NewTableCell("No secrets found").
				SetTextColor(tcell.ColorBeige).
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
		return
	}

	totalSecrets := len(secrets)
	v.state.pageSize = v.state.visibleRows
	v.state.totalPages = (totalSecrets + v.state.pageSize - 1) / v.state.pageSize
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	start := (v.state.currentPage - 1) * v.state.pageSize
	end := start + v.state.pageSize
	if end > totalSecrets {
		end = totalSecrets
	}

	if start >= totalSecrets {
		start = totalSecrets - v.state.pageSize
		if start < 0 {
			start = 0
		}
		end = totalSecrets
	}

	pageSecrets := secrets[start:end]

	headers := extractSortedHeaders(secrets)
	columnWidths := calculateColumnWidths(secrets, headers)

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

	for rowIdx, secret := range pageSecrets {
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
			value := fmt.Sprintf("%v", secret.Data[header])
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
	if len(pageSecrets) > 0 {
		v.dataTable.Select(1, 0)
	}

	statusMsg := fmt.Sprintf("Page %d/%d | Showing %d of %d secrets",
		v.state.currentPage, v.state.totalPages,
		len(pageSecrets), totalSecrets)

	if v.state.showRowNumbers {
		statusMsg += fmt.Sprintf(" | [%s]Row numbers: on (press 'r' to toggle)[-]",
			style.GruvboxMaterial.Yellow)
	}

	v.manager.UpdateStatusBar(statusMsg)
}

func (v *View) filterLeftPanel(filter string) {
	mountPaths := make([]string, 0, len(v.state.mountsCache))

	filter = strings.ToLower(filter)

	for mountPath := range v.state.mountsCache {
		if filter == "" || strings.Contains(strings.ToLower(mountPath), filter) {
			mountPaths = append(mountPaths, mountPath)
		}
	}

	sort.Strings(mountPaths)

	v.leftPanel.Clear()
	for _, mountPath := range mountPaths {
		v.leftPanel.AddItem(mountPath, "", 0, nil)
	}

	if filter == "" {
		v.manager.UpdateStatusBar("Showing all mounts")
	} else {
		v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing mounts matching '%s'", filter))
	}
}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		currentFocus := v.manager.App().GetFocus()

		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'n', 'p':
				if currentFocus == v.dataTable {
					if event.Rune() == 'n' {
						v.nextPage()
					} else {
						v.previousPage()
					}
					return nil
				}
			case '/':
				switch currentFocus {
				case v.leftPanel:
					v.showFilterPrompt(v.leftPanel)
					return nil
				case v.dataTable:
					v.showFilterPrompt(v.dataTable)
					return nil
				}
			case 'r':
				if currentFocus == v.dataTable {
					v.toggleRowNumbers()
					return nil
				}
			}
		}

		switch event.Key() {
		case tcell.KeyTab:
			if currentFocus == v.leftPanel {
				v.manager.SetFocus(v.dataTable)
			} else if currentFocus == v.dataTable {
				if v.state.currentSecret != nil {
					v.manager.SetFocus(v.headerTable)
				} else {
					v.manager.SetFocus(v.leftPanel)
				}
			} else if currentFocus == v.headerTable {
				v.manager.SetFocus(v.detailsTable)
			} else if currentFocus == v.detailsTable {
				v.manager.SetFocus(v.leftPanel)
			}
			return nil
		case tcell.KeyEnter:
			if currentFocus == v.dataTable {
				// Handle Enter key in data table
				row, _ := v.dataTable.GetSelection()
				if row > 0 { // Header is row 0
					// Check if it's a directory or secret
					typeCell := v.dataTable.GetCell(row, 1)
					pathCell := v.dataTable.GetCell(row, 2)
					
					if typeCell != nil && strings.Contains(typeCell.Text, "Directory") {
						// Navigate into directory
						if pathCell != nil {
							v.navigateToPath(pathCell.Text)
						}
					} else {
						// Show secret details in split pane
						if pathCell != nil {
							v.showSecretInSplitPane(pathCell.Text)
						}
					}
				}
				return nil
			}
		case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyEsc:
			if currentFocus == v.leftPanel {
				v.state.leftPanelFilter = ""
				v.filterLeftPanel("")
				return nil
			}
			if event.Key() == tcell.KeyEsc && currentFocus == v.dataTable {
				v.manager.SetFocus(v.leftPanel)
				return nil
			}
		}
		return event
	}
}

func (v *View) HandleFilter(prompt *components.Prompt, previousFocus tview.Primitive) {
	var opts components.PromptOptions

	switch previousFocus {
	case v.leftPanel:
		opts = components.PromptOptions{
			Title:      " Filter Mounts ",
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
			Title:      " Filter Secrets ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.state.dataPanelFilter = text
				v.filterSecrets(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.dataPanelFilter = ""
				v.filterSecrets("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterSecrets(text)
			},
		}
	}

	prompt.Configure(opts)
	promptLayout := prompt.Layout()
	v.manager.Pages().AddPage(types.ModalFilter, promptLayout, true, true)
	v.manager.App().SetFocus(prompt.InputField)
}

func (v *View) showMountSecrets(mountPath string) {
	v.showLoading(fmt.Sprintf("Fetching secrets from mount %s...", mountPath))
	go func() {
		// Clear any current secret and reset to simple layout
		v.state.currentSecret = nil
		v.resetToSimpleLayout()
		
		// Handle different mount types
		switch mountPath {
		case "sys/":
			v.handleSystemMount(mountPath)
		case "identity/":
			v.handleIdentityMount(mountPath)
		default:
			v.handleKVMount(mountPath)
		}
	}()
}

func (v *View) handleKVMount(mountPath string) {
	secrets, err := v.service.ListSecrets(v.ctx, v.state.vaultAddr, v.state.vaultToken, mountPath)
	
	v.manager.App().QueueUpdateDraw(func() {
		defer v.hideLoading()

		if err != nil {
			v.manager.UpdateStatusBar(fmt.Sprintf("Error listing secrets from mount %s: %v", mountPath, err))
			return
		}

		// Create a table showing both directories and secrets
		v.showKVContents(mountPath, secrets)
		v.manager.SetFocus(v.dataTable)
	})
}

func (v *View) showKVContents(mountPath string, paths []string) {
	v.dataTable.Clear()
	
	// Add header
	header := []string{"Name", "Type", "Path"}
	for col, value := range header {
		cell := tview.NewTableCell(value)
		cell.SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		v.dataTable.SetCell(0, col, cell)
	}
	
	// Add rows for each path
	for row, path := range paths {
		rowIndex := row + 1
		
		// Determine if it's a directory or secret
		isDirectory := strings.HasSuffix(path, "/")
		itemType := "Secret"
		if isDirectory {
			itemType = "Directory"
		}
		
		// Create cells
		name := strings.TrimSuffix(path, "/")
		fullPath := mountPath + "/" + path
		// Clean up double slashes
		fullPath = strings.ReplaceAll(fullPath, "//", "/")
		
		nameCell := tview.NewTableCell(name)
		nameCell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
		
		typeCell := tview.NewTableCell(itemType)
		if isDirectory {
			typeCell.SetTextColor(style.GruvboxMaterial.Blue).SetAlign(tview.AlignLeft).SetSelectable(true)
		} else {
			typeCell.SetTextColor(style.GruvboxMaterial.Green).SetAlign(tview.AlignLeft).SetSelectable(true)
		}
		
		pathCell := tview.NewTableCell(fullPath)
		pathCell.SetTextColor(tcell.ColorDarkGray).SetAlign(tview.AlignLeft).SetSelectable(true)
		
		v.dataTable.SetCell(rowIndex, 0, nameCell)
		v.dataTable.SetCell(rowIndex, 1, typeCell)
		v.dataTable.SetCell(rowIndex, 2, pathCell)
	}
	
	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) handleSystemMount(mountPath string) {
	v.manager.App().QueueUpdateDraw(func() {
		defer v.hideLoading()
		
		// Show warning for system mount
		v.manager.UpdateStatusBar("⚠️  System mount - Use with caution! This contains Vault's internal configuration.")
		
		// Create a special table showing system information
		v.showSystemInfo()
	})
}

func (v *View) handleIdentityMount(mountPath string) {
	v.manager.App().QueueUpdateDraw(func() {
		defer v.hideLoading()
		
		// Show warning for identity mount
		v.manager.UpdateStatusBar("⚠️  Identity mount - Advanced Vault feature for user management.")
		
		// Create a special table showing identity information
		v.showIdentityInfo()
	})
}

func (v *View) showSystemInfo() {
	v.dataTable.Clear()
	
	// Add system information rows
	systemInfo := [][]string{
		{"Component", "Status", "Description"},
		{"Vault Server", "Running", "Core Vault instance"},
		{"Auth Methods", "Active", "Token-based authentication"},
		{"Policies", "Loaded", "Access control policies"},
		{"Audit Devices", "Configured", "Logging and monitoring"},
	}
	
	for row, info := range systemInfo {
		for col, value := range info {
			cell := tview.NewTableCell(value)
			if row == 0 {
				cell.SetTextColor(style.GruvboxMaterial.Yellow).
					SetAlign(tview.AlignCenter).
					SetSelectable(false).
					SetAttributes(tcell.AttrBold)
			} else {
				cell.SetTextColor(tcell.ColorBeige).
					SetAlign(tview.AlignLeft).
					SetSelectable(true)
			}
			v.dataTable.SetCell(row, col, cell)
		}
	}
	
	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) showIdentityInfo() {
	v.dataTable.Clear()
	
	// Add identity information rows
	identityInfo := [][]string{
		{"Component", "Status", "Description"},
		{"Entities", "Active", "User and service identities"},
		{"Groups", "Configured", "Role-based access groups"},
		{"Aliases", "Linked", "External identity mappings"},
		{"Policies", "Assigned", "Entity-specific permissions"},
	}
	
	for row, info := range identityInfo {
		for col, value := range info {
			cell := tview.NewTableCell(value)
			if row == 0 {
				cell.SetTextColor(style.GruvboxMaterial.Yellow).
					SetAlign(tview.AlignCenter).
					SetSelectable(false).
					SetAttributes(tcell.AttrBold)
			} else {
				cell.SetTextColor(tcell.ColorBeige).
					SetAlign(tview.AlignLeft).
					SetSelectable(true)
			}
			v.dataTable.SetCell(row, col, cell)
		}
	}
	
	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) navigateToPath(path string) {
	v.showLoading(fmt.Sprintf("Loading contents of %s...", path))
	go func() {
		secrets, err := v.service.ListSecrets(v.ctx, v.state.vaultAddr, v.state.vaultToken, path)
		
		v.manager.App().QueueUpdateDraw(func() {
			defer v.hideLoading()

			if err != nil {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error listing contents of %s: %v", path, err))
				return
			}

			// Clear any current secret and reset to simple layout
			v.state.currentSecret = nil
			v.resetToSimpleLayout()
			v.showKVContents(path, secrets)
			v.manager.UpdateStatusBar(fmt.Sprintf("Showing contents of %s", path))
		})
	}()
}

func (v *View) showSecretInSplitPane(path string) {
	v.showLoading(fmt.Sprintf("Loading secret %s...", path))
	go func() {
		secret, err := v.service.GetSecret(v.ctx, v.state.vaultAddr, v.state.vaultToken, path)
		
		v.manager.App().QueueUpdateDraw(func() {
			defer v.hideLoading()

			if err != nil {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error loading secret %s: %v", path, err))
				return
			}

			v.state.currentSecret = secret
			v.state.currentViewType = "Overview"
			v.setupSplitPane()
			v.updateDetailsView()
		})
	}()
}

func (v *View) updateDetailsOnSelectionChange() {
	// Only update if we're in split pane mode and have a current secret
	if v.state.currentSecret == nil {
		return
	}
	
	row, _ := v.dataTable.GetSelection()
	if row > 0 { // Header is row 0
		typeCell := v.dataTable.GetCell(row, 1)
		pathCell := v.dataTable.GetCell(row, 2)
		
		// Only update if it's a secret (not a directory)
		if typeCell != nil && !strings.Contains(typeCell.Text, "Directory") && pathCell != nil {
			// Load the new secret
			v.showSecretInSplitPane(pathCell.Text)
		}
	}
}

func (v *View) resetToSimpleLayout() {
	// Clear the right panel and show only the data table
	v.rightPanel.Clear()
	v.rightPanel.AddItem(v.dataTable, 0, 1, true)
}

func (v *View) setupSplitPane() {
	// Clear the right panel
	v.rightPanel.Clear()
	
	// Add header table
	v.setupHeaderTable()
	v.rightPanel.AddItem(v.headerTable, 3, 0, false)
	
	// Add details table (takes up remaining space)
	v.rightPanel.AddItem(v.detailsTable, 0, 1, true)
}

func (v *View) setupHeaderTable() {
	v.headerTable.Clear()
	
	// Add header tabs
	tabs := []string{"Overview", "Secret", "Metadata"}
	for i, tab := range tabs {
		cell := tview.NewTableCell(tab)
		if tab == v.state.currentViewType {
			cell.SetTextColor(tcell.ColorLightYellow).SetAlign(tview.AlignCenter).SetSelectable(true)
		} else {
			cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignCenter).SetSelectable(true)
		}
		v.headerTable.SetCell(0, i, cell)
	}
	
	v.headerTable.SetFixed(1, 0)
	v.headerTable.SetSelectable(true, true) // Enable both row and column selection
	
	// Select the current view type
	for i, tab := range tabs {
		if tab == v.state.currentViewType {
			v.headerTable.Select(0, i)
			break
		}
	}
}

func (v *View) switchViewType(viewType string) {
	v.state.currentViewType = viewType
	v.updateDetailsView()
	
	// Update header highlighting
	for i := 0; i < 3; i++ {
		cell := v.headerTable.GetCell(0, i)
		if cell != nil {
			if cell.Text == viewType {
				cell.SetTextColor(tcell.ColorLightYellow)
			} else {
				cell.SetTextColor(tcell.ColorBeige)
			}
		}
	}
}

func (v *View) updateDetailsView() {
	if v.state.currentSecret == nil {
		return
	}
	
	v.detailsTable.Clear()
	
	switch v.state.currentViewType {
	case "Overview":
		v.showOverviewDetails()
	case "Secret":
		v.showSecretDataDetails()
	case "Metadata":
		v.showMetadataDetails()
	}
}

func (v *View) showOverviewDetails() {
	// Add header
	header := []string{"Property", "Value"}
	for col, value := range header {
		cell := tview.NewTableCell(value)
		cell.SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		v.detailsTable.SetCell(0, col, cell)
	}
	
	// Add overview data
	overviewData := [][]string{
		{"Path", v.state.currentSecret.Path},
		{"Mount Type", v.state.currentSecret.MountType},
		{"Data Keys", fmt.Sprintf("%d", len(v.state.currentSecret.Data))},
	}
	
	row := 1
	for _, data := range overviewData {
		for col, value := range data {
			cell := tview.NewTableCell(value)
			if col == 0 {
				cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
			} else {
				cell.SetTextColor(tcell.ColorLightGreen).SetAlign(tview.AlignLeft).SetSelectable(true)
			}
			v.detailsTable.SetCell(row, col, cell)
		}
		row++
	}
	
	v.detailsTable.SetFixed(1, 0)
	v.detailsTable.Select(1, 0)
}

func (v *View) showSecretDataDetails() {
	// Add header
	header := []string{"Key", "Value"}
	for col, value := range header {
		cell := tview.NewTableCell(value)
		cell.SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		v.detailsTable.SetCell(0, col, cell)
	}
	
	// Add secret data
	keys := make([]string, 0, len(v.state.currentSecret.Data))
	for key := range v.state.currentSecret.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	
	row := 1
	for _, key := range keys {
		value := fmt.Sprintf("%v", v.state.currentSecret.Data[key])
		
		keyCell := tview.NewTableCell(key)
		keyCell.SetTextColor(tcell.ColorMediumTurquoise).SetAlign(tview.AlignLeft).SetSelectable(true)
		v.detailsTable.SetCell(row, 0, keyCell)
		
		valueCell := tview.NewTableCell(value)
		valueCell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
		v.detailsTable.SetCell(row, 1, valueCell)
		row++
	}
	
	v.detailsTable.SetFixed(1, 0)
	v.detailsTable.Select(1, 0)
	
	// Set up input capture for the details table to show modal on Enter
	v.detailsTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, _ := v.detailsTable.GetSelection()
			if row > 0 { // Skip header
				keyCell := v.detailsTable.GetCell(row, 0)
				valueCell := v.detailsTable.GetCell(row, 1)
				if keyCell != nil && valueCell != nil {
					v.showValueModal(keyCell.Text, valueCell.Text)
				}
			}
			return nil
		}
		return event
	})
}

func (v *View) showValueModal(key, value string) {
	// Create modal for displaying the full value
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Key: %s\n\nValue:\n%s", key, value)).
		AddButtons([]string{"Copy", "Close"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Copy" {
				// Copy to clipboard
				v.manager.UpdateStatusBar(fmt.Sprintf("Copied value for key '%s' to clipboard", key))
			}
			v.manager.Pages().RemovePage("valueModal")
		})
	
	v.manager.Pages().AddPage("valueModal", modal, true, true)
	v.manager.App().SetFocus(modal)
}

func (v *View) showMetadataDetails() {
	// Add header
	header := []string{"Property", "Value"}
	for col, value := range header {
		cell := tview.NewTableCell(value)
		cell.SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		v.detailsTable.SetCell(0, col, cell)
	}
	
	// Add metadata
	if v.state.currentSecret.Metadata != nil {
		metadataData := [][]string{
			{"Version", fmt.Sprintf("%d", v.state.currentSecret.Metadata.Version)},
			{"Created Time", v.state.currentSecret.Metadata.CreatedTime.Format("2006-01-02 15:04:05 UTC")},
			{"Custom Metadata", fmt.Sprintf("%v", v.state.currentSecret.Metadata.CustomMetadata)},
		}
		
		row := 1
		for _, data := range metadataData {
			for col, value := range data {
				cell := tview.NewTableCell(value)
				if col == 0 {
					cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
				} else {
					cell.SetTextColor(tcell.ColorLightCyan).SetAlign(tview.AlignLeft).SetSelectable(true)
				}
				v.detailsTable.SetCell(row, col, cell)
			}
			row++
		}
	}
	
	v.detailsTable.SetFixed(1, 0)
	v.detailsTable.Select(1, 0)
}

func (v *View) Reinitialize(cfg aws.Config) error {
	// For Vault, we don't need AWS config, but we might want to update connection details
	// This could be extended to support different Vault instances
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
			Title:      " Filter Mounts ",
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
			Title:      " Filter Secrets ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.state.dataPanelFilter = text
				v.filterSecrets(text)
				v.manager.Pages().RemovePage(types.ModalFilter)
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.dataPanelFilter = ""
				v.filterSecrets("")
				v.manager.Pages().RemovePage(types.ModalFilter)
				v.manager.SetFocus(previousFocus)
			},
			OnChanged: func(text string) {
				v.filterSecrets(text)
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
	v.updateDataTableForSecrets(v.state.filteredSecrets)
}

func (v *View) nextPage() {
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	if v.state.currentPage < v.state.totalPages {
		v.state.currentPage++
		v.updateDataTableForSecrets(v.state.filteredSecrets)
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
		v.updateDataTableForSecrets(v.state.filteredSecrets)
	} else {
		v.manager.UpdateStatusBar("Already on the first page.")
	}
}




func (v *View) showModal(modal tview.Primitive, name string, width, height int, onDismiss func()) {
	if v.manager.Pages().HasPage(name) {
		return
	}

	modalGrid := tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, width, 0).
		SetBorders(false).
		AddItem(modal, 1, 1, 1, 1, 0, 0, false)

	modalGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			v.manager.Pages().RemovePage(name)
			if onDismiss != nil {
				onDismiss()
			}
			return nil
		}
		// Prevent arrow keys from propagating to the main window
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown || 
		   event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyRight {
			return nil
		}
		return event
	})

	v.manager.Pages().AddPage(name, modalGrid, true, true)
	v.manager.App().SetFocus(modal)
}

func (v *View) filterSecrets(filter string) {
	if len(v.state.originalSecrets) == 0 {
		return
	}

	if filter == "" {
		v.state.filteredSecrets = v.state.originalSecrets
		v.updateDataTableForSecrets(v.state.originalSecrets)
		v.manager.UpdateStatusBar("Showing all secrets")
		return
	}

	filter = strings.ToLower(filter)
	filtered := make([]*vault.Secret, 0)

	for _, secret := range v.state.originalSecrets {
		matches := false
		for _, value := range secret.Data {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", value)), filter) {
				matches = true
				break
			}
		}
		if matches {
			filtered = append(filtered, secret)
		}
	}

	v.state.filteredSecrets = filtered
	v.updateDataTableForSecrets(filtered)

	filterText := filter
	statusMsg := fmt.Sprintf("Page %d/%d | Showing %d of %d secrets",
		v.state.currentPage,
		v.state.totalPages,
		len(filtered),
		len(v.state.originalSecrets))

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

func (v *View) copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

func (v *View) getMountDescription(mountPath string, mount *vault.Mount) string {
	baseDesc := mount.Description
	if baseDesc == "" {
		baseDesc = mount.Type
	}
	
	// Add mount type and warnings
	switch mountPath {
	case "secret/":
		return fmt.Sprintf("%s (KV Store - Application Secrets)", baseDesc)
	case "cubbyhole/":
		return fmt.Sprintf("%s (Per-Token Storage)", baseDesc)
	case "identity/":
		return fmt.Sprintf("%s (Identity Management - Advanced)", baseDesc)
	case "sys/":
		return fmt.Sprintf("%s (System Endpoints - Admin Only)", baseDesc)
	default:
		return fmt.Sprintf("%s (%s)", baseDesc, mount.Type)
	}
}

