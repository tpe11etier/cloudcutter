# Elasticsearch Terminal Viewer

A terminal-based Elasticsearch viewer built with tview, providing an intuitive interface for querying and viewing Elasticsearch data.

## Architecture Overview

### Core Components

#### View (Controller)
The View acts as the main controller for the Elasticsearch interface. It:
- Manages application state
- Handles user input
- Coordinates data fetching and display
- Contains references to all UI components

```go
type View struct {
    manager    *manager.Manager     // UI/App management
    components viewComponents       // UI component references
    service    *elastic.Service     // ES client
    state      viewState           // Application state
}
```

#### Pages (Screen Container)
Pages serves as a container that can hold multiple screens/views:
- Currently contains single "elastic" page
- Provides infrastructure for future screen additions
- Manages screen transitions (if needed)

```go
v.components.pages = tview.NewPages()
v.components.pages.AddPage("elastic", v.components.content, true, true)
```

#### Layout (Component Organization)
The layout system uses tview's Flex containers to organize components:
- Defines component positioning and sizing
- Handles responsive behavior
- Manages component hierarchy

```go
// Example layout configuration
cfg := manager.LayoutConfig{
    Direction: tview.FlexRow,
    Components: []manager.Component{
        // Control Panel
        {
            Type: manager.ComponentFlex,
            Direction: tview.FlexRow,
            FixedSize: 15,
            Children: [...],
        },
        // Main Content
        {
            Type: manager.ComponentFlex,
            Direction: tview.FlexColumn,
            Proportion: 1,
            Children: [...],
        },
    }
}
```

### Layout Structure

```
┌─────────────────────────────────────────────┐
│ ES Filter Input                             │ 
├─────────────────────────────────────────────┤
│ Local Filter Input                          │
├─────────────────────────────────────────────┤
│ Active Filters                              │
├─────────────────────────────────────────────┤
│ Index Selector                              │
├─────────────────────────────────────────────┤
│ Selected Fields                             │
├─────────────────────────────────────────────┤
│                │                            │
│ Field List     │      Results Table         │
│                │                            │
│                │                            │
└────────────────┴────────────────────────────┘
```

### Layout Organization

#### FlexRow vs FlexColumn
The layout uses two types of Flex containers:

1. **FlexRow** (Vertical Stacking):
    - Components stack top-to-bottom
    - FixedSize controls height
    - Used for control panel section

2. **FlexColumn** (Horizontal Arrangement):
    - Components arrange left-to-right
    - FixedSize controls width
    - Used for main content area (fields/results)

## Component Reference

### Control Panel Components
- **ES Filter Input**: Primary query input
- **Local Filter Input**: Results filtering
- **Active Filters**: Shows current filters
- **Index Selector**: ES index selection
- **Selected Fields**: Displays active fields

### Main Content Components
- **Field List**: Available ES fields
- **Results Table**: Query results display

## Navigation

Key bindings:
- `Tab`: Cycle through components
- `Esc`: Return to main filter
- `~`: Open index selector
- `Enter`: Confirm selection/input
