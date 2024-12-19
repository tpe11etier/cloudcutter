package dynamodb

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"testing"
	"time"
)

type MockDynamoDB struct {
	tables            []string
	tableDescriptions map[string]*types.TableDescription
	items             map[string][]map[string]types.AttributeValue
	forceError        bool
}

func (m *MockDynamoDB) ListTables(ctx context.Context) ([]string, error) {
	if m.forceError {
		return nil, errors.New("forced error")
	}
	return m.tables, nil
}

func (m *MockDynamoDB) DescribeTable(ctx context.Context, tableName string) (*types.TableDescription, error) {
	if m.forceError {
		return nil, errors.New("forced error")
	}
	if desc, ok := m.tableDescriptions[tableName]; ok {
		return desc, nil
	}
	return nil, &types.ResourceNotFoundException{Message: aws.String("Table not found")}
}

func (m *MockDynamoDB) ScanTable(ctx context.Context, tableName string) ([]map[string]types.AttributeValue, error) {
	if m.forceError {
		return nil, errors.New("forced error")
	}
	if items, ok := m.items[tableName]; ok {
		return items, nil
	}
	return nil, &types.ResourceNotFoundException{Message: aws.String("Table not found")}
}

func setupTest() (*View, *MockDynamoDB, *ui.App) {
	app := &ui.App{
		Application: tview.NewApplication(),
	}
	viewManager := manager.NewViewManager(context.Background(), app, aws.Config{Region: "us-west-2"})

	mockService := &MockDynamoDB{
		tables: []string{"Table1", "Table2"},
		tableDescriptions: map[string]*types.TableDescription{
			"Table1": {
				TableName:      aws.String("Table1"),
				TableStatus:    types.TableStatusActive,
				ItemCount:      aws.Int64(100),
				TableSizeBytes: aws.Int64(1024),
			},
			"Table2": {
				TableName:      aws.String("Table2"),
				TableStatus:    types.TableStatusActive,
				ItemCount:      aws.Int64(200),
				TableSizeBytes: aws.Int64(2048),
			},
		},
		items: map[string][]map[string]types.AttributeValue{
			"Table1": {
				{
					"id":   &types.AttributeValueMemberS{Value: "1"},
					"name": &types.AttributeValueMemberS{Value: "Item 1"},
				},
				{
					"id":   &types.AttributeValueMemberS{Value: "2"},
					"name": &types.AttributeValueMemberS{Value: "Item 2"},
				},
			},
		},
	}

	view := NewView(viewManager, mockService)
	view.Show()
	return view, mockService, app
}

func TestView(t *testing.T) {
	t.Run("UI Components", func(t *testing.T) {
		view, _, _ := setupTest()

		done := make(chan bool)
		go func() {
			view.wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			leftPanel := view.leftPanel
			assert.NotNil(t, leftPanel)
			assert.Equal(t, 2, leftPanel.GetItemCount(), "Should show both tables")

			dataTable := view.dataTable
			assert.NotNil(t, dataTable)

			assert.Equal(t, 2, len(view.tableCache), "Table cache should contain both tables")

		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for async operations")
		}
	})

	t.Run("Table Selection", func(t *testing.T) {
		view, _, _ := setupTest()

		// Wait for initial loading
		done := make(chan bool)
		go func() {
			view.wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			// get first table
			view.leftPanel.SetCurrentItem(0)
			mainText, _ := view.leftPanel.GetItemText(0)
			view.fetchTableDetails(mainText)

			// Wait for data table to be populated
			time.Sleep(100 * time.Millisecond)

			// First verify the table exists in cache
			cached := view.tableCache[mainText]
			assert.NotNil(t, cached)
			assert.Equal(t, mainText, *cached.TableName)

			// Then check if items were loaded
			items, err := view.service.ScanTable(context.Background(), mainText)
			assert.NoError(t, err)
			assert.NotEmpty(t, items)

		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for initialization")
		}
	})

	t.Run("Error Handling", func(t *testing.T) {
		_, mockService, _ := setupTest()

		// Force errors
		mockService.forceError = true

		// Test error handling for ListTables
		tables, err := mockService.ListTables(context.Background())
		assert.Error(t, err)
		assert.Nil(t, tables)

		// Test error handling for DescribeTable
		table, err := mockService.DescribeTable(context.Background(), "Table1")
		assert.Error(t, err)
		assert.Nil(t, table)

		// Test error handling for ScanTable
		items, err := mockService.ScanTable(context.Background(), "Table1")
		assert.Error(t, err)
		assert.Nil(t, items)
	})

	t.Run("Panel Interaction", func(t *testing.T) {
		view, _, _ := setupTest()

		// Wait for initialization
		time.Sleep(100 * time.Millisecond)

		// Test table navigation
		view.leftPanel.SetCurrentItem(1)
		mainText, _ := view.leftPanel.GetItemText(1)
		assert.Equal(t, "Table2", mainText, "Should be able to select second table")

		// Test table selection triggers data update
		cached := view.tableCache[mainText]
		assert.NotNil(t, cached)
		assert.Equal(t, mainText, *cached.TableName)
		assert.Equal(t, int64(200), *cached.ItemCount)
	})

	t.Run("Service Methods", func(t *testing.T) {
		_, mockService, _ := setupTest()

		tables, err := mockService.ListTables(context.Background())
		assert.NoError(t, err)
		assert.Len(t, tables, 2)
		assert.Contains(t, tables, "Table1")
		assert.Contains(t, tables, "Table2")

		table, err := mockService.DescribeTable(context.Background(), "Table1")
		assert.NoError(t, err)
		assert.Equal(t, "Table1", *table.TableName)
		assert.Equal(t, int64(100), *table.ItemCount)

		items, err := mockService.ScanTable(context.Background(), "Table1")
		assert.NoError(t, err)
		assert.Len(t, items, 2)
		assert.Equal(t, "1", items[0]["id"].(*types.AttributeValueMemberS).Value)
	})
}
