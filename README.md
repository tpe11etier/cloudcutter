# CloudCutter

A terminal-based UI application for interacting with and managing cloud services, featuring a clean interface for AWS services and Elasticsearch.

## Features

- **Multi-Service Support**
    - AWS DynamoDB browsing and querying
    - Elasticsearch log viewing and analysis
    - Extensible architecture for additional services

- **Terminal UI Features**
    - Split panel views for efficient navigation
    - Real-time filtering and search
    - Keyboard-driven interface
    - Color-coded interface elements
    - Status bar for feedback and information
    - Command mode for quick navigation

## Architecture

### Core Components

#### View System
Each service view implements a common interface:
```go
type View interface {
    Name() string
    GetContent() tview.Primitive
    Show()
    Hide()
    GetActiveField() string
    InputHandler() func(event *tcell.EventKey) *tcell.EventKey
}
```

#### Manager
The manager handles:
- View registration and switching
- Layout creation
- Global input handling
- Modal and prompt management
- Status updates

### Layout System

Uses a flexible configuration-based approach:
```go
type LayoutConfig struct {
    Title       string
    Components  []Component
    Direction   int
    FixedSizes  []int
    Proportions []int
}
```

Supports multiple component types:
- Lists
- Tables
- Text Views
- Input Fields
- Flex Containers

## Service Views

### DynamoDB View
- Table listing and selection
- Item viewing and filtering
- Dynamic attribute handling
- Cached table descriptions

### Elasticsearch View
- Query building and execution
- Field selection and filtering
- Real-time result filtering
- Index selection and management

## Navigation

### Global Keys
- `/`: Open filter prompt
- `:`: Open command prompt
- `ESC`: Close modals/return to main view
- `Tab`: Cycle through components

### View-Specific Keys
Each view implements custom key handlers for specific functionality.

## Development

### Adding a New View

1. Implement the View interface:
```go
type CustomView struct {
    manager    *manager.Manager
    components viewComponents
    state      viewState
}
```

2. Configure layout:
```go
func (v *CustomView) setupLayout() {
    cfg := manager.LayoutConfig{
        Direction: tview.FlexRow,
        Components: []manager.Component{
            // Define components
        },
    }
}
```

3. Register with manager:
```go
manager.RegisterView(newView)
```

### Component Configuration

Components are configured using a declarative style:
```go
{
    ID:        "componentID",
    Type:      manager.ComponentType,
    FixedSize: size,
    Style: manager.Style{
        Border:      true,
        BorderColor: tcell.ColorBeige,
        Title:       "Title",
    },
    Properties: map[string]any{
        // Component properties
    },
}
```

## Building

Requirements:
- Go 1.19 or higher
- Terminal with color support

```bash
go build -o cloudcutter main.go
```

## Running

```bash
./cloudcutter [flags]

Flags:
  --aws-profile string       AWS profile to use
  --elasticsearch-url string Elasticsearch URL
  --index string            Default Elasticsearch index
```

## Project Structure

```
cloudcutter/
├── internal/
│   ├── services/           # Service implementations
│   │   ├── aws/           # AWS service clients
│   │   ├── elastic/       # Elasticsearch client
│   │   └── manager/       # View management
│   └── ui/
│       └── components/    # Reusable UI components
└── ui/                    # UI implementation
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement changes with appropriate tests
4. Submit pull request

## License

MIT License - See LICENSE file for details

## Acknowledgments

Built with:
- [tview](https://github.com/rivo/tview) - Terminal UI library
- [tcell](https://github.com/gdamore/tcell) - Terminal handling
- AWS SDK for Go v2
- Elasticsearch Go client