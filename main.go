package main

import (
	"github.com/google/gopacket/pcap"
	"github.com/xumc/datadt/display"
	tcpmonitor "github.com/xumc/datadt/tcpmonitor"
	"golang.org/x/sync/errgroup"
	"log"
	"os"
)

func main() {
	var g = errgroup.Group{}

	terminalOutputer := display.NewTerminalOutputer(os.Stdout)
	g.Go(terminalOutputer.Run)

	device := pcap.Interface{Name: "lo0"}

	// HTTP
	http := tcpmonitor.NewHttp(tcpmonitor.TcpCommon{
		Device: device,
		Kind:   tcpmonitor.KindHttp,
		Name:   "http-1",
		Port:   8000,
	})

	g.Go(func() error {
		tcpmonitor.RunTcpMonitor(http, terminalOutputer)
		return nil
	})

	// MYSQL
	mysql := tcpmonitor.NewMysql(tcpmonitor.TcpCommon{
		Device: device,
		Kind:   tcpmonitor.KindHttp,
		Name:   "mysql-1",
		Port:   3306,
	})

	g.Go(func() error {
		tcpmonitor.RunTcpMonitor(mysql, terminalOutputer)
		return nil
	})
	//
	//g.Go(func() error {
	//	ddbStream := &DDBStreamMonitor{outputer: terminalOutputer}
	//	ddbStream.Monitor([]string{"table1"})
	//	return nil
	//})
	//
	//g.Go(func() error {
	//	mysqlbinlog := &MysqlBinlogMonitor{outputer: terminalOutputer}
	//	mysqlbinlog.Monitor()
	//	return nil
	//})

	err := g.Wait()
	if err != nil {
		log.Fatalln(err)
	}
}
