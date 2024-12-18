package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services"
	awsservice "github.com/tpelletiersophos/cloudcutter/internal/services/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services/aws/dynamodb"
	ec2Service "github.com/tpelletiersophos/cloudcutter/internal/services/aws/ec2"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	ddbv "github.com/tpelletiersophos/cloudcutter/internal/ui/views/dynamodb"
	elasticView "github.com/tpelletiersophos/cloudcutter/internal/ui/views/elastic"
	"log"
)

func initializeServices(cfg aws.Config, region string) (*services.Services, error) {
	cfg.Region = region

	// Create all services with the same config
	ec2Svc := ec2Service.NewService(cfg)
	dynamoService := dynamodb.NewService(cfg)

	elasticService, err := elastic.NewService(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch service: %v", err)
	}

	return &services.Services{
		EC2:      ec2Svc,
		DynamoDB: dynamoService,
		Elastic:  elasticService,
		Region:   region,
	}, nil
}

func initializeViews(viewManager *manager.Manager, services *services.Services) error {
	// Initialize views
	//ec2View := ec2view.NewView(viewManager, services.EC2)
	dynamoView := ddbv.NewView(viewManager, services.DynamoDB)

	eView, err := elasticView.NewView(viewManager, services.Elastic, "main-summary-*")
	if err != nil {
		log.Printf("Warning: Failed to initialize Elastic view: %v", err)
	}

	// Register views
	if eView != nil {
		viewManager.RegisterView(eView)
	}
	viewManager.RegisterView(dynamoView)
	//viewManager.RegisterView(ec2View)
	//viewManager.RegisterView(testview.NewView(viewManager))

	return nil
}

func main() {
	ctx := context.Background()
	app := ui.NewApp()

	// Start with default region
	defaultRegion := "us-west-2"
	awsConfig, err := awsservice.Authenticate("default", defaultRegion)
	if err != nil {
		log.Fatalf("Failed to initialize AWS config: %v", err)
	}

	viewManager := manager.NewViewManager(ctx, app, awsConfig)

	// Initialize services with default region
	services, err := initializeServices(awsConfig, defaultRegion)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	if err := initializeViews(viewManager, services); err != nil {
		log.Fatalf("Failed to initialize views: %v", err)
	}

	// Set initial view
	if err := viewManager.SwitchToView("dynamodb"); err != nil {
		log.Fatalf("Failed to set initial view: %v", err)
	}

	if err := viewManager.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
