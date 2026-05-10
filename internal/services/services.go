package services

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpe11etier/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpe11etier/cloudcutter/internal/services/elastic"
)

type Services struct {
	DynamoDB    dynamodb.Interface
	Elastic     *elastic.Service
	Region      string
	currentView string
}

func New(cfg aws.Config, region string) (*Services, error) {
	cfg.Region = region

	return &Services{
		Region: region,
	}, nil
}

func (s *Services) InitializeDynamoDB(cfg aws.Config) error {
	if s.DynamoDB == nil {
		s.DynamoDB = dynamodb.NewService(cfg)
	}
	return nil
}
