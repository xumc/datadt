package main

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
)

const usage = `
	Usage: TBD
`

func main(){
	app := cli.NewApp()

	app.Name = "datadt"
	app.Usage = "data monitor"
	app.Authors = []*cli.Author{{Name: "xumc"}}
	app.UsageText = usage
	app.Description = app.Usage
	app.Version =  "1.0.0"


	app.Commands = append(app.Commands, httpCommand)

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "config path",
		},
	}
	app.Action = func(c *cli.Context) error {
		var configPath = c.String("config")
		return startWithConfigMode(configPath)
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}