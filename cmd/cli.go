package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Servflow/servflow/config"
	"github.com/Servflow/servflow/internal/storage"
	"github.com/Servflow/servflow/pkg/engine/server"
	"github.com/Servflow/servflow/pkg/engine/yamlloader"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const appName = "ServFlow"

// ValidationError represents a validation error for a specific config
type ValidationError struct {
	ConfigID string
	Error    error
}

func RunServer(cfg *config.Config, watch bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP)
	defer stop()

	storageClient, err := storage.GetClient()
	if err != nil {
		return err
	}

	eng, err := server.NewWithConfig(cfg)
	if err != nil {
		return err
	}

	if err := eng.Start(watch); err != nil {
		return err
	}

	// Handle SIGHUP for manual reload
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	go func() {
		for {
			select {
			case <-sigChan:
				log.Println("SIGHUP received, reloading configs...")
				if err := eng.Reload(); err != nil {
					log.Printf("Reload failed: %v", err)
				} else {
					log.Println("Reload successful")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case <-eng.DoneChan():
		return nil
	case <-ctx.Done():
		log.Println("Shutting down server...")
		if err := storageClient.Close(); err != nil {
			return err
		}
		if err := eng.Stop(); err != nil {
			return err
		}
		stop()
	}
	return nil
}

func ValidateConfigs(configFolder string, verbose bool) error {
	var logger = zap.NewNop()
	loader := yamlloader.NewYAMLLoader(configFolder, "", logger)
	configs, err := loader.FetchAPIConfigs(true)
	if err != nil {
		return fmt.Errorf("failed to load API configs: %w", err)
	}

	if len(configs) == 0 {
		return fmt.Errorf("no API configs found in %s", configFolder)
	}

	var validationErrors []ValidationError
	validCount := 0

	for _, cfg := range configs {
		if err := cfg.Validate(); err != nil {
			validationErrors = append(validationErrors, ValidationError{
				ConfigID: cfg.ID,
				Error:    err,
			})
		} else {
			validCount++
		}
	}

	fmt.Println("Validation Summary:")
	fmt.Printf("   Total configs: %d\n", len(configs))
	fmt.Printf("   Valid configs: %d\n", validCount)
	fmt.Printf("   Invalid configs: %d\n", len(validationErrors))

	if len(validationErrors) > 0 {
		fmt.Printf("\n Validation Errors:\n")
		for _, validationErr := range validationErrors {
			if verbose {
				fmt.Printf("   • Config '%s':\n     %v\n", validationErr.ConfigID, validationErr.Error)
			} else {
				fmt.Printf("   • Config '%s': Validation failed\n", validationErr.ConfigID)
			}
		}

		if !verbose {
			fmt.Printf("\nUse --verbose flag to see detailed error messages\n")
		}

		return fmt.Errorf("validation failed for %d configuration(s)", len(validationErrors))
	}

	fmt.Printf("\n🎉 All configurations are valid!\n")
	return nil
}

func CreateApp() *cli.App {
	app := &cli.App{
		Name:  "servflow",
		Usage: "ServFlow API server",
		Commands: []*cli.Command{
			{
				Name:      "start",
				Usage:     "Start the ServFlow server",
				ArgsUsage: "[CONFIG_FOLDER]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "integrations",
						Aliases:  []string{"i"},
						Usage:    "Path to integrations configuration folder",
						Required: false,
					},
					&cli.BoolFlag{
						Name:     "watch",
						Aliases:  []string{"w"},
						Usage:    "Enable hot reload - watch for config file changes",
						Value:    false,
						Required: false,
					},
				},
				Action: func(c *cli.Context) error {
					if err := godotenv.Load(); err != nil {
						log.Printf("Warning: %v", err)
					}

					var cfg config.Config
					err := envconfig.Process(appName, &cfg)
					if err != nil {
						return err
					}

					configFolder := c.Args().First()
					if configFolder != "" {
						cfg.ConfigFolder = configFolder
					}

					integrations := c.String("integrations")
					if integrations != "" {
						cfg.IntegrationsFile = integrations
					}

					if cfg.ConfigFolder == "" {
						return cli.Exit("Config folder for APIs must be specified either via environment variable SERVFLOW_CONFIGFOLDERS_APIS or as the first argument to 'run' command", 1)
					}

					watchEnabled := c.Bool("watch")

					log.Printf("Starting ServFlow with config folders - APIs: %s, Integrations: %s",
						cfg.ConfigFolder, cfg.IntegrationsFile)
					if watchEnabled {
						log.Printf("Hot reload enabled - watching for config changes")
					}

					return RunServer(&cfg, watchEnabled)
				},
			},
			{
				Name:        "validate",
				Usage:       "Validate configuration files",
				ArgsUsage:   "[CONFIG_FOLDER]",
				Description: "Validates all YAML configuration files in the specified folder using JSON Schema validation",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Show detailed validation errors",
						Value:   false,
					},
				},
				Action: func(c *cli.Context) error {
					configFolder := c.Args().First()
					if configFolder == "" {
						return cli.Exit("Config folder must be specified as the first argument", 1)
					}

					if _, err := os.Stat(configFolder); os.IsNotExist(err) {
						return cli.Exit(fmt.Sprintf("Config folder '%s' does not exist", configFolder), 1)
					}

					return ValidateConfigs(configFolder, c.Bool("verbose"))
				},
			},
		},
	}

	return app
}
