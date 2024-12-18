package services

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/ec2"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
)

type Reinitializer interface {
	Reinitialize(cfg aws.Config)
}

type Services struct {
	EC2      *ec2.Service
	DynamoDB *dynamodb.Service
	Elastic  *elastic.Service
	Region   string
}

func New(cfg aws.Config, region string) (*Services, error) {
	cfg.Region = region

	ec2Svc := ec2.NewService(cfg)
	dynamoService := dynamodb.NewService(cfg)

	elasticService, err := elastic.NewService(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch service: %v", err)
	}

	return &Services{
		EC2:      ec2Svc,
		DynamoDB: dynamoService,
		Elastic:  elasticService,
		Region:   region,
	}, nil
}
