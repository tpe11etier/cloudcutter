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

## Authentication Setup

### Prerequisites

1. **AWS Credentials**
  - **Configuration**: Set up standard AWS credentials in `~/.aws/credentials`.
  - **Supported Profile Types**:
    - **Standard AWS Profiles**: Default and named profiles as defined by AWS.
    - **Opal-Managed Profiles**: `dev` and `prod` profiles managed via Opal.
    - **Local Development Profile**: For local development environments.

2. **Opal CLI**
  - **Purpose**: Required for using Opal-managed profiles.
  - **Installation & Configuration**:
    - Ensure the Opal CLI is installed on your system.
    - Configure the Opal CLI by following the [installation instructions](https://example.com/opal-cli-install).
    - Run `opal login` to authenticate before first use.

---

**Important:** If you encounter any issues with Opal authentication, please refer to the [AWS Opal Authentication Guide](https://sophos.atlassian.net/wiki/spaces/MDR/pages/226671693067/AWS+-+Opal+Authentication#Authenticating-with-AWS-using-Opal) for detailed troubleshooting steps.

### Environment Variables

To enable Opal authentication, set the following environment variables:

1. **`OPAL_DEV_ROLE_ID`**
2. **`OPAL_PROD_ROLE_ID`**

#### Obtaining Your Role ID

The Role ID required for authentication is visible when you log into Opal via your web browser. During the login process, Opal provides a command that includes this Role ID.

**Example Command:**

```bash
opal iam-roles:start \
  --id 123ce456-9c7a-123e-b123-a1bcde123456e
```

#### Setting the Environment Variables

After obtaining your Role ID, set the environment variables in your shell as follows:

```bash
export OPAL_DEV_ROLE_ID=123ce456-9c7a-123e-b123-a1bcde123456e
export OPAL_PROD_ROLE_ID=
```

### Supported Authentication Methods

1. **Standard AWS Profiles**
    - Uses credentials from AWS credentials file
    - Supports default and named profiles
    - Automatically handles credential refresh

2. **Opal Authentication**
    - Supports two predefined profiles: (They **MUST** be named this.)
        - `opal_dev`: Development environment access
        - `opal_prod`: Production environment access
    - Handles automatic session management
    - Requires a valid Opal CLI session

``3. **Local Development**
    - **Profile Name**: `local`
    - Uses local endpoints for development
    - Region automatically set to "local"
``
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

