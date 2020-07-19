package entity


import "golang.org/x/net/http2"

type Http2Frame struct {
	Frame        http2.Frame
	IsClientFlow bool
}

