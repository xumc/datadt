package main

import (
	"github.com/google/gopacket/pcap"
	mysql2 "github.com/xumc/taildt/mysql"
	"golang.org/x/sync/errgroup"
	"log"
	"os"
)

func main() {
	tableOutputChan := make(chan OutputItem)
	stringChan := make(chan string)

	var g = errgroup.Group{}

	g.Go(func() error {
		//ddbStream := &DDBStreamMonitor{tableChan: tableOutputChan}
		//ddbStream.Monitor([]string{"table1"})
		return nil
	})

	g.Go(func() error {
		mysql := &mysql2.MysqlMonitor{StringChan: stringChan}
		mysql.Monitor(pcap.Interface{Name: "lo0"})
		return nil
	})

	g.Go(func() error {
		//mysqlbinlog := &MysqlBinlogMonitor{tableChan: tableOutputChan}
		//mysqlbinlog.Monitor()
		return nil
	})

	g.Go(func() error {
		defaultOutputer := &DefaultOutputer{Writer: os.Stdout,}

		for {
			select {
			case item := <- tableOutputChan:
				err := defaultOutputer.WriteTableItem(item)
				if err != nil {
					return err
				}
			case str := <- stringChan:
				err := defaultOutputer.WriteStringItem(str)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	err := g.Wait()
	if err != nil {
		log.Fatalln(err)
	}
}
