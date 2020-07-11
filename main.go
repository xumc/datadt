package main

import (
	"flag"
	"fmt"
	"github.com/spf13/viper"
	"github.com/xumc/datadt/display"
	tcpmonitor "github.com/xumc/datadt/tcpmonitor"
	"golang.org/x/sync/errgroup"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "config file path")

	flag.Parse()

	dir := filepath.Dir(configPath)
	fileFullName := filepath.Base(configPath)
	filename := strings.Split(fileFullName, ".")[0]

	viper.SetConfigName(filename)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	var g = errgroup.Group{}


	// Display
	terminalOutputer := display.NewTerminalOutputer(os.Stdout)
	g.Go(terminalOutputer.Run)


	// HTTP
	httpEndPoints := viper.GetStringMap("datasources.HTTP")
	for name, ep := range httpEndPoints {
		realEp := ep.(map[string]interface{})
		http := tcpmonitor.NewHttp(tcpmonitor.TcpCommon{
			Device: realEp["device"].(string),
			Kind:   tcpmonitor.KindHttp,
			Name:   name,
			Port:   (uint32)(realEp["port"].(int)),
		})

		g.Go(func() error {
			tcpmonitor.RunTcpMonitor(http, terminalOutputer)
			return nil
		})
	}

	// HTTP2
	http2EndPoints := viper.GetStringMap("datasources.HTTP2")
	for name, ep := range http2EndPoints {
		realEp := ep.(map[string]interface{})
		http2 := tcpmonitor.NewHttp2(tcpmonitor.TcpCommon{
			Device: realEp["device"].(string),
			Kind:   tcpmonitor.KindHttp,
			Name:   name,
			Port:   (uint32)(realEp["port"].(int)),
		})

		g.Go(func() error {
			tcpmonitor.RunTcpMonitor(http2, terminalOutputer)
			return nil
		})
	}

	// MYSQL
	mysqlEndPoints := viper.GetStringMap("datasources.MYSQL")
	for name, ep := range mysqlEndPoints {
		realEp := ep.(map[string]interface{})
		mysql := tcpmonitor.NewMysql(tcpmonitor.TcpCommon{
			Device: realEp["device"].(string),
			Kind:   tcpmonitor.KindHttp,
			Name:   name,
			Port:   (uint32)(realEp["port"].(int)),
		})

		g.Go(func() error {
			tcpmonitor.RunTcpMonitor(mysql, terminalOutputer)
			return nil
		})
	}
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

	err = g.Wait()
	if err != nil {
		log.Fatalln(err)
	}
}
