package services

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpe11etier/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpe11etier/cloudcutter/internal/services/elastic"
)

type Services struct {
	DynamoDB dynamodb.Interface
	Elastic  *elastic.Service
	Region   string
}

func New(region string) *Services {
	return &Services{
		Region: region,
	}
}

func (s *Services) InitializeDynamoDB(cfg aws.Config) error {
	if s.DynamoDB == nil {
		s.DynamoDB = dynamodb.NewService(cfg)
	}
	return nil
}
