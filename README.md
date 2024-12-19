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