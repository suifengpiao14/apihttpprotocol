package apihttpprotocol

import (
	"encoding/json"

	"resty.dev/v3"
)

func NewRestyClientProtocol(method string, url string) *Protocol {
	client := resty.New().EnableGenerateCurlCmd()
	defer client.Close()
	req := client.R()

	readFn := func(message *Message) (err error) {
		req.SetHeader("Content-Type", "application/json")
		response, err := req.Execute(method, url)
		if err != nil {
			panic(err)
		}
		b := response.Bytes()
		err = json.Unmarshal(b, message.GoStructRef)
		return err
	}
	writeFn := func(message *Message) (err error) {
		req.SetBody(message.GoStructRef)
		return err
	}
	protocol := NewClitentProtocol(readFn, writeFn).AddResponseMiddleware(MakeMiddlewareFuncWriteData(func(message *Message) error {
		response := &Response{
			Data: message.GoStructRef,
		}
		message.GoStructRef = response
		return nil
	}))
	return protocol
}
