package tcpmonitor

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/xumc/datadt/display"
	"io"
	"log"
	"time"
)

var (
	snapshotLen int32  = 65535
	promiscuous bool   = false
	timeout     time.Duration = 30 * time.Second
	handle      *pcap.Handle

	registeredMonitors = make(map[string]Monitor)
)

type TcpStreamFactory struct{
	outputer display.Outputer
	instance Monitor
}

func (h *TcpStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	mysqlStream := &TcpStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	go h.instance.Run(net, transport, &(mysqlStream.r), h.outputer)

	return &mysqlStream.r
}

type TcpStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

type Kind uint32
const (
	KindHttp Kind = iota
	KindMysql
)

type Monitor interface {
	GetDevice() pcap.Interface
	GetKind() Kind
	GetName() string
	GetPort() uint32
	Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer)
}

type TcpCommon struct {
	Device pcap.Interface
	Kind Kind
	Name string
	Port uint32
}

func (tc TcpCommon) GetDevice() pcap.Interface {
	return tc.Device
}
func (tc TcpCommon) GetKind() Kind {
	return tc.Kind
}
func (tc TcpCommon) GetName() string {
	return tc.Name
}
func (tc TcpCommon) GetPort() uint32 {
	return tc.Port
}

func RunTcpMonitor(monitor Monitor, out display.Outputer) {
	// Open device
	handle, err := pcap.OpenLive(monitor.GetDevice().Name, snapshotLen, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	var filter string = fmt.Sprintf("tcp and port %d", monitor.GetPort())
	err = handle.SetBPFFilter(filter)
	if err != nil {
		log.Fatalln(err)
	}

	streamFactory := &TcpStreamFactory{
		outputer: out,
		instance: monitor,
	}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	ticker := time.Tick(time.Second)

	for {
		select {
		case packet := <-packets:
			if packet == nil {
				return
			}
			if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
				log.Println("Unusable packet")
				continue
			}
			tcp := packet.TransportLayer().(*layers.TCP)
			assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)

		case <-ticker:
			assembler.FlushOlderThan(time.Now().Add(time.Second * -2))
		}
	}
}
