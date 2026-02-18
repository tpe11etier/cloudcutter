package elastic

import (
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/ui/common"
)

type ElasticComponentType = common.ComponentType

const (
	FilterInputComponent ElasticComponentType = iota
	ActiveFiltersComponent
	IndexInputComponent
	FieldListComponent
	SelectedListComponent
	ResultsTableComponent
	TimeframeInputComponent
	NumResultsInputComponent
	LocalFilterInputComponent
)

type ElasticComponentMapper struct {
	view *View
}

func NewElasticComponentMapper(view *View) *ElasticComponentMapper {
	return &ElasticComponentMapper{view: view}
}

func (m *ElasticComponentMapper) GetComponentType(primitive tview.Primitive) *common.ComponentType {
	switch primitive {
	case m.view.components.filterInput:
		return &[]common.ComponentType{FilterInputComponent}[0]
	case m.view.components.activeFilters:
		return &[]common.ComponentType{ActiveFiltersComponent}[0]
	case m.view.components.indexInput:
		return &[]common.ComponentType{IndexInputComponent}[0]
	case m.view.components.fieldList:
		return &[]common.ComponentType{FieldListComponent}[0]
	case m.view.components.selectedList:
		return &[]common.ComponentType{SelectedListComponent}[0]
	case m.view.components.resultsTable:
		return &[]common.ComponentType{ResultsTableComponent}[0]
	case m.view.components.timeframeInput:
		return &[]common.ComponentType{TimeframeInputComponent}[0]
	case m.view.components.numResultsInput:
		return &[]common.ComponentType{NumResultsInputComponent}[0]
	case m.view.components.localFilterInput:
		return &[]common.ComponentType{LocalFilterInputComponent}[0]
	}
	return nil
}

func (m *ElasticComponentMapper) GetComponents() map[common.ComponentType]tview.Primitive {
	return map[common.ComponentType]tview.Primitive{
		FilterInputComponent:      m.view.components.filterInput,
		ActiveFiltersComponent:    m.view.components.activeFilters,
		IndexInputComponent:       m.view.components.indexInput,
		FieldListComponent:        m.view.components.fieldList,
		SelectedListComponent:     m.view.components.selectedList,
		ResultsTableComponent:     m.view.components.resultsTable,
		TimeframeInputComponent:   m.view.components.timeframeInput,
		NumResultsInputComponent:  m.view.components.numResultsInput,
		LocalFilterInputComponent: m.view.components.localFilterInput,
	}
}

func (m *ElasticComponentMapper) GetNavigationOrder() []tview.Primitive {
	return []tview.Primitive{
		m.view.components.indexInput,
		m.view.components.filterInput,
		m.view.components.resultsTable,
		m.view.components.fieldList,
		m.view.components.selectedList,
	}
}

func GetComponentName(componentType common.ComponentType) string {
	switch componentType {
	case FilterInputComponent:
		return "FilterInput"
	case ActiveFiltersComponent:
		return "ActiveFilters"
	case IndexInputComponent:
		return "IndexInput"
	case FieldListComponent:
		return "FieldList"
	case SelectedListComponent:
		return "SelectedList"
	case ResultsTableComponent:
		return "ResultsTable"
	case TimeframeInputComponent:
		return "TimeframeInput"
	case NumResultsInputComponent:
		return "NumResultsInput"
	case LocalFilterInputComponent:
		return "LocalFilterInput"
	default:
		return "Unknown"
	}
}
