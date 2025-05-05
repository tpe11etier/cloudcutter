# CloudCutter

A terminal-based UI application for interacting with and managing cloud services, featuring a clean interface for AWS services and Elasticsearch.

## Features

### Multi-Service Support
- **AWS DynamoDB**
    - Browse and query DynamoDB tables
- **Elasticsearch**
    - View and analyze logs
- **Extensible Architecture**
    - Easily add support for additional services

### Terminal UI Features
- **Split Panel Views**
    - Efficient navigation between different sections
- **Real-Time Filtering and Search**
    - Quickly find the data you need
- **Keyboard-Driven Interface**
    - Navigate without leaving the keyboard
- **Color-Coded Elements**
    - Enhance readability and usability
- **Status Bar**
    - Provides feedback and essential information
- **Command Mode**
    - Execute quick navigation and commands

## Screenshots

![CloudCutter](./screenshots/screenshot.png)

## Authentication Setup

### Prerequisites

1. **AWS Credentials**
- **Configuration**: Set up standard AWS credentials in `~/.aws/credentials` and `~/.aws/config`
- **Supported Profile Types**:
    - **Standard AWS Profiles**: Any profiles defined in your AWS configuration
    - **Opal-Managed Profiles**: Profiles that match configured Opal environment patterns
    - **Local Development Profile**: For local development environments

2. **Opal CLI**
- **Purpose**: Required for using Opal-managed profiles
- **Installation & Configuration**:
    - Ensure the Opal CLI is installed on your system
    - Configure the Opal CLI
    - Run `opal login` to authenticate before first use

---

**Important:** If you encounter any issues with Opal authentication, please refer to the [AWS Opal Authentication Guide](https://sophos.atlassian.net/wiki/spaces/MDR/pages/226671693067/AWS+-+Opal+Authentication#Authenticating-with-AWS-using-Opal) for detailed troubleshooting steps.

### Opal Configuration

CloudCutter supports flexible profile mapping for Opal authentication. You can configure this in two ways:

1. **Environment Variables**:
```bash
export OPAL_DEV_ROLE_ID=********-****-****-****-************
export OPAL_PROD_ROLE_ID=********-****-****-****-************
```

2. **Configuration File** (Optional):
   Create `~/.cloudcutter/opal.json`:
```json
{
    "environments": {
        "dev": {
            "roleId": "********-****-****-****-************",
            "profileTags": ["dev", "development", "opal_dev"]
        },
        "prod": {
            "roleId": "********-****-****-****-************",
            "profileTags": ["prod", "production", "opal_prod"]
        }
    }
}
```

This configuration allows CloudCutter to map AWS profiles to the appropriate Opal roles based on profile name patterns.

### Supported Authentication Methods

1. **Standard AWS Profiles**
    - Uses credentials from AWS credentials and config files
    - Supports default and named profiles
    - Automatically handles credential refresh

2. **Opal Authentication**
    - Automatically detects and maps profiles to Opal environments
    - Handles automatic session management
    - Requires a valid Opal CLI session
    - Maps profiles based on configured patterns (e.g., "dev", "prod" in profile name)

3. **Local Development**
    - **Profile Name**: `local`
    - Uses local endpoints for development
    - Region automatically set to "local"

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

## Troubleshooting

### Authentication Issues

1. **Opal Session Expired**
    - **Error**: "opal session expired"
    - **Solution**:
        1. Run `opal login` in terminal
        2. Retry the operation

2. **Profile Switching**
    - **Issue**: Only one authentication process can run at a time
    - **Solution**: Wait for the current authentication to complete before switching profiles

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement changes with appropriate tests
4. Submit a pull request

## License

MIT License - See [LICENSE](LICENSE) file for details

## Acknowledgments

Built with:

- [tview](https://github.com/rivo/tview) - Terminal UI library
- [tcell](https://github.com/gdamore/tcell) - Terminal handling
- AWS SDK for Go v2
- Elasticsearch Go client