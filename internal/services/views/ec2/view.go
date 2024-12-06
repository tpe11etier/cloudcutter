package ec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/ec2"
	"github.com/tpelletiersophos/cloudcutter/internal/services/manager"
	"github.com/tpelletiersophos/cloudcutter/ui/components"
)

type View struct {
	name        string
	manager     *manager.Manager
	ec2Service  *ec2.Service
	instanceMap map[string]*ec2types.Instance
	pages       *tview.Pages
	leftPanel   *components.LeftPanel
	dataTable   *components.DataTable
}

func NewView(manager *manager.Manager, ec2Service *ec2.Service) *View {
	view := &View{
		name:        "ec2",
		manager:     manager,
		ec2Service:  ec2Service,
		instanceMap: make(map[string]*ec2types.Instance),
		pages:       tview.NewPages(),
	}
	view.setupLayouts()
	return view
}

func (v *View) setupLayouts() {
	mainLayout := v.createMainLayout()
	v.pages.AddPage("main", mainLayout, true, true)
}

func (v *View) createMainLayout() *tview.Flex {
	v.leftPanel = components.NewLeftPanel()
	v.dataTable = components.NewDataTable()

	// Configure the left panel
	v.leftPanel.SetBorder(true).
		SetTitle(" EC2 ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise)

	// Configure the data table
	v.dataTable.SetBorder(true).
		SetTitle(" EC2 Instance Details ").
		SetTitleColor(tcell.ColorTeal)

	// Set up the data table with headers
	v.dataTable.Setup(
		[]string{"Property", "Value"},
		[]int{tview.AlignLeft, tview.AlignLeft},
	)

	// Set up the event handler for when the selection changes in the left panel
	v.leftPanel.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		instance, exists := v.instanceMap[mainText]
		if !exists || instance == nil {
			v.manager.UpdateHeader(nil)
			v.dataTable.Clear()
			return
		}
		v.updateEC2Summary(instance)
		v.updateDataTableForInstance(instance)
	})

	// Set up component styles (selection, focus)
	v.setupComponentStyles()

	// Create and return the main layout using a Flex container
	return tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(v.leftPanel, 40, 0, true).
		AddItem(v.dataTable, 0, 1, false) // Adjusted proportion to 1 for consistency
}

func (v *View) setupComponentStyles() {
	selectedStyle := tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorDarkCyan).
		Attributes(tcell.AttrBold)

	v.leftPanel.SetSelectedStyle(selectedStyle)
	v.dataTable.SetSelectedStyle(selectedStyle)

	v.leftPanel.SetFocusFunc(func() { v.leftPanel.SetBorderColor(tcell.ColorMediumTurquoise) })
	v.leftPanel.SetBlurFunc(func() { v.leftPanel.SetBorderColor(tcell.ColorGray) })
	v.dataTable.SetFocusFunc(func() { v.dataTable.SetBorderColor(tcell.ColorMediumTurquoise) })
	v.dataTable.SetBlurFunc(func() { v.dataTable.SetBorderColor(tcell.ColorGray) })
}

func (v *View) Name() string                { return v.name }
func (v *View) GetContent() tview.Primitive { return v.pages }
func (v *View) Hide()                       {}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			v.manager.SetFocus(v.leftPanel)
			return nil
		}
		return event
	}
}

func (v *View) Show() {
	v.fetchEC2Instances()
}

func (v *View) fetchEC2Instances() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instances, err := v.ec2Service.FetchInstances(ctx)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching EC2 instances: %v", err))
		return
	}

	v.leftPanel.Clear()
	v.instanceMap = make(map[string]*ec2types.Instance)

	if len(instances) == 0 {
		v.leftPanel.AddItem("No instances found", "", 0, nil)
		v.manager.UpdateStatusBar("No EC2 instances available")
		return
	}

	for _, instance := range instances {
		displayText := aws.ToString(instance.InstanceId)
		v.leftPanel.AddItem(displayText, "", 0, nil)
		v.instanceMap[displayText] = instance
	}

	v.manager.UpdateStatusBar("Select an instance to view details")
}

func (v *View) updateDataTableForInstance(instance *ec2types.Instance) {
	if instance == nil {
		v.dataTable.Clear()
		return
	}

	v.dataTable.Clear()

	addRow := func(label, value string) {
		v.dataTable.AddRow([]string{label, value})
	}

	addRow("Instance ID", aws.ToString(instance.InstanceId))
	addRow("Instance State", string(instance.State.Name))
	addRow("Instance Type", string(instance.InstanceType))
	addRow("Availability Zone", aws.ToString(instance.Placement.AvailabilityZone))
	addRow("Public IPv4 Address", aws.ToString(instance.PublicIpAddress))
	addRow("Launch Time", instance.LaunchTime.Format("2006-01-02 15:04:05"))

	v.manager.UpdateStatusBar(fmt.Sprintf("Showing details for instance %s", aws.ToString(instance.InstanceId)))
}

func (v *View) updateEC2Summary(instance *ec2types.Instance) {
	if instance == nil {
		v.manager.UpdateHeader(nil)
		return
	}

	summary := []components.SummaryItem{
		{Key: "Instance ID", Value: aws.ToString(instance.InstanceId)},
		{Key: "Instance State", Value: string(instance.State.Name)},
		{Key: "Instance Type", Value: string(instance.InstanceType)},
		{Key: "Public IPv4", Value: aws.ToString(instance.PublicIpAddress)},
		{Key: "Launch Time", Value: instance.LaunchTime.Format("2006-01-02 15:04:05")},
	}
	v.manager.UpdateHeader(summary)
}
