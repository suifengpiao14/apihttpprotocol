package apihttpprotocol

import (
	"context"
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
				Context: context.Background(),
				Metadata: Metadata{
					"timeNow": now,
				},
			},
		},
		response: &ResponseMessage{
			Message: Message[ResponseMessage]{
				Context: context.Background(),
				Metadata: Metadata{
					"timeNow": now,
				},
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
