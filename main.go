package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"go.uber.org/zap"

	_ "github.com/picostack/pico/logger"
	"github.com/picostack/pico/service"
	"github.com/picostack/pico/task"
)

var version = "master"

func main() {
	app := cli.NewApp()

	app.Name = "pico"
	app.Usage = "A git-driven task automation butler."
	app.UsageText = `pico [flags] [command]`
	app.Version = version
	app.Description = `Pico is a git-driven task runner to automate the application of configs.`
	app.Author = "Southclaws"
	app.Email = "hello@southcla.ws"

	app.Commands = []cli.Command{
		{
			Name:    "run",
			Aliases: []string{"r"},
			Description: `Starts the Pico daemon with the specified target repository. This
repository should contain one or more configuration files for Pico. When
this repository has new commits, Pico will automatically reconfigure.`,
			Usage:     "argument `target` specifies Git repository for configuration.",
			ArgsUsage: "target",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "git-username", EnvVar: "GIT_USERNAME"},
				cli.StringFlag{Name: "git-password", EnvVar: "GIT_PASSWORD"},
				cli.StringFlag{Name: "hostname", EnvVar: "HOSTNAME"},
				cli.StringFlag{Name: "directory", EnvVar: "DIRECTORY", Value: "./cache/"},
				cli.BoolFlag{Name: "ssh", EnvVar: "SSH"},
				cli.DurationFlag{Name: "check-interval", EnvVar: "CHECK_INTERVAL", Value: time.Second * 10},
				cli.StringFlag{Name: "vault-addr", EnvVar: "VAULT_ADDR"},
				cli.StringFlag{Name: "vault-token", EnvVar: "VAULT_TOKEN"},
				cli.StringFlag{Name: "vault-path", EnvVar: "VAULT_PATH", Value: "/secret"},
				cli.DurationFlag{Name: "vault-renew-interval", EnvVar: "VAULT_RENEW_INTERVAL", Value: time.Hour * 24},
			},
			Action: func(c *cli.Context) (err error) {
				if !c.Args().Present() {
					cli.ShowCommandHelp(c, "run")
					return errors.New("missing argument: target")
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				// If no hostname is provided, use the actual host's hostname
				hostname := c.String("hostname")
				if hostname == "" {
					hostname, err = os.Hostname()
					if err != nil {
						return errors.Wrap(err, "failed to get hostname")
					}
				}

				zap.L().Debug("initialising service")

				svc, err := service.Initialise(service.Config{
					Target: task.Repo{
						URL:  c.Args().First(),
						User: c.String("git-username"),
						Pass: c.String("git-password"),
					},
					Hostname:      hostname,
					Directory:     c.String("directory"),
					SSH:           c.Bool("ssh"),
					CheckInterval: c.Duration("check-interval"),
					VaultAddress:  c.String("vault-addr"),
					VaultToken:    c.String("vault-token"),
					VaultPath:     c.String("vault-path"),
					VaultRenewal:  c.Duration("vault-renew-interval"),
				})
				if err != nil {
					return errors.Wrap(err, "failed to initialise")
				}

				zap.L().Info("service initialised")

				errs := make(chan error, 1)
				go func() { errs <- svc.Start(ctx) }()

				s := make(chan os.Signal, 1)
				signal.Notify(s, os.Interrupt)

				select {
				case <-ctx.Done():
					err = ctx.Err()
				case sig := <-s:
					err = errors.New(sig.String())
				case err = <-errs:
				}

				return
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		zap.L().Fatal("exit", zap.Error(err))
	}
}
