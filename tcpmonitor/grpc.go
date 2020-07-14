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
}

func NewGrpc(tc TcpCommon) *Grpc {
	return &Grpc{
		Http2: NewHttp2(tc),
	}
}

type grpcOutputer struct {
	inputer chan interface{}
	upOutputer display.Outputer
}

func (g grpcOutputer) Inputer() chan<- interface{} {
	return g.inputer
}

func (g grpcOutputer) Run() error {
	for {
		select {
		case item := <-g.inputer:
			realItem, ok := item.(*entity.Http2Frame)
			if !ok {
				panic("unsupported grpc type")
			}

			switch rf := realItem.Frame.(type) {
			case *http2.HeadersFrame:
				decoder := hpack.NewDecoder(2048, nil)
				hf, _ := decoder.DecodeFull(rf.HeaderBlockFragment())
				for _, h := range hf {
					fmt.Println(h.Name, " => ", h.Value)
				}
			case *http2.DataFrame:
				if realItem.IsClientFlow {
					msg := helloworld.HelloRequest{
					}
					err := msg.XXX_Unmarshal(rf.Data()[5:])
					pretty.Println(realItem.IsClientFlow, err, msg)
				} else {
					msg := helloworld.HelloReply{
					}
					err := msg.XXX_Unmarshal(rf.Data()[5:])
					pretty.Println(realItem.IsClientFlow, err, msg)
				}
			}
		}
	}
}

func (g *Grpc) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	gOutputer := &grpcOutputer{
		upOutputer: outputer,
	}

	go gOutputer.Run()

	g.Http2.Run(net, transport, buf, gOutputer)
}
