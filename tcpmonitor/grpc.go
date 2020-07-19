package tcpmonitor

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/kr/pretty"
	"github.com/xumc/datadt/display"
	"github.com/xumc/datadt/tcpmonitor/entity"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc/examples/helloworld/helloworld"
	"io"
)

type Grpc struct {
	*Http2
	outputer *grpcOutputer
}

func NewGrpc(tc TcpCommon) *Grpc {
	gOutputer := &grpcOutputer{
		inputer:       make(chan interface{}),
		exitChan:      make(chan struct{}),
		clientDecoder: hpack.NewDecoder(2048, nil),
		serverDecoder: hpack.NewDecoder(2048, nil),
	}

	go gOutputer.Run()

	return &Grpc{
		Http2: NewHttp2(tc),
		outputer: gOutputer,
	}
}

type grpcOutputer struct {
	inputer       chan interface{}
	upOutputer    display.Outputer
	exitChan      chan struct{}
	clientDecoder *hpack.Decoder
	serverDecoder *hpack.Decoder
}

func (g grpcOutputer) Inputer() chan<- interface{} {
	return g.inputer
}

func (g grpcOutputer) Run() error {
	for {
		select {
		case <- g.exitChan:
			return nil
		case item := <-g.inputer:
			realItem, ok := item.(*entity.Http2Frame)
			if !ok {
				panic("unsupported grpc type")
			}

			switch rf := realItem.Frame.(type) {
			case *http2.HeadersFrame:
				var hf []hpack.HeaderField
				var e error
				if realItem.IsClientFlow {
					hf, e = g.clientDecoder.DecodeFull(rf.HeaderBlockFragment())
					e = g.clientDecoder.Close()
				} else {
					hf, e = g.serverDecoder.DecodeFull(rf.HeaderBlockFragment())
					e = g.clientDecoder.Close()
				}
				fmt.Println("decode err:", e)
				fmt.Printf("header %v	%d	%v\n", realItem.IsClientFlow, rf.StreamID, rf.HeaderBlockFragment())

				for _, h := range hf {
					fmt.Println(h.Name, " => ", h.Value)
				}
			case *http2.DataFrame:
				if realItem.IsClientFlow {
					msg := helloworld.HelloRequest{
					}
					_ = msg.XXX_Unmarshal(rf.Data()[5:])
					pretty.Printf("body %b %d %s\n", realItem.IsClientFlow, rf.StreamID, msg.Name)
				} else {
					msg := helloworld.HelloReply{
					}
					_ = msg.XXX_Unmarshal(rf.Data()[5:])
					pretty.Printf("body %b %d %s\n", realItem.IsClientFlow, rf.StreamID, msg.Message)
				}
			default:
				pretty.Println(rf)
				continue
			}
		}
	}
}

func (g *Grpc) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	g.Http2.Run(net, transport, buf, g.outputer)
}
