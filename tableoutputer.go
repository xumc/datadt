package main

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"io"
)

type OutputItem struct {
	TableName string
	Action    string
	Changes   [][3]string
}

type TableOutputer interface {
	WriteTableItem(item OutputItem) error
	WriteStringItem(str string) error
}

type DefaultOutputer struct{
	Writer io.Writer
}

func (dto *DefaultOutputer) WriteTableItem(item OutputItem) error {
	fmt.Fprintln(dto.Writer)
	fmt.Fprintf(dto.Writer, "%s	%s\n", item.Action, item.TableName)

	tw := tablewriter.NewWriter(dto.Writer)
	tw.SetHeader([]string{"Name", "Old Value", "New Value"})
	for _, c := range item.Changes {
		tw.Append(c[:3])
	}
	tw.SetRowLine(true)
	tw.SetColMinWidth(1, 35)
	tw.SetColMinWidth(2, 35)
	tw.SetAlignment(tablewriter.ALIGN_LEFT)
	tw.Render()

	return nil
}

func (dto *DefaultOutputer) WriteStringItem(str string) error {
	dto.Writer.Write(([]byte)(str))
	return nil
}
