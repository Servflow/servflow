package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/Servflow/servflow/config"
	"github.com/Servflow/servflow/internal/storage"
	"github.com/Servflow/servflow/pkg/engine/server"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/urfave/cli/v2"
)

const appName = "ServFlow"

func RunServer(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	storageClient, err := storage.GetClient()
	if err != nil {
		return err
	}

	eng, err := server.NewWithConfig(cfg)
	if err != nil {
		return err
	}

	if err := eng.Start(); err != nil {
		return err
	}

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

					log.Printf("Starting ServFlow with config folders - APIs: %s, Integrations: %s",
						cfg.ConfigFolder, cfg.IntegrationsFile)

					return RunServer(&cfg)
				},
			},
		},
	}

	return app
}
