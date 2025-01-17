package main

import (
	"context"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/views"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/services"
	awsservice "github.com/tpelletiersophos/cloudcutter/internal/services/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	ddbv "github.com/tpelletiersophos/cloudcutter/internal/ui/views/dynamodb"
	elasticView "github.com/tpelletiersophos/cloudcutter/internal/ui/views/elastic"
)

var (
	debugLevel string
	rootCmd    = &cobra.Command{
		Use:   "cloudcutter",
		Short: "Cloudcutter CLI",
		Run: func(cmd *cobra.Command, args []string) {
			runApplication()
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&debugLevel, "logging", "info", "Set the debug level (e.g., debug, info, warn, error)")
	viper.BindPFlag("logging", rootCmd.PersistentFlags().Lookup("logging"))

	viper.SetDefault("logging", "info")
	viper.AutomaticEnv()
}

func initializeServices(cfg aws.Config, region string) (*services.Services, error) {
	return services.New(cfg, region)
}

func runApplication() {

	ctx := context.Background()
	app := ui.NewApp()

	logDir := "./logs"
	logPrefix := "cloudcutter"
	logLevel := strings.ToLower(viper.GetString("logging"))

	level, err := logger.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("Invalid log level %q: %v", logLevel, err)
	}

	logInstance, err := logger.New(logger.Config{
		LogDir: logDir,
		Prefix: logPrefix,
		Level:  level,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logInstance.Close()

	defaultRegion := "us-west-2"
	awsConfig, err := awsservice.Authenticate("default", defaultRegion)
	if err != nil {
		log.Fatalf("Failed to initialize AWS config: %v", err)
	}

	viewManager := manager.NewViewManager(ctx, app, awsConfig, logInstance)

	services, err := initializeServices(awsConfig, defaultRegion)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	viewManager.RegisterLazyView(manager.ViewDynamoDB, func() (views.View, error) {
		return ddbv.NewView(viewManager, services.DynamoDB), nil
	})

	viewManager.RegisterLazyView(manager.ViewElastic, func() (views.View, error) {
		return elasticView.NewView(viewManager, services.Elastic, "main-summary-*")
	})

	// Set initial view
	if err := viewManager.SwitchToView(manager.ViewDynamoDB); err != nil {
		log.Fatalf("Failed to set initial view: %v", err)
	}

	// Run the application
	if err := viewManager.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
