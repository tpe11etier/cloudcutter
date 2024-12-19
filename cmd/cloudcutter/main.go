package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services"
	awsservice "github.com/tpelletiersophos/cloudcutter/internal/services/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	ddbv "github.com/tpelletiersophos/cloudcutter/internal/ui/views/dynamodb"
	elasticView "github.com/tpelletiersophos/cloudcutter/internal/ui/views/elastic"
	"log"
)

func initializeServices(cfg aws.Config, region string) (*services.Services, error) {
	return services.New(cfg, region)
}

func initializeViews(viewManager *manager.Manager, services *services.Services) error {

	dynamoView := ddbv.NewView(viewManager, services.DynamoDB)

	eView, err := elasticView.NewView(viewManager, services.Elastic, "main-summary-*")
	if err != nil {
		log.Printf("Warning: Failed to initialize Elastic view: %v", err)
	}

	viewManager.RegisterView(eView)
	viewManager.RegisterView(dynamoView)

	return nil
}

func main() {
	ctx := context.Background()
	app := ui.NewApp()

	defaultRegion := "us-west-2"
	awsConfig, err := awsservice.Authenticate("default", defaultRegion)
	if err != nil {
		log.Fatalf("Failed to initialize AWS config: %v", err)
	}

	viewManager := manager.NewViewManager(ctx, app, awsConfig)

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
