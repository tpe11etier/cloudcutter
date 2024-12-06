package dynamodb

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Service struct {
	Client    *awsdynamodb.Client
	awsConfig aws.Config
}

func NewService(cfg aws.Config) *Service {
	client := awsdynamodb.NewFromConfig(cfg)
	return &Service{
		Client: client,
	}
}

func (s *Service) ListAllTables(ctx context.Context) ([]string, error) {
	var tableNames []string
	paginator := awsdynamodb.NewListTablesPaginator(s.Client, &awsdynamodb.ListTablesInput{})

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
	output, err := s.Client.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
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
	paginator := awsdynamodb.NewScanPaginator(s.Client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, output.Items...)
	}

	return items, nil
}
