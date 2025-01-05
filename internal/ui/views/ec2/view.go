package ec2

//
//import (
//	"context"
//	"fmt"
//	"github.com/aws/aws-sdk-go-v2/aws"
//	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
//	"github.com/gdamore/tcell/v2"
//	"github.com/rivo/tview"
//	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/ec2"
//	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/header"
//	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
//)
//
//type View struct {
//	name        string
//	manager     *manager.Manager
//	ec2Service  *ec2.Service
//	instanceMap map[string]*ec2types.Instance
//	pages       *tview.pages
//	leftPanel   *tview.List
//	dataTable   *tview.Table
//}
//
//func NewView(manager *manager.Manager, ec2Service *ec2.Service) *View {
//	view := &View{
//		name:        "ec2",
//		manager:     manager,
//		ec2Service:  ec2Service,
//		instanceMap: make(map[string]*ec2types.Instance),
//		pages:       tview.NewPages(),
//	}
//	view.setupLayout()
//	return view
//}
//
//func (v *View) setupLayout() tview.Primitive {
//	cfg := manager.LayoutConfig{
//		Direction: tview.FlexColumn,
//		Components: []manager.Component{
//			{
//				ID:        "leftPanel",
//				Type:      manager.ComponentList,
//				FixedSize: 40,
//				Focus:     true,
//				Style: manager.Style{
//					Border:      true,
//					Title:       " EC2 ",
//					TitleAlign:  tview.AlignCenter,
//					TitleColor:  tcell.ColorMediumTurquoise,
//					BorderColor: tcell.ColorMediumTurquoise,
//				},
//				Properties: map[string]any{
//					"onFocus": func(list *tview.List) {
//						list.SetBorderColor(tcell.ColorMediumTurquoise)
//					},
//					"onBlur": func(list *tview.List) {
//						list.SetBorderColor(tcell.ColorBeige)
//					},
//					"onChanged": func(index int, mainText string, secondaryText string, shortcut rune) {
//						instance, exists := v.instanceMap[mainText]
//						if !exists || instance == nil {
//							v.manager.updateHeader(nil)
//							if table := v.manager.GetPrimitiveByID("dataTable").(*tview.Table); table != nil {
//								table.Clear()
//							}
//							return
//						}
//
//						v.updateEC2Summary(instance)
//						v.updateDataTableForInstance(instance)
//					},
//				},
//			},
//			{
//				ID:         "dataTable",
//				Type:       manager.ComponentTable,
//				Proportion: 1,
//				Style: manager.Style{
//					Border:      true,
//					BorderColor: tcell.ColorBeige,
//				},
//				Properties: map[string]any{
//					"onFocus": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorMediumTurquoise)
//					},
//					"onBlur": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorBeige)
//					},
//				},
//			},
//		},
//	}
//
//	layout := v.manager.CreateLayout(cfg)
//	v.leftPanel = v.manager.GetPrimitiveByID("leftPanel").(*tview.List)
//	v.dataTable = v.manager.GetPrimitiveByID("dataTable").(*tview.Table)
//	v.dataTable.SetSelectable(true, false)
//	v.pages.AddPage("ec2", layout, true, true)
//
//	return layout
//}
//
//func (v *View) updateDataTableForInstance(instance *ec2types.Instance) {
//	if instance == nil {
//		return
//	}
//
//	table := v.dataTable
//	table.Clear()
//
//	// Set header row
//	table.SetCell(0, 0, tview.NewTableCell("Property").
//		SetTextColor(style.GruvboxMaterial.Yellow).
//		SetAlign(tview.AlignLeft).
//		SetSelectable(false).
//		SetAttributes(tcell.AttrBold))
//	table.SetCell(0, 1, tview.NewTableCell("Value").
//		SetTextColor(style.GruvboxMaterial.Yellow).
//		SetAlign(tview.AlignLeft).
//		SetSelectable(false).
//		SetAttributes(tcell.AttrBold))
//
//	// Add data rows
//	rows := [][]string{
//		{"Instance ID", aws.ToString(instance.InstanceId)},
//		{"Instance State", string(instance.State.Name)},
//		{"Instance Type", string(instance.InstanceType)},
//		{"Availability Zone", aws.ToString(instance.Placement.AvailabilityZone)},
//		{"Public IPv4 Address", aws.ToString(instance.PublicIpAddress)},
//		{"Launch Time", instance.LaunchTime.Format("2006-01-02 15:04:05")},
//	}
//
//	for i, row := range rows {
//		table.SetCell(i+1, 0, tview.NewTableCell(row[0]).
//			SetTextColor(tcell.ColorBeige).
//			SetAlign(tview.AlignLeft))
//		table.SetCell(i+1, 1, tview.NewTableCell(row[1]).
//			SetTextColor(tcell.ColorBeige).
//			SetAlign(tview.AlignLeft))
//	}
//
//	table.SetFixed(1, 0) // Fix header row
//	table.Select(1, 0)   // Select first data row
//
//	v.manager.UpdateStatusBar(fmt.Sprintf("Showing details for instance %s", aws.ToString(instance.InstanceId)))
//}
//
//func (v *View) ActiveField() string {
//	currentFocus := v.manager.App.GetFocus()
//	switch currentFocus {
//	case v.leftPanel:
//		return "leftPanel"
//	case v.dataTable:
//		return "dataTable"
//	default:
//		return ""
//	}
//}
//
//func (v *View) Name() string {
//	return v.name
//}
//
//func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
//	return func(event *tcell.EventKey) *tcell.EventKey {
//		switch event.Key() {
//		case tcell.KeyEsc:
//			v.manager.SetFocus(v.leftPanel)
//			return nil
//		case tcell.KeyTab:
//			if v.leftPanel.HasFocus() {
//				v.manager.SetFocus(v.dataTable)
//			} else {
//				v.manager.SetFocus(v.leftPanel)
//			}
//			return nil
//		}
//		return event
//	}
//}
//
//func (v *View) Render() {
//	v.fetchEC2Instances()
//}
//
//func (v *View) Content() tview.Primitive {
//	return v.pages
//}
//
//func (v *View) Hide() {}
//
//func (v *View) fetchEC2Instances() {
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	instances, err := v.ec2Service.FetchInstances(ctx)
//	if err != nil {
//		v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching EC2 instances: %v", err))
//		return
//	}
//
//	v.leftPanel.Clear()
//	v.instanceMap = make(map[string]*ec2types.Instance)
//
//	if len(instances) == 0 {
//		v.leftPanel.AddItem("No instances found", "", 0, nil)
//		v.manager.UpdateStatusBar("No EC2 instances available")
//		return
//	}
//
//	for _, instance := range instances {
//		displayText := aws.ToString(getInstanceName(instance))
//		v.leftPanel.AddItem(displayText, "", 0, nil)
//		v.instanceMap[displayText] = instance
//	}
//
//	v.manager.UpdateStatusBar("Select an instance to view details")
//}
//
//func (v *View) updateEC2Summary(instance *ec2types.Instance) {
//	if instance == nil {
//		v.manager.updateHeader(nil)
//		return
//	}
//
//	summary := []header.SummaryItem{
//		{Key: "Instance ID", Value: aws.ToString(instance.InstanceId)},
//		{Key: "Instance Name", Value: aws.ToString(getInstanceName(instance))},
//		{Key: "Instance State", Value: string(instance.State.Name)},
//		{Key: "Instance Type", Value: string(instance.InstanceType)},
//		{Key: "Public IPv4", Value: aws.ToString(instance.PublicIpAddress)},
//		{Key: "Launch Time", Value: instance.LaunchTime.Format("2006-01-02 15:04:05")},
//	}
//	v.manager.updateHeader(summary)
//}
//
//func getInstanceName(instance *ec2types.Instance) *string {
//	for _, tag := range instance.Tags {
//		if aws.ToString(tag.Key) == "Name" {
//			return tag.Value
//		}
//	}
//	return nil
//}
