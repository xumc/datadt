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
	clientFaceNotPassed bool
}

func NewHttp2(tc TcpCommon) *Http2 {
	return &Http2{
		TcpCommon: tc,
		source:    make(map[string]*Http2Conn),
	}
}



func (h2 *Http2) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	uuid := fmt.Sprintf("%v:%v", net.FastHash(), transport.FastHash())

	var newConn *Http2Conn
	var ok bool
	if newConn, ok = h2.source[uuid]; !ok {
		newConn = &Http2Conn{
			outputer: outputer,
			frameChan: make(chan *entity.Http2Frame),
			clientFaceNotPassed: true,
		}

		h2.mu.Lock()
		h2.source[uuid] = newConn
		h2.mu.Unlock()

		go newConn.run()
	}

	bio := bufio.NewReader(buf)
	emptyWriter := bufio.NewWriter(nil)
	fr := http2.NewFramer(emptyWriter, bio)

	for {
		isFromServer := transport.Src().String() == strconv.FormatUint(uint64(h2.Port), 10)

		frame := &entity.Http2Frame{
			IsClientFlow: !isFromServer,
		}

		if !isFromServer && newConn.clientFaceNotPassed {
			bs := make([]byte, len(http2.ClientPreface))
			n, err := io.ReadFull(bio, bs)
			if err == io.EOF {
				return
			} else if err != nil {
				newReader := io.MultiReader(bytes.NewReader(bs[:n]), bio)
				bio = bufio.NewReader(newReader)
				continue
			} else if string(bs) == http2.ClientPreface {
				newConn.clientFaceNotPassed = false
				continue
			}
		}

		f, err := fr.ReadFrame()
		if err == io.EOF {
			return
		} else if err == io.ErrUnexpectedEOF {
			continue
		} else if err != nil {
			continue
		}else {
			newConn.clientFaceNotPassed = false
			copiedFrame := copyFrame(f)

			if copiedFrame != nil {
				frame.Frame = copiedFrame
				newConn.frameChan <- frame
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

func copyFrame(frame http2.Frame) http2.Frame {
	switch rf := frame.(type) {
	case *http2.SettingsFrame:
		copiedFrame := &http2.SettingsFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copyBytes(copiedFrame, frame, "p")
		return copiedFrame
	case *http2.HeadersFrame:
		copiedFrame := &http2.HeadersFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.Priority = rf.Priority
		copyBytes(copiedFrame, frame, "headerFragBuf")
		return copiedFrame
	case *http2.MetaHeadersFrame:
		copiedFrame := &http2.MetaHeadersFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.Truncated = rf.Truncated
		copiedFrame.Fields = rf.Fields
		return copiedFrame
	case *http2.WindowUpdateFrame:
		copiedFrame := &http2.WindowUpdateFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.Increment = rf.Increment
		return copiedFrame
	case *http2.PingFrame:
		copiedFrame := &http2.PingFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.Data = rf.Data
		return copiedFrame
	case *http2.RSTStreamFrame:
		copiedFrame := &http2.RSTStreamFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.ErrCode = rf.ErrCode
		return copiedFrame
	case *http2.PriorityFrame:
		copiedFrame := &http2.PriorityFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.PriorityParam = rf.PriorityParam
		return copiedFrame
	case *http2.GoAwayFrame:
		copiedFrame := &http2.GoAwayFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.ErrCode = rf.ErrCode
		copiedFrame.LastStreamID = rf.LastStreamID
		copyBytes(copiedFrame, frame, "debugData")
		return copiedFrame
	case *http2.PushPromiseFrame:
		copiedFrame := &http2.PushPromiseFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copiedFrame.PromiseID = rf.PromiseID
		copyBytes(copiedFrame, frame, "headerFragBuf")
		return copiedFrame
	case *http2.DataFrame:
		copiedFrame := &http2.DataFrame{}
		copiedFrame.FrameHeader = rf.FrameHeader
		copyBytes(copiedFrame, frame, "data")
		return copiedFrame
	default:
		panic("unsupport frame type")
	}

	return nil
}

func copyBytes(copiedFrame http2.Frame, rf http2.Frame, fieldName string) {
	pointerVal := reflect.ValueOf(copiedFrame)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName(fieldName)
	ptrToP := unsafe.Pointer(member.UnsafeAddr())
	realPtrToP := (*[]byte)(ptrToP)

	rfPointerVal := reflect.ValueOf(rf)
	rfVal := reflect.Indirect(rfPointerVal)
	pv := rfVal.FieldByName(fieldName)
	rtPData := pv.Bytes()

	*realPtrToP = make([]byte, len(rtPData))
	copy(*realPtrToP, rtPData)
}
