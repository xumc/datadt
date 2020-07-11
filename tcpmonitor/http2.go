package tcpmonitor

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/gopacket"
	"github.com/kr/pretty"
	"github.com/xumc/datadt/display"
	"golang.org/x/net/http2"
	"io"
	"strconv"
)

type Http2 struct {
	TcpCommon
	source map[string]*Http2Conn
}
type Http2Frame struct {
	frame http2.Frame
	isClientFlow bool
}
type Http2Conn struct {
	frameChan chan *Http2Frame
}

func NewHttp2(tc TcpCommon) *Http2 {
	return &Http2{
		TcpCommon: tc,
		source:    make(map[string]*Http2Conn),
	}
}


func (h2 *Http2) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	uuid := fmt.Sprintf("%v:%v", net.FastHash(), transport.FastHash())
	fmt.Println(uuid)

	if _, ok := h2.source[uuid]; !ok {
		var newConn = &Http2Conn{
			frameChan: make(chan *Http2Frame),
		}
		h2.source[uuid] = newConn

		go newConn.run()
	}

	var startWithClientFace = true

	bio := bufio.NewReader(buf)
	emptyWriter := bufio.NewWriter(nil)
	fr := http2.NewFramer(emptyWriter, bio)

	for {
		isFromServer := transport.Src().String() == strconv.FormatUint(uint64(h2.Port), 10)

		frame := &Http2Frame{
			isClientFlow: !isFromServer,
		}

		if !isFromServer && startWithClientFace {
			bs := make([]byte, len(http2.ClientPreface))
			n, err := io.ReadFull(bio, bs)
			if err == io.EOF {
				return
			} else if err == io.ErrUnexpectedEOF {
				newReader := io.MultiReader(bytes.NewReader(bs[:n]), bio)
				bio = bufio.NewReader(newReader)
				continue
			} else if string(bs) == http2.ClientPreface {
				fmt.Println("clientface")
				startWithClientFace = false
				continue
			}
		}

		f, err := fr.ReadFrame()
		if err == io.EOF {
			return
		} else if err == io.ErrUnexpectedEOF {
			fmt.Println("server unexpected err: ", isFromServer, err)
			continue
		} else if err != nil {
			fmt.Println("server err: ", err)
			continue
		}else {
			frame.frame = f
			h2.source[uuid].frameChan <- frame
		}

	}
}

func (h2Conn *Http2Conn) run() {
	for {
		select {
		case frame := <- h2Conn.frameChan:
			switch rf := frame.frame.(type) {
			//case *http2.HeadersFrame:
			case *http2.DataFrame:
				pretty.Println(frame.isClientFlow, " => ", string(rf.Data()))
			default:
				pretty.Println(frame.isClientFlow, " => ", rf)
			}
		}
	}
}
