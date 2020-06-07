package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"io"
	"log"
	"strconv"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"time"
)

const (
	ComQueryRequestPacket string     = "【Query】"

	COM_QUERY               = 3
)

var (
	snapshotLen int32  = 65535
	promiscuous bool   = false
	err         error
	timeout     time.Duration = 30 * time.Second
	handle      *pcap.Handle
)

func MonitorMysql(device pcap.Interface) {
	// Open device
	handle, err = pcap.OpenLive(device.Name, snapshotLen, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	var filter string = "tcp and port 3306"
	err = handle.SetBPFFilter(filter)
	if err != nil {
		log.Fatalln(err)
	}

	streamFactory := &mysqlStreamFactory{}
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

type mysqlStreamFactory struct{}

type mysqlStream struct {
	net, transport gopacket.Flow
	r              tcpreader.ReaderStream
}

func (h *mysqlStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	mysqlStream := &mysqlStream{
		net:       net,
		transport: transport,
		r:         tcpreader.NewReaderStream(),
	}
	go mysqlStream.run()

	return &mysqlStream.r
}

func (h *mysqlStream) run() {
	ResolveStream(h.net, h.transport, &(h.r))
}

func ResolveStream(net, transport gopacket.Flow, buf io.Reader){
	for {
		var payload bytes.Buffer
		var err error
		_, err = resolvePacketTo(buf, &payload)
		if err != nil {
			return
		}

		//close stream
		if err == io.EOF {
			fmt.Println(net, transport, " close")
			return
		} else if err != nil {
			fmt.Println("ERR : Unknown stream", net, transport, ":", err)
			return
		}

		//newPKseq := int(seq)
		//newPKLength := payload.Len()
		newPKPayload := payload.Bytes()
		var newPKIsClientFlow bool
		if transport.Src().String() == strconv.Itoa(3306) {
			newPKIsClientFlow = false
		}else{
			newPKIsClientFlow = true
		}

		if newPKIsClientFlow {
			switch newPKPayload[0] {
			case COM_QUERY:
				statement := string(newPKPayload[1:])
				msg := fmt.Sprintf("%s %s", ComQueryRequestPacket, statement)
				fmt.Println(msg)
			}
		}
	}
}

func resolvePacketTo(r io.Reader, w io.Writer) (uint8, error) {
	header := make([]byte, 4)
	if n, err := io.ReadFull(r, header); err != nil {
		if n == 0 && err == io.EOF {
			return 0, io.EOF
		}
		return 0, errors.New("ERR : Unknown stream")
	}

	length := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)

	var seq uint8
	seq = header[3]

	if n, err := io.CopyN(w, r, int64(length)); err != nil {
		return 0, errors.New("ERR : Unknown stream")
	} else if n != int64(length) {
		return 0, errors.New("ERR : Unknown stream")
	} else {
		return seq, nil
	}

	return seq, nil
}
