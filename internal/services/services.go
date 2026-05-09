package services

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpe11etier/cloudcutter/internal/auth"
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

// InitializeElastic always (re)builds the Elastic service against the given
// AWS config. Replacing unconditionally avoids leaving a Dragos-transport
// service in place when the user switches from a Dragos profile to an AWS
// profile without first visiting the elastic view.
func (s *Services) InitializeElastic(cfg aws.Config) error {
	elasticService, err := elastic.NewService(cfg)
	if err != nil {
		return fmt.Errorf("error creating Elasticsearch service: %v", err)
	}
	s.Elastic = elasticService
	return nil
}

func (s *Services) InitializeElasticDragos(d *auth.DragosSession) error {
	svc, err := elastic.NewDragosService(d)
	if err != nil {
		return fmt.Errorf("error creating Dragos Elasticsearch service: %v", err)
	}
	s.Elastic = svc
	return nil
}
