package main

import (
	"fmt"
	"github.com/kr/pretty"
	"github.com/siddontang/go-mysql/canal"
	"github.com/xumc/datadt/display"
	"strconv"
)

type MysqlBinlogMonitor struct {
	canal.DummyEventHandler
	outputer display.Outputer
}

func(binlog *MysqlBinlogMonitor) Monitor() {
	cfg := canal.NewDefaultConfig()
	cfg.Addr = "127.0.0.1:3306"
	cfg.User = "root"
	cfg.Password = "root"
	// We only care table canal_test in test db
	cfg.Dump.TableDB = "mytest"
	cfg.Dump.Tables = []string{"test_table"}
	cfg.Dump.ExecutionPath = ""

	c, err := canal.NewCanal(cfg)
	if err != nil {
		panic(err)
	}

	// Register a handler to handle RowsEvent
	c.SetEventHandler(binlog)

	fmt.Println("hello")
	// Start canal
	pos, err := c.GetMasterPos()
	if err != nil {
		panic(err)
	}
	fmt.Println(pos, "aaa")

	c.RunFrom(pos)
}

func(binlog *MysqlBinlogMonitor) OnRow(e *canal.RowsEvent) error {
	fmt.Printf("%s %v\n", e.Action, e.Rows)

	changes := make([][3]string, len(e.Rows[0]))
	for i := 0; i < len(e.Rows[0]); i++ {
		changes[i][0] = e.Table.Columns[i].Name
	}

	var delta = 0
	if e.Action == "insert" {
		delta = 1
	}

	for i, row := range e.Rows {
		for j, c := range row {
			changes[j][i+1+delta] = translateMysqlValue(c)
			changes[j][i+1+delta] = translateMysqlValue(c)
		}
	}

	binlog.outputer.Inputer() <- display.ChangeItem{
		TableName:e.Table.Name,
		Action: e.Action,
		Changes: changes,
	}

	return nil
}

func translateMysqlValue(c interface{}) string {
	switch v := c.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	default:
		return pretty.Sprint(v)
	}
}
