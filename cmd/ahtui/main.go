package main

import (
	"context"
	"github.com/tpelletiersophos/ahtui/internal/services/aws"
	"github.com/tpelletiersophos/ahtui/internal/services/aws/dynamodb"
	ec2Service "github.com/tpelletiersophos/ahtui/internal/services/aws/ec2"
	"github.com/tpelletiersophos/ahtui/internal/services/manager"
	ddbv "github.com/tpelletiersophos/ahtui/internal/services/views/dynamodb"
	ec2view "github.com/tpelletiersophos/ahtui/internal/services/views/ec2"
	"github.com/tpelletiersophos/ahtui/ui"
	"log"
)

func main() {
	app := ui.NewApp()

	awsConfig, err := aws.Authenticate("default", "us-west-2")
	if err != nil {
		log.Fatalf("Failed to initialize AWS config: %v", err)
	}

	ctx := context.Background()
	viewManager := manager.NewViewManager(ctx, app, awsConfig)

	// Register service views
	ec2Svc := ec2Service.NewService(awsConfig)
	ec2View := ec2view.NewView(viewManager, ec2Svc)
	dynamoService := dynamodb.NewService(awsConfig)
	dynamoView := ddbv.NewView(viewManager, dynamoService)
	viewManager.RegisterView(dynamoView)
	viewManager.RegisterView(ec2View)

	// Set initial view
	if err := viewManager.SwitchToView("dynamodb"); err != nil {
		log.Fatalf("Failed to set initial view: %v", err)
	}

	// Run the application with the app
	if err := viewManager.Run(app); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
