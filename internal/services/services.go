package services

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
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

func (s *Services) InitializeElastic(cfg aws.Config) error {
	if s.Elastic == nil {
		elasticService, err := elastic.NewService(cfg)
		if err != nil {
			return fmt.Errorf("error creating Elasticsearch service: %v", err)
		}
		s.Elastic = elasticService
	}
	return nil
}

func (s *Services) ReinitializeWithConfig(cfg aws.Config, viewName string) error {
	s.Region = cfg.Region

	switch viewName {
	case "dynamodb":
		s.DynamoDB = dynamodb.NewService(cfg)
	case "elastic":
		elasticService, err := elastic.NewService(cfg)
		if err != nil {
			return fmt.Errorf("error creating Elasticsearch service: %v", err)
		}
		s.Elastic = elasticService
	}

	return nil

}
