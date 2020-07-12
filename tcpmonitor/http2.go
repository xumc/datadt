package tcpmonitor

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/gopacket"
	"github.com/xumc/datadt/display"
	"github.com/xumc/datadt/tcpmonitor/entity"
	"golang.org/x/net/http2"
	"io"
	"reflect"
	"strconv"
	"unsafe"
)

type Http2 struct {
	TcpCommon
	source map[string]*Http2Conn
}

type Http2Conn struct {
	outputer display.Outputer
	frameChan chan *entity.Http2Frame
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
			outputer: outputer,
			frameChan: make(chan *entity.Http2Frame),
		}
		h2.source[uuid] = newConn

		go newConn.run()
	}

	var clientFaceNotPassed = true

	bio := bufio.NewReader(buf)
	emptyWriter := bufio.NewWriter(nil)
	fr := http2.NewFramer(emptyWriter, bio)

	for {
		isFromServer := transport.Src().String() == strconv.FormatUint(uint64(h2.Port), 10)

		frame := &entity.Http2Frame{
			IsClientFlow: !isFromServer,
		}

		if !isFromServer && clientFaceNotPassed {
			bs := make([]byte, len(http2.ClientPreface))
			n, err := io.ReadFull(bio, bs)
			if err == io.EOF {
				return
			} else if err != nil {
				fmt.Println(err)
				newReader := io.MultiReader(bytes.NewReader(bs[:n]), bio)
				bio = bufio.NewReader(newReader)
				continue
			} else if string(bs) == http2.ClientPreface {
				fmt.Println("clientface")
				clientFaceNotPassed = false
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
			clientFaceNotPassed = false
			rf, ok := f.(*http2.DataFrame)
			if ok {
				fff := &http2.DataFrame{}
				fff.FrameHeader = rf.FrameHeader

				pointerVal := reflect.ValueOf(fff)
				val := reflect.Indirect(pointerVal)

				member := val.FieldByName("data")
				ptrToY := unsafe.Pointer(member.UnsafeAddr())
				realPtrToY := (*[]byte)(ptrToY)
				*realPtrToY = make([]byte, len(rf.Data()))
				copy(*realPtrToY, rf.Data())

				frame.Frame = fff
				h2.source[uuid].frameChan <- frame
			}
		}

	}
}

func (h2Conn *Http2Conn) run() {
	for {
		select {
		case frame := <- h2Conn.frameChan:
			h2Conn.outputer.Inputer() <- frame
		}
	}
}
