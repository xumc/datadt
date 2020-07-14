package tcpmonitor

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/gopacket"
	"github.com/xumc/datadt/display"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"
)

type Http struct {
	TcpCommon
	source map[string]*HttpConn
}

type HttpConn struct {
	outputer     display.Outputer
	requestChan  chan *http.Request
	responseChan chan *http.Response
}

func (conn *HttpConn) run() {
	for {
		select {
		case req := <-conn.requestChan:
			conn.outputer.Inputer() <- req
		case resp := <-conn.responseChan:
			conn.outputer.Inputer() <- resp
		}
	}
}

func NewHttp(tc TcpCommon) *Http {
	return &Http{
		TcpCommon: tc,
		source:    make(map[string]*HttpConn),
	}
}

func (h *Http) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	uuid := fmt.Sprintf("%v:%v", net.FastHash(), transport.FastHash())

	var newConn *HttpConn
	var ok bool
	if newConn, ok = h.source[uuid]; !ok {
		newConn = &HttpConn{
			outputer: outputer,
			requestChan: make(chan *http.Request),
			responseChan: make(chan *http.Response),
		}

		h.mu.Lock()
		h.source[uuid] = newConn
		h.mu.Unlock()

		go newConn.run()
	}

	bio := bufio.NewReader(buf)

	var req *http.Request

	for {
		isResponse := transport.Src().String() == strconv.FormatUint(uint64(h.Port), 10)
		if isResponse {
			resp, err := http.ReadResponse(bio, req)
			if err == io.EOF {
				return
			} else if err == io.ErrUnexpectedEOF{
				continue
			} else if err != nil {
				continue
			} else {
				dumpedRespBytes, err := httputil.DumpResponse(resp, true)
				if err != nil {
					continue
				}
				reader := bytes.NewReader(dumpedRespBytes)
				copiedResq, err := http.ReadResponse(bufio.NewReader(reader), req)
				if err != nil {
					continue
				}
				newConn.responseChan <- copiedResq
			}
		} else {
			var err error
			req, err = http.ReadRequest(bio)
			if err == io.EOF {
				return
			} else if err != nil {
				continue
			} else {
				dumpedReqBytes, err := httputil.DumpRequest(req, true)
				if err != nil {
					continue
				}
				reader := bytes.NewReader(dumpedReqBytes)
				copiedReq, err := http.ReadRequest(bufio.NewReader(reader))
				if err != nil {
					continue
				}
				newConn.requestChan <- copiedReq
			}
		}
	}
}
