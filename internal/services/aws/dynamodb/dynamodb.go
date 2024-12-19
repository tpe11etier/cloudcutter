package dynamodb

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Interface interface {
	ListTables(ctx context.Context) ([]string, error)
	DescribeTable(ctx context.Context, tableName string) (*dynamodbtypes.TableDescription, error)
	ScanTable(ctx context.Context, tableName string) ([]map[string]dynamodbtypes.AttributeValue, error)
}

type Service struct {
	client *awsdynamodb.Client
}

func NewService(cfg aws.Config) Interface {
	return &Service{
		client: awsdynamodb.NewFromConfig(cfg),
	}
}

func (s *Service) ListTables(ctx context.Context) ([]string, error) {
	var tableNames []string
	paginator := awsdynamodb.NewListTablesPaginator(s.client, &awsdynamodb.ListTablesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		tableNames = append(tableNames, output.TableNames...)
	}
	return tableNames, nil
}

func (s *Service) DescribeTable(ctx context.Context, tableName string) (*dynamodbtypes.TableDescription, error) {
	output, err := s.client.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return nil, err
	}
	return output.Table, nil
}

func (s *Service) ScanTable(ctx context.Context, tableName string) ([]map[string]dynamodbtypes.AttributeValue, error) {
	var items []map[string]dynamodbtypes.AttributeValue
	input := &awsdynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(100),
	}
	paginator := awsdynamodb.NewScanPaginator(s.client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, output.Items...)
	}
	return items, nil
}
