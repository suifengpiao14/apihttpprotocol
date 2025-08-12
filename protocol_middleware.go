package apihttpprotocol

import "github.com/spf13/cast"

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

//ProtocolMiddlewareCodeMessage 封装返回体code/message 协议
func ProtocolMiddlewareCodeMessage(p *Protocol) *Protocol {
	protocol := p.AddResponseMiddleware(MakeMiddlewareFuncWriteData(func(message *Message) error {
		code := cast.ToInt(message.GetMetaData(MetaData_Code, MetaData_Code_Success))
		data := message.GoStructRef
		msg := "success"
		if code > 0 {
			msg = cast.ToString(data)
			data = nil
		}
		response := &Response{
			Code:    code,
			Message: msg,
			Data:    data,
		}
		message.GoStructRef = response
		return nil
	}))
	return protocol
}
