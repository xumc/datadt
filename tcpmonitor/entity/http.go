package entity

import "net/http"

type HttpPair struct{
	Request *http.Request
	Response *http.Response
}
