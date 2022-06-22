package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	watchmon "github.com/realitycheck/watchmon/app"
	log "github.com/sirupsen/logrus"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "watchmon",
		Usage: "Streaming data into live charts.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Debug mode, enable for verbose logging",
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Quiet mode, enable to log nothing",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "create",
				Usage:  "Create new configuration",
				Action: create,
			},
			{
				Name:  "run",
				Usage: "Run specified configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "addr",
						Value: "127.0.0.1:8081",
						Usage: "Server address",
					},
					&cli.DurationFlag{
						Name:  "refreshPeriod",
						Value: 1 * time.Second,
						Usage: "Refresh period",
					},
					&cli.PathFlag{
						Name:     "configFile",
						Usage:    "Load configuration from `FILE`",
						Aliases:  []string{"f"},
						Required: true,
					},
				},
				Action: run,
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("quiet") {
				log.SetOutput(io.Discard)
			}
			if c.Bool("debug") {
				log.SetLevel(log.DebugLevel)
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	config, err := watchmon.LoadConfig(c.Path("configFile"))
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	ws := watchmon.NewWatchService(config)
	hs := watchmon.NewHTTPService(config)

	go ws.Start(context.Background(), c.Duration("refreshPeriod"))
	fmt.Printf("Run at http://%s\n", c.String("addr"))
	http.ListenAndServe(c.String("addr"), hs)
	return nil
}

func create(c *cli.Context) error {
	answers := struct {
		Filename string
		Parser   string
	}{}

	err := survey.Ask([]*survey.Question{
		{
			Name: "filename",
			Prompt: &survey.Input{
				Message: "Enter filename",
				Suggest: func(toComplete string) []string {
					files, _ := filepath.Glob(toComplete + "*")
					return files
				},
			},
			Validate: survey.Required,
		},
		{
			Name: "parser",
			Prompt: &survey.Select{
				Message: "Choose parser:",
				Options: []string{"csv", "htmlquery"},
				Default: "csv",
			},
		},
	}, &answers)
	if err != nil {
		return err
	}

	return watchmon.AppConfig{
		Monitors: []watchmon.MonitorConfig{
			{
				Id:    "my_monitor",
				Title: "My monitor",
				Value: watchmon.MonitorValueConfig{
					SourceId: "my_source",
					RecordId: "my_record",
					Header:   "value",
					Labels: []watchmon.MonitorValueLabelConfig{
						{Header: "id"},
					},
				},
			},
		},
		Sources: []watchmon.SourceConfig{
			{
				Id:      "my_source",
				Command: "echo 123",
				Timeout: 1 * time.Second,
				Output: watchmon.SourceOutputConfig{
					Parser: answers.Parser,
					Records: []watchmon.ParserRecordConfig{
						{
							Id:                "my_record",
							FirstLineIsHeader: false,
							Header:            []string{"id", "value"},
						},
					},
				},
			},
		},
		Graphs: []watchmon.GraphConfig{
			{
				Id: "my_monitor",
			},
		},
	}.Save(answers.Filename)
}
