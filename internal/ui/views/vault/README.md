# Vault View

The Vault view provides a terminal-based interface for interacting with HashiCorp Vault. It allows you to browse mounted secret engines, view secrets, and copy values to the clipboard.

## Features

- **Mount Browser**: View all mounted secret engines in your Vault instance
- **Secret Listing**: Browse secrets within each mount
- **Secret Details**: View detailed information about individual secrets
- **Filtering**: Filter mounts and secrets by name or content
- **Pagination**: Navigate through large lists of secrets
- **Clipboard Integration**: Copy secret values to clipboard with 'y' key
- **Row Numbers**: Toggle row numbers on/off with 'r' key

## Configuration

The Vault view requires the following environment variables:

- `VAULT_TOKEN`: Your Vault authentication token
- `VAULT_ADDR`: Vault server address (defaults to `http://localhost:8200`)

## Usage

1. Start the application: `./build/cloudcutter`
2. Type `:vault` to switch to the Vault view
3. Use the left panel to browse mounted secret engines
4. Press Enter on a mount to view its secrets
5. Use Tab to switch between panels
6. Press Enter on a secret to view its details
7. Use 'y' to copy secret values to clipboard
8. Use '/' to filter content
9. Use 'n'/'p' for pagination
10. Use 'r' to toggle row numbers

## Keyboard Shortcuts

- `Tab`: Switch between left panel (mounts) and right panel (secrets)
- `Enter`: Select mount or view secret details
- `/`: Open filter prompt
- `n`: Next page
- `p`: Previous page
- `r`: Toggle row numbers
- `y`: Copy selected value to clipboard (in secret details modal)
- `Esc`: Close modals or return to previous panel

## Architecture

The Vault view follows the same pattern as other views in the application:

- **Service Layer**: `internal/services/vault/vault.go` - Handles Vault API communication
- **View Layer**: `internal/ui/views/vault/view.go` - Manages UI components and user interactions
- **Manager Integration**: Registered as a lazy view in the main application

The view supports the same features as the DynamoDB view including filtering, pagination, and modal dialogs for detailed views.
