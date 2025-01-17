package services

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
)

type Services struct {
	DynamoDB dynamodb.Interface
	Elastic  *elastic.Service
	Region   string
}

func New(cfg aws.Config, region string) (*Services, error) {
	cfg.Region = region

	dynamoService := dynamodb.NewService(cfg)

	elasticService, err := elastic.NewService(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch service: %v", err)
	}

	return &Services{
		DynamoDB: dynamoService,
		Elastic:  elasticService,
		Region:   region,
	}, nil
}

func (s *Services) ReinitializeWithConfig(cfg aws.Config) {
	s.DynamoDB = dynamodb.NewService(cfg)
	if elasticService, err := elastic.NewService(cfg); err == nil {
		s.Elastic = elasticService
	}
	s.Region = cfg.Region
}
