package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/views"
	"log"
	"os"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/services"
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

func runApplication() {
	ctx := context.Background()
	app := ui.NewApp()

	logLevel := strings.ToLower(viper.GetString("logging"))
	level, err := logger.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("Invalid log level %q: %v", logLevel, err)
	}

	logInstance, err := logger.New(logger.Config{
		LogDir: "./logs",
		Prefix: "cloudcutter",
		Level:  level,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logInstance.Close()

	defaultConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
	if err != nil {
		logInstance.Error("Failed to load default config", "error", err)
		defaultConfig = awssdk.Config{}
	}

	viewManager := manager.NewViewManager(ctx, app, defaultConfig, logInstance)

	// Show the profile selector if not authenticated
	viewManager.EnsureAuthenticated()

	// Register lazy views
	services, _ := services.New(defaultConfig, "us-west-2")
	viewManager.RegisterLazyView(manager.ViewDynamoDB, func() (views.View, error) {
		currentConfig := viewManager.GetCurrentConfig()
		if err := services.InitializeDynamoDB(currentConfig); err != nil {
			return nil, err
		}
		return ddbv.NewView(viewManager, services.DynamoDB), nil
	})
	viewManager.RegisterLazyView(manager.ViewElastic, func() (views.View, error) {
		currentConfig := viewManager.GetCurrentConfig()
		if err := services.InitializeElastic(currentConfig); err != nil {
			return nil, err
		}
		elasticViewInstance, err := elasticView.NewView(viewManager, services.Elastic, "main-summary-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create elastic view: %w", err)
		}
		return elasticViewInstance, nil
	})

	if err := viewManager.Run(); err != nil {
		logInstance.Error("Application error", "error", err)
		os.Exit(1)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
