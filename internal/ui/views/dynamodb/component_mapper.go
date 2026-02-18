package dynamodb

import (
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/ui/common"
)

type DynamoDBComponentType = common.ComponentType

const (
	LeftPanelComponent DynamoDBComponentType = iota
	DataTableComponent
	FilterPromptComponent
)

type DynamoDBComponentMapper struct {
	view *View
}

func NewDynamoDBComponentMapper(view *View) *DynamoDBComponentMapper {
	return &DynamoDBComponentMapper{view: view}
}

func (m *DynamoDBComponentMapper) GetComponentType(primitive tview.Primitive) *common.ComponentType {
	switch primitive {
	case m.view.leftPanel:
		return &[]common.ComponentType{LeftPanelComponent}[0]
	case m.view.dataTable:
		return &[]common.ComponentType{DataTableComponent}[0]
	case m.view.filterPrompt.InputField:
		return &[]common.ComponentType{FilterPromptComponent}[0]
	}
	return nil
}

func (m *DynamoDBComponentMapper) GetComponents() map[common.ComponentType]tview.Primitive {
	return map[common.ComponentType]tview.Primitive{
		LeftPanelComponent:    m.view.leftPanel,
		DataTableComponent:    m.view.dataTable,
		FilterPromptComponent: m.view.filterPrompt.InputField,
	}
}

func (m *DynamoDBComponentMapper) GetNavigationOrder() []tview.Primitive {
	return []tview.Primitive{
		m.view.leftPanel,
		m.view.dataTable,
	}
}
