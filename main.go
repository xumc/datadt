package main

import (
	"flag"
	"fmt"
	"github.com/google/gopacket/pcap"
	"github.com/olekukonko/tablewriter"
	"os"
)

type outputItem struct {
	tableName string
	action string
	changes [][3]string
}

func main() {
	typ := flag.String("type", "ddb", "ddb or mysql")
	table := flag.String("table", "", "table names")

	flag.Parse()

	outputer := make(chan outputItem)
	switch *typ {
	case "ddb":
		TailDDB([]string{*table}, outputer)
	case "mysql":
		MonitorMysql(pcap.Interface{Name: "lo0"})
	}


	outputTo := os.Stdout
	for o := range outputer {
		fmt.Fprintln(outputTo)
		fmt.Fprintf(outputTo, "%s	%s\n", o.action, o.tableName)

		tw := tablewriter.NewWriter(outputTo)
		tw.SetHeader([]string{"Name", "Old Value", "New Value"})
		for _, c := range o.changes {
			tw.Append(c[:3])
		}
		tw.SetRowLine(true)
		tw.SetColMinWidth(1, 35)
		tw.SetColMinWidth(2, 35)
		tw.SetAlignment(tablewriter.ALIGN_LEFT)
		tw.Render()
	}
}

