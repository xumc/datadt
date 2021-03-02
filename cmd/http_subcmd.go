package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xumc/datadt/display"
	"github.com/xumc/datadt/tcpmonitor"
	"golang.org/x/sync/errgroup"
	"os"
)

var httpCommand = &cli.Command{
	Name:  "http",
	Usage: "http monitor",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "device",
			Aliases: []string{"d"},
			Usage:   "device",
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Usage:   "port",
		},
	},
	Action: func(c *cli.Context) error {
		var g = errgroup.Group{}

		// Display
		terminalOutputer := display.NewTerminalOutputer(os.Stdout)
		g.Go(terminalOutputer.Run)

		// HTTP
		http := tcpmonitor.NewHttp(tcpmonitor.TcpCommon{
			Device: c.String("device"),
			Kind:   tcpmonitor.KindHttp,
			Name:   "no name",
			Port:   (uint32)(c.Int("port")),
		})

		g.Go(func() error {
			tcpmonitor.RunTcpMonitor(http, terminalOutputer)
			return nil
		})

		return g.Wait()
	},
}
