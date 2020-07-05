package display

import (
	"fmt"
	"github.com/kr/pretty"
	"github.com/olekukonko/tablewriter"
	"github.com/xumc/datadt/tcpmonitor/entity"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

type TerminalOutputer struct {
	writer  io.Writer
	inputer chan interface{}
}

func NewTerminalOutputer(writer io.Writer) Outputer {
	return &TerminalOutputer{
		writer:        writer,
		inputer: make(chan interface{}),
	}
}

func (to *TerminalOutputer) Inputer() chan <- interface{} {
	return to.inputer
}

func (to *TerminalOutputer) writeTableItem(item ChangeItem) error {
	fmt.Fprintln(to.writer)
	fmt.Fprintf(to.writer, "%s	%s\n", item.Action, item.TableName)

	tw := tablewriter.NewWriter(to.writer)
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

func (to *TerminalOutputer) writeStringItem(str string) error {
	to.writer.Write(([]byte)(str))
	return nil
}

func (to *TerminalOutputer) writeHttpRequest(request *http.Request) error {
	reqStr := fmt.Sprintf("\n\n%s %s\n", request.Method, request.RequestURI)
	to.writer.Write(([]byte)(reqStr))
	if request.Method == "POST" || request.Method == "PUT" {
		body, err := ioutil.ReadAll(request.Body)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		} else {
			to.writer.Write(body)
		}
	}
	request.Body.Close()
	return nil
}

func (to *TerminalOutputer) writeHttpResponse(response *http.Response) error {
	to.writer.Write(([]byte)(strconv.Itoa(response.StatusCode) + "\n"))
	body, err := ioutil.ReadAll(response.Body)
	if err == io.EOF {
		return nil
	} else if err != nil {
		return err
	} else if len(body) > 0 {
		to.writer.Write(body)
	}
	response.Body.Close()
	return nil
}



func (to *TerminalOutputer) Run() error {
	for {
		select {
		case item := <-to.inputer:
			switch realItem := item.(type) {
			case ChangeItem:
				err := to.writeTableItem(realItem)
				if err != nil {
					return err
				}

			case string:
				err := to.writeStringItem(realItem)
				if err != nil {
					return err
				}
			case *entity.HttpPair:
				err := to.writeHttpRequest(realItem.Request)
				if err != nil {
					return err
				}
				err = to.writeHttpResponse(realItem.Response)
				if err != nil {
					return err
				}

			default:
				pretty.Println(realItem)
			}
		}
	}

	return nil
}