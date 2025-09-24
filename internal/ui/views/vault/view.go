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
	name          string
	content       *tview.Flex
	manager       *manager.Manager
	service       vault.Interface
	leftPanel     *tview.List
	dataTable     *tview.Table
	rightPanel    *tview.Flex
	detailsTable  *tview.Table
	breadcrumbBar *tview.Table
	filterPrompt  *components.Prompt

	state viewState
	ctx   context.Context
	mu    sync.Mutex
	wg    sync.WaitGroup
}

type viewState struct {
	isLoading         bool
	mountsCache       map[string]*vault.Mount
	originalSecrets   []*vault.Secret
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
	currentViewType   string // "overview", "secrets", "metadata", "paths", "history"
	currentSecret     *vault.Secret
	currentPath       string // Current navigation path
	breadcrumbVisible bool   // Whether to show breadcrumb navigation
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
		v.leftPanel.AddItem(mountPath, "", 0, nil)
	}

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
	v.leftPanel.ShowSecondaryText(false)

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
		SetTitle(" Contents: ↑↓ Navigate | y Copy ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorMediumTurquoise)
	v.dataTable.SetSelectable(true, false).
		SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorLightYellow).Background(tcell.ColorDarkCyan))

	// Bottom right panel for secret details (initially hidden)
	v.detailsTable = tview.NewTable()
	v.detailsTable.SetBorder(true).
		SetTitle(" Secret Details ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorBeige)
	v.detailsTable.SetSelectable(true, false)

	// Create breadcrumb navigation bar (initially hidden)
	v.breadcrumbBar = tview.NewTable()
	v.breadcrumbBar.SetBorder(true).
		SetTitle(" Navigation: ← → Navigate views ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorMediumTurquoise)
	v.breadcrumbBar.SetSelectable(true, false)

	// Initially show only the data table
	v.rightPanel.AddItem(v.dataTable, 0, 1, true)

	// Add panels to main layout
	v.content.AddItem(v.leftPanel, 30, 0, true)
	v.content.AddItem(v.rightPanel, 0, 1, false)


	// Add to pages
	pages := v.manager.Pages()
	pages.AddPage("vault", v.content, true, true)
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
			case 'y':
				v.handleCopyToClipboard()
				return nil
			case '?', 'h':
				v.showHelpModal()
				return nil
			case '1', '2', '3', '4', '5':
				// Quick view switching with number keys when breadcrumbs are visible
				if v.state.breadcrumbVisible {
					viewTypes := []string{"Overview", "Secrets", "Metadata", "Paths", "History"}
					viewIndex := int(event.Rune() - '1')
					if viewIndex >= 0 && viewIndex < len(viewTypes) {
						v.switchToView(viewTypes[viewIndex])
					}
					return nil
				}
			}
		}

		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight:
			// Allow side arrow keys to navigate breadcrumb views when breadcrumbs are visible
			if v.state.breadcrumbVisible && currentFocus == v.dataTable {
				viewTypes := []string{"Overview", "Secrets", "Metadata", "Paths", "History"}
				currentIndex := -1
				for i, viewType := range viewTypes {
					if viewType == v.state.currentViewType {
						currentIndex = i
						break
					}
				}

				if currentIndex >= 0 {
					if event.Key() == tcell.KeyLeft && currentIndex > 0 {
						v.switchToView(viewTypes[currentIndex-1])
					} else if event.Key() == tcell.KeyRight && currentIndex < len(viewTypes)-1 {
						v.switchToView(viewTypes[currentIndex+1])
					}
				}
				return nil
			}
		case tcell.KeyTab:
			if currentFocus == v.leftPanel {
				v.manager.SetFocus(v.dataTable)
			} else {
				v.manager.App().SetFocus(v.leftPanel)
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
						// Navigate into directory or try to navigate first, fallback to secret modal
						if pathCell != nil {
							// For "Secret/Directory", try navigation first, if it fails, show as secret
							if strings.Contains(typeCell.Text, "Secret/Directory") {
								v.tryNavigateOrShowSecret(pathCell.Text)
							} else {
								v.navigateToPath(pathCell.Text)
							}
						}
					} else {
						// Show secret details in modal
						if pathCell != nil {
							v.showSecretModal(pathCell.Text)
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
				v.manager.App().SetFocus(v.leftPanel)
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
	// Clear the right panel data immediately when switching mounts
	v.dataTable.Clear()
	v.resetToSimpleLayout() // Ensure we're in simple layout (no breadcrumbs)

	v.showLoading(fmt.Sprintf("Fetching secrets from mount %s...", mountPath))
	go func() {
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
		// For Vault KV, directories end with "/" and secrets don't
		isDirectory := strings.HasSuffix(path, "/")
		itemType := "Secret"
		if isDirectory {
			itemType = "Directory"
		} else {
			// For paths without trailing slash, we need to try to determine if it's a directory
			// by attempting to list it. For now, assume it's a secret unless it has a trailing slash
			itemType = "Secret/Directory" // Indicate it could be either
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

func (v *View) handleSystemMount(_ string) {
	v.manager.App().QueueUpdateDraw(func() {
		defer v.hideLoading()

		// Show warning for system mount
		v.manager.UpdateStatusBar("⚠️  System mount - Use with caution! This contains Vault's internal configuration.")

		// Create a special table showing system information
		v.showSystemInfo()
		v.manager.SetFocus(v.dataTable)
	})
}

func (v *View) handleIdentityMount(_ string) {
	v.manager.App().QueueUpdateDraw(func() {
		defer v.hideLoading()

		// Show warning for identity mount
		v.manager.UpdateStatusBar("⚠️  Identity mount - Advanced Vault feature for user management.")

		// Create a special table showing identity information
		v.showIdentityInfo()
		v.manager.SetFocus(v.dataTable)
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

			// Just show directory contents - NO breadcrumbs for directory listings
			v.state.currentSecret = nil
			v.state.currentPath = path
			v.resetToSimpleLayout() // Make sure we're using simple layout
			v.showKVContents(path, secrets)
			v.manager.SetFocus(v.dataTable) // Focus on right panel
			v.manager.UpdateStatusBar(fmt.Sprintf("Showing contents of %s | ↑↓ Navigate | Enter: View item | Tab: Switch panels | /: Filter | ?: Help", path))
		})
	}()
}

func (v *View) tryNavigateOrShowSecret(path string) {
	// First try to navigate (as if it's a directory)
	v.showLoading(fmt.Sprintf("Loading contents of %s...", path))
	go func() {
		secrets, err := v.service.ListSecrets(v.ctx, v.state.vaultAddr, v.state.vaultToken, path)

		v.manager.App().QueueUpdateDraw(func() {
			defer v.hideLoading()

			if err != nil {
				// If navigation fails, try showing as secret instead WITH breadcrumbs
				v.showSecretWithBreadcrumbs(path)
				return
			}

			// Just show directory contents - NO breadcrumbs for directory listings
			v.state.currentSecret = nil
			v.state.currentPath = path
			v.resetToSimpleLayout() // Make sure we're using simple layout
			v.showKVContents(path, secrets)
			v.manager.SetFocus(v.dataTable) // Focus on right panel
			v.manager.UpdateStatusBar(fmt.Sprintf("Showing contents of %s | ↑↓ Navigate | Enter: View item | Tab: Switch panels | /: Filter | ?: Help", path))
		})
	}()
}

func (v *View) showSecretModal(path string) {
	v.showLoading(fmt.Sprintf("Loading secret %s...", path))
	go func() {
		secret, err := v.service.GetSecret(v.ctx, v.state.vaultAddr, v.state.vaultToken, path)

		v.manager.App().QueueUpdateDraw(func() {
			defer v.hideLoading()

			if err != nil {
				v.showErrorModal("Error Loading Secret", fmt.Sprintf("Failed to load secret %s: %v", path, err))
				return
			}

			v.showSecretDetailsModal(secret)
		})
	}()
}

func (v *View) showSecretWithBreadcrumbs(path string) {
	v.showLoading(fmt.Sprintf("Loading secret %s...", path))
	go func() {
		secret, err := v.service.GetSecret(v.ctx, v.state.vaultAddr, v.state.vaultToken, path)

		v.manager.App().QueueUpdateDraw(func() {
			defer v.hideLoading()

			if err != nil {
				v.showErrorModal("Error Loading Secret", fmt.Sprintf("Failed to load secret %s: %v", path, err))
				return
			}

			// This is when we show breadcrumbs - when viewing a specific secret
			v.state.currentSecret = secret
			v.state.currentPath = path
			v.state.currentViewType = "Overview"
			v.showBreadcrumbLayout()
			v.updateBreadcrumbBar(path)

			// Show the Overview view by default (like the web interface)
			v.showOverviewView()
			v.manager.SetFocus(v.dataTable) // Focus on right panel
			v.manager.UpdateStatusBar(fmt.Sprintf("Viewing secret %s", path))
		})
	}()
}

func (v *View) resetToSimpleLayout() {
	// Clear the right panel and show only the data table
	v.rightPanel.Clear()
	v.state.breadcrumbVisible = false
	v.rightPanel.AddItem(v.dataTable, 0, 1, true)
}

func (v *View) showBreadcrumbLayout() {
	// Clear and set up layout with breadcrumb navigation
	v.rightPanel.Clear()
	v.state.breadcrumbVisible = true

	// Add breadcrumb bar (small fixed height)
	v.rightPanel.AddItem(v.breadcrumbBar, 4, 0, false)

	// Add data table (remaining space)
	v.rightPanel.AddItem(v.dataTable, 0, 1, true)
}

func (v *View) updateBreadcrumbBar(path string) {
	if !v.state.breadcrumbVisible {
		return
	}

	v.breadcrumbBar.Clear()

	// Available views for the current context
	viewTypes := []string{"Overview", "Secrets", "Metadata", "Paths", "History"}

	for col, viewName := range viewTypes {
		cell := tview.NewTableCell(viewName)

		// Clean breadcrumb colors - selected is turquoise on black, inactive is black on turquoise
		if viewName == v.state.currentViewType {
			cell.SetTextColor(tcell.ColorMediumTurquoise).
				SetBackgroundColor(tcell.ColorBlack).
				SetAttributes(tcell.AttrBold)
		} else {
			cell.SetTextColor(tcell.ColorBlack).
				SetBackgroundColor(tcell.ColorMediumTurquoise)
		}

		cell.SetAlign(tview.AlignCenter).
			SetSelectable(true)

		v.breadcrumbBar.SetCell(0, col, cell)
	}

	// Add path information in second row
	pathCell := tview.NewTableCell(fmt.Sprintf("Path: %s", path)).
		SetTextColor(tcell.ColorGray).
		SetAlign(tview.AlignLeft).
		SetSelectable(false)
	v.breadcrumbBar.SetCell(1, 0, pathCell)

	// Span the path across all columns
	for col := 1; col < len(viewTypes); col++ {
		v.breadcrumbBar.SetCell(1, col, tview.NewTableCell("").SetSelectable(false))
	}

	// Add navigation instructions in third row
	instructionsCell := tview.NewTableCell("1-5: Quick switch | ← → : Navigate views | Enter: Select | Tab: Switch panels").
		SetTextColor(tcell.ColorDarkGray).
		SetAlign(tview.AlignCenter).
		SetSelectable(false)
	v.breadcrumbBar.SetCell(2, 0, instructionsCell)

	// Span instructions across all columns
	for col := 1; col < len(viewTypes); col++ {
		v.breadcrumbBar.SetCell(2, col, tview.NewTableCell("").SetSelectable(false))
	}

	v.breadcrumbBar.SetFixed(3, 0)
	v.breadcrumbBar.SetSelectable(true, false)

	// Set up input handling for breadcrumb navigation
	v.breadcrumbBar.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, col := v.breadcrumbBar.GetSelection()
			if row == 0 && col < len(viewTypes) {
				v.switchToView(viewTypes[col])
			}
			return nil
		case tcell.KeyLeft:
			row, col := v.breadcrumbBar.GetSelection()
			if col > 0 {
				v.breadcrumbBar.Select(row, col-1)
			}
			return nil
		case tcell.KeyRight:
			row, col := v.breadcrumbBar.GetSelection()
			if col < len(viewTypes)-1 {
				v.breadcrumbBar.Select(row, col+1)
			}
			return nil
		case tcell.KeyDown:
			v.manager.SetFocus(v.dataTable)
			return nil
		}
		return event
	})
}

func (v *View) switchToView(viewType string) {
	v.state.currentViewType = viewType
	v.updateBreadcrumbBar(v.state.currentPath)

	// Update the data table based on view type
	switch viewType {
	case "Overview":
		v.showOverviewView()
	case "Secrets":
		v.showSecretsView()
	case "Metadata":
		v.showMetadataView()
	case "Paths":
		v.showPathsView()
	case "History":
		v.showHistoryView()
	}
}

func (v *View) Reinitialize(_ aws.Config) error {
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

// handleCopyToClipboard handles copying the currently selected item to clipboard
func (v *View) handleCopyToClipboard() {
	currentFocus := v.manager.App().GetFocus()

	switch currentFocus {
	case v.dataTable:
		row, col := v.dataTable.GetSelection()
		if row > 0 { // Skip header
			cell := v.dataTable.GetCell(row, col)
			if cell != nil {
				if err := v.copyToClipboard(cell.Text); err != nil {
					v.manager.UpdateStatusBar("Failed to copy to clipboard")
				} else {
					v.manager.UpdateStatusBar(fmt.Sprintf("Copied \"%s\" to clipboard", cell.Text))
				}
			}
		}
	case v.leftPanel:
		currentItem := v.leftPanel.GetCurrentItem()
		mainText, _ := v.leftPanel.GetItemText(currentItem)
		if err := v.copyToClipboard(mainText); err != nil {
			v.manager.UpdateStatusBar("Failed to copy to clipboard")
		} else {
			v.manager.UpdateStatusBar(fmt.Sprintf("Copied \"%s\" to clipboard", mainText))
		}
	default:
		v.manager.UpdateStatusBar("Nothing selected to copy")
	}
}

// showErrorModal displays an error in a modal dialog
func (v *View) showErrorModal(title, message string) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("%s\n\n%s", title, message)).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			v.manager.Pages().RemovePage("error")
		})

	v.manager.Pages().AddPage("error", modal, true, true)
	v.manager.App().SetFocus(modal)
}

// showSecretDetailsModal displays secret details in a clean modal
func (v *View) showSecretDetailsModal(secret *vault.Secret) {
	// Create a table to display the secret data
	table := tview.NewTable()
	table.SetBorder(true).
		SetTitle(fmt.Sprintf(" Secret: %s ", secret.Path)).
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorMediumTurquoise)

	// Add header
	table.SetCell(0, 0, tview.NewTableCell("Key").
		SetTextColor(tcell.ColorYellow).
		SetAlign(tview.AlignCenter).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Value").
		SetTextColor(tcell.ColorYellow).
		SetAlign(tview.AlignCenter).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false))

	// Add secret data rows
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for i, key := range keys {
		value := fmt.Sprintf("%v", secret.Data[key])

		keyCell := tview.NewTableCell(key)
		keyCell.SetTextColor(tcell.ColorMediumTurquoise).
			SetAlign(tview.AlignLeft).
			SetSelectable(true)

		valueCell := tview.NewTableCell(value)
		valueCell.SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectable(true)

		table.SetCell(i+1, 0, keyCell)
		table.SetCell(i+1, 1, valueCell)
	}

	table.SetFixed(1, 0)
	table.SetSelectable(true, false)
	if len(keys) > 0 {
		table.Select(1, 0)
	}

	// Add input capture for copying values and closing
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			v.manager.Pages().RemovePage("secretModal")
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'y' {
				row, col := table.GetSelection()
				if row > 0 {
					cell := table.GetCell(row, col)
					if cell != nil {
						if err := v.copyToClipboard(cell.Text); err != nil {
							v.manager.UpdateStatusBar("Failed to copy to clipboard")
						} else {
							v.manager.UpdateStatusBar(fmt.Sprintf("Copied \"%s\" to clipboard", cell.Text))
						}
					}
				}
				return nil
			}
		}
		return event
	})

	// Create the modal container
	modal := tview.NewFlex().SetDirection(tview.FlexRow)

	// Add navigation instructions
	instructions := tview.NewTextView().
		SetText("↑↓ Navigate • y Copy • Tab Switch panels • / Filter • ? Help • Esc Close").
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tcell.ColorGray)

	modal.AddItem(instructions, 1, 0, false)
	modal.AddItem(table, 0, 1, true)

	// Add metadata info if available
	if secret.Metadata != nil {
		metaText := fmt.Sprintf("Version: %d | Created: %s",
			secret.Metadata.Version,
			secret.Metadata.CreatedTime.Format("2006-01-02 15:04:05"))
		metaInfo := tview.NewTextView().
			SetText(metaText).
			SetTextAlign(tview.AlignCenter).
			SetTextColor(tcell.ColorDarkCyan)
		modal.AddItem(metaInfo, 1, 0, false)
	}

	// Show the modal
	v.manager.Pages().AddPage("secretModal", modal, true, true)
	v.manager.App().SetFocus(table)
}

// showHelpModal displays keyboard shortcuts help
func (v *View) showHelpModal() {
	helpText := `Vault View - Keyboard Shortcuts

Navigation:
  Tab        - Switch between panels
  Enter      - Select mount or view secret
  Esc        - Return to previous panel

Breadcrumb Navigation:
  1-5        - Quick switch to views (Overview/Secrets/Metadata/Paths/History)
  ← →        - Navigate between views when breadcrumbs visible
  Arrow keys - Navigate breadcrumb tabs when focused on breadcrumb bar

Search & Filter:
  /          - Filter current panel
  Backspace  - Clear filter

Data Operations:
  y          - Copy selected item to clipboard
  r          - Toggle row numbers
  n/p        - Next/Previous page

Help:
  ? or h     - Show this help
  Esc        - Close help`

	modal := tview.NewModal().
		SetText(helpText).
		AddButtons([]string{"Close"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			v.manager.Pages().RemovePage("help")
		})

	v.manager.Pages().AddPage("help", modal, true, true)
	v.manager.App().SetFocus(modal)
}

// View functions for breadcrumb navigation
func (v *View) showOverviewView() {
	v.dataTable.Clear()

	// Add overview header
	overviewData := [][]string{
		{"Property", "Value"},
		{"Current Path", v.state.currentPath},
		{"Mount Type", "KV Store"},
		{"Status", "Active"},
		{"Last Updated", "Now"},
	}

	for row, data := range overviewData {
		for col, value := range data {
			cell := tview.NewTableCell(value)
			if row == 0 {
				cell.SetTextColor(style.GruvboxMaterial.Yellow).
					SetAlign(tview.AlignCenter).
					SetSelectable(false).
					SetAttributes(tcell.AttrBold)
			} else {
				if col == 0 {
					cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
				} else {
					cell.SetTextColor(tcell.ColorLightGreen).SetAlign(tview.AlignLeft).SetSelectable(true)
				}
			}
			v.dataTable.SetCell(row, col, cell)
		}
	}

	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) showSecretsView() {
	v.dataTable.Clear()

	// Show the current secret's data when we have a specific secret loaded
	if v.state.currentSecret != nil {
		// Create a table showing the secret data
		headers := []string{"Key", "Value"}
		for col, value := range headers {
			cell := tview.NewTableCell(value)
			cell.SetTextColor(style.GruvboxMaterial.Yellow).
				SetAlign(tview.AlignCenter).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold)
			v.dataTable.SetCell(0, col, cell)
		}

		// Get sorted keys
		keys := make([]string, 0, len(v.state.currentSecret.Data))
		for key := range v.state.currentSecret.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Add secret data rows
		for i, key := range keys {
			value := fmt.Sprintf("%v", v.state.currentSecret.Data[key])

			keyCell := tview.NewTableCell(key)
			keyCell.SetTextColor(tcell.ColorMediumTurquoise).
				SetAlign(tview.AlignLeft).
				SetSelectable(true)

			valueCell := tview.NewTableCell(value)
			valueCell.SetTextColor(tcell.ColorBeige).
				SetAlign(tview.AlignLeft).
				SetSelectable(true)

			v.dataTable.SetCell(i+1, 0, keyCell)
			v.dataTable.SetCell(i+1, 1, valueCell)
		}

		v.dataTable.SetFixed(1, 0)
		if len(keys) > 0 {
			v.dataTable.Select(1, 0)
		}
	} else {
		// If no secret loaded, show a message
		v.dataTable.SetCell(0, 0,
			tview.NewTableCell("No secret data available. Select a secret to view its contents.").
				SetTextColor(tcell.ColorBeige).
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
	}
}

func (v *View) showMetadataView() {
	v.dataTable.Clear()

	// Add metadata header
	metadataData := [][]string{
		{"Metadata Type", "Value"},
		{"Path", v.state.currentPath},
		{"Engine Version", "KV Version 2"},
		{"Max Versions", "10"},
		{"CAS Required", "false"},
		{"Delete Version After", "0s"},
	}

	for row, data := range metadataData {
		for col, value := range data {
			cell := tview.NewTableCell(value)
			if row == 0 {
				cell.SetTextColor(style.GruvboxMaterial.Yellow).
					SetAlign(tview.AlignCenter).
					SetSelectable(false).
					SetAttributes(tcell.AttrBold)
			} else {
				if col == 0 {
					cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
				} else {
					cell.SetTextColor(tcell.ColorLightCyan).SetAlign(tview.AlignLeft).SetSelectable(true)
				}
			}
			v.dataTable.SetCell(row, col, cell)
		}
	}

	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) showPathsView() {
	v.dataTable.Clear()

	// Show path navigation history
	pathsData := [][]string{
		{"Path", "Type", "Last Accessed"},
		{v.state.currentPath, "Current", "Now"},
		{"secret/", "Mount", "Recently"},
		{"secret/myapp/", "Directory", "Recently"},
		{"secret/configs/", "Directory", "Recently"},
	}

	for row, data := range pathsData {
		for col, value := range data {
			cell := tview.NewTableCell(value)
			if row == 0 {
				cell.SetTextColor(style.GruvboxMaterial.Yellow).
					SetAlign(tview.AlignCenter).
					SetSelectable(false).
					SetAttributes(tcell.AttrBold)
			} else {
				switch col {
				case 0:
					cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
				case 1:
					if value == "Current" {
						cell.SetTextColor(style.GruvboxMaterial.Green).SetAlign(tview.AlignLeft).SetSelectable(true)
					} else {
						cell.SetTextColor(style.GruvboxMaterial.Blue).SetAlign(tview.AlignLeft).SetSelectable(true)
					}
				case 2:
					cell.SetTextColor(tcell.ColorGray).SetAlign(tview.AlignLeft).SetSelectable(true)
				}
			}
			v.dataTable.SetCell(row, col, cell)
		}
	}

	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}

func (v *View) showHistoryView() {
	v.dataTable.Clear()

	// Show version history for the current path
	historyData := [][]string{
		{"Version", "Created", "Operation", "User"},
		{"3", "2024-01-15 10:30", "UPDATE", "admin"},
		{"2", "2024-01-10 14:20", "UPDATE", "user1"},
		{"1", "2024-01-05 09:15", "CREATE", "admin"},
	}

	for row, data := range historyData {
		for col, value := range data {
			cell := tview.NewTableCell(value)
			if row == 0 {
				cell.SetTextColor(style.GruvboxMaterial.Yellow).
					SetAlign(tview.AlignCenter).
					SetSelectable(false).
					SetAttributes(tcell.AttrBold)
			} else {
				switch col {
				case 0:
					cell.SetTextColor(tcell.ColorLightYellow).SetAlign(tview.AlignCenter).SetSelectable(true)
				case 1:
					cell.SetTextColor(tcell.ColorBeige).SetAlign(tview.AlignLeft).SetSelectable(true)
				case 2:
					if value == "CREATE" {
						cell.SetTextColor(style.GruvboxMaterial.Green).SetAlign(tview.AlignCenter).SetSelectable(true)
					} else {
						cell.SetTextColor(style.GruvboxMaterial.Blue).SetAlign(tview.AlignCenter).SetSelectable(true)
					}
				case 3:
					cell.SetTextColor(tcell.ColorGray).SetAlign(tview.AlignLeft).SetSelectable(true)
				}
			}
			v.dataTable.SetCell(row, col, cell)
		}
	}

	v.dataTable.SetFixed(1, 0)
	v.dataTable.Select(1, 0)
}
