package ec2

import (
	"context"
	"fmt"
	"github.com/tpelletiersophos/ahtui/internal/services/aws/ec2"
	"github.com/tpelletiersophos/ahtui/internal/services/manager"
	"github.com/tpelletiersophos/ahtui/ui/components"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type View struct {
	name           string
	manager        *manager.Manager
	ec2Service     *ec2.Service
	contentFlex    *tview.Flex
	instanceMap    map[string]*ec2types.Instance
	LeftPanel      *components.LeftPanel
	DataTable      *components.DataTable
	currentContext context.Context
}

func NewView(manager *manager.Manager, ec2Service *ec2.Service) *View {
	view := &View{
		name:        "ec2",
		manager:     manager,
		ec2Service:  ec2Service,
		contentFlex: tview.NewFlex(),
		instanceMap: make(map[string]*ec2types.Instance),
	}

	view.setupLayout()
	return view
}

func (v *View) Name() string {
	return v.name
}

func (v *View) GetContent() tview.Primitive {
	return v.contentFlex
}

func (v *View) Show() {
	v.fetchEC2Instances()
}

func (v *View) Hide() {
	// Implement any necessary cleanup
}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		// Handle view-specific key events
		return event
	}
}

func (v *View) setupLayout() {
	v.DataTable = components.NewDataTable()
	v.DataTable.SetBorder(true).
		SetTitle(" EC2 Instance Details ").
		SetTitleColor(tcell.ColorTeal)

	v.DataTable.Setup(
		[]string{"Property", "Value"},
		[]int{
			tview.AlignLeft,
			tview.AlignLeft,
		},
	)

	v.LeftPanel = components.NewLeftPanel()
	v.LeftPanel.SetBorder(true).
		SetTitle(" EC2 Instances ").
		SetTitleColor(tcell.ColorTeal)

	v.contentFlex = tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(v.LeftPanel, 40, 0, true).
		AddItem(v.DataTable, 0, 2, false)

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

	v.LeftPanel.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		instance, exists := v.instanceMap[mainText]
		if !exists || instance == nil {
			v.manager.UpdateHeader(nil)
			v.DataTable.Clear()
			return
		}
		v.updateEC2Summary(instance)
		v.updateDataTableForInstance(instance)
	})
}

func (v *View) fetchEC2Instances() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instances, err := v.ec2Service.FetchInstances(ctx)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching EC2 instances: %v", err))
		return
	}

	v.LeftPanel.Clear()
	v.instanceMap = make(map[string]*ec2types.Instance)

	for _, instance := range instances {
		displayText := aws.ToString(instance.InstanceId)
		v.LeftPanel.AddItem(displayText, "", 0, nil)
		v.instanceMap[displayText] = instance
	}

	v.manager.UpdateStatusBar("Select an instance to view details")
}

func (v *View) updateDataTableForInstance(instance *ec2types.Instance) {
	if instance == nil {
		v.DataTable.Clear()
		return
	}

	v.DataTable.Clear()

	addRow := func(label, value string) {
		v.DataTable.AddRow([]string{label, value})
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
