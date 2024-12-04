package ec2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Service struct {
	Client *awsec2.Client
}

func NewService(cfg aws.Config) *Service {
	client := awsec2.NewFromConfig(cfg)
	return &Service{
		Client: client,
	}
}

func (s *Service) FetchInstances(ctx context.Context) (map[string]*ec2types.Instance, error) {
	if s.Client == nil {
		return nil, fmt.Errorf("EC2 client is not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	paginator := awsec2.NewDescribeInstancesPaginator(s.Client, &awsec2.DescribeInstancesInput{})

	instances := make(map[string]*ec2types.Instance)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, reservation := range output.Reservations {
			for i := range reservation.Instances {
				instance := &reservation.Instances[i]
				instances[aws.ToString(instance.InstanceId)] = instance
			}
		}
	}
	return instances, nil
}
