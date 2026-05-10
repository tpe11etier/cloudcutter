package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/logger"
	"github.com/tpe11etier/cloudcutter/internal/ui/views"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpe11etier/cloudcutter/internal/services"
	"github.com/tpe11etier/cloudcutter/internal/services/elastic"
	"github.com/tpe11etier/cloudcutter/internal/ui"
	"github.com/tpe11etier/cloudcutter/internal/ui/manager"
	ddbv "github.com/tpe11etier/cloudcutter/internal/ui/views/dynamodb"
	elasticView "github.com/tpe11etier/cloudcutter/internal/ui/views/elastic"
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
	if path := config.DefaultConfigPath(); path != "" {
		wrote, err := config.EnsureExists(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cloudcutter: %v\n", err)
			os.Exit(1)
		}
		if wrote {
			fmt.Fprintf(os.Stderr, "wrote starter config to %s — edit it and run cloudcutter again\n", path)
			os.Exit(0)
		}
	}

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

	defaultConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-west-2"))
	if err != nil {
		logInstance.Error("Failed to load default config", "error", err)
		defaultConfig = awssdk.Config{}
	}

	configPath := config.DefaultConfigPath()
	rawCfg, err := config.Load(configPath)
	if err != nil {
		logInstance.Error("Failed to load config", "path", configPath, "error", err)
		fmt.Fprintf(os.Stderr, "cloudcutter: failed to load %s: %v\n", configPath, err)
		os.Exit(1)
	}
	if err := config.Validate(rawCfg); err != nil {
		logInstance.Error("Config validation failed", "error", err)
		fmt.Fprintf(os.Stderr, "cloudcutter: config validation: %v\n", err)
		os.Exit(1)
	}
	homeDir, _ := os.UserHomeDir()
	awsProfiles, _ := environments.DiscoverAWSProfiles(homeDir)
	resolver := environments.NewResolver(rawCfg, awsProfiles)

	viewManager := manager.NewViewManager(ctx, app, defaultConfig, logInstance, resolver)

	viewManager.ShowProfileSelector()

	// Register lazy views
	services := services.New("us-west-2")
	viewManager.RegisterLazyView(manager.ViewDynamoDB, func() (views.View, error) {
		currentConfig := viewManager.GetCurrentConfig()
		if err := services.InitializeDynamoDB(currentConfig); err != nil {
			return nil, err
		}
		return ddbv.NewView(viewManager, services.DynamoDB), nil
	})
	viewManager.RegisterLazyView(manager.ViewElastic, func() (views.View, error) {
		session := viewManager.CurrentSession()
		if session == nil {
			return nil, fmt.Errorf("no active session for elastic view")
		}

		svc, err := elastic.NewServiceFromEnv(session.Environment, session.Config, session.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to create elastic service: %w", err)
		}
		services.Elastic = svc

		defaultIndex := session.Environment.IndexPattern
		if defaultIndex == "" {
			defaultIndex = "main-summary-*"
		}

		elasticViewInstance, err := elasticView.NewView(viewManager, services.Elastic, defaultIndex)
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
