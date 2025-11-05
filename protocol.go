package apihttpprotocol

import (
	"context"
	"net/http"
	"time"
)

type _Protocol struct {
	request  *RequestMessage
	response *ResponseMessage
}

func newProtocol() _Protocol {
	now := time.Now().Local()
	p := _Protocol{
		request: &RequestMessage{
			Message: Message[RequestMessage]{
				context: context.Background(),
				MetaData: MetaData{
					"timeNow": now,
				},
				Headers: http.Header{},
			},
		},
		response: &ResponseMessage{
			Message: Message[ResponseMessage]{
				context: context.Background(),
				MetaData: MetaData{
					"timeNow": now,
				},
				Headers: http.Header{},
			},
		},
	}
	p.request.Message.self = p.request
	p.response.Message.self = p.response
	p.request.responseMessage = p.response
	p.response.requestMessage = p.request
	return p
}

func (p *_Protocol) Request() *RequestMessage {
	return p.request
}

func (p *_Protocol) Response() *ResponseMessage {
	return p.response
}
func (p *_Protocol) SetLog(log LogI) *_Protocol {
	p.request.SetLog(log)
	p.response.SetLog(log)
	return p
}
