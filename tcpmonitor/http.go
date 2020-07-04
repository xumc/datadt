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

type Http struct{
	TcpCommon
	source map[string]*httpPair
}

type httpPair struct{
	resquest string
	response string
}

func NewHttp(tc TcpCommon) *Http {
	return &Http{
		TcpCommon: tc,
	}
}

func(h *Http) Run(net, transport gopacket.Flow, buf io.Reader, outputer display.Outputer) {
	bio := bufio.NewReader(buf)

	var req *http.Request

	for {
		isResponse := transport.Src().String() == strconv.FormatUint(uint64(h.Port), 10)
		if isResponse {
			resp, err := http.ReadResponse(bio, req)
			req = nil
			if err == io.EOF {
				return
			} else if err != nil {
				continue
			} else {
				dumpedRespBytes, err := httputil.DumpResponse(resp, true)
				if err != nil {
					fmt.Println(err)
				}
				reader := bytes.NewReader(dumpedRespBytes)
				copiedResq, err := http.ReadResponse(bufio.NewReader(reader), req)
				outputer.Inputer() <- copiedResq
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
					fmt.Println(err)
				}
				reader := bytes.NewReader(dumpedReqBytes)
				copiedReq, err := http.ReadRequest(bufio.NewReader(reader))
				outputer.Inputer() <- copiedReq
			}
		}
	}
}
