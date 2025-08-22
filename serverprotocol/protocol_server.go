package serverprotocol

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
)

type ServerProtocol struct {
	Request  *apihttpprotocol.Message
	Response *apihttpprotocol.Message
}

func NewServerProtocol(readFn apihttpprotocol.HandlerFunc, writeFn apihttpprotocol.HandlerFunc) *ServerProtocol {
	p := &ServerProtocol{
		Request: &apihttpprotocol.Message{
			Context: context.Background(),
		},
		Response: &apihttpprotocol.Message{},
	}
	p.Response.Context = context.WithValue(p.Request.Context, ContextReqeustMessageKey, p.Request)
	p.WithReadIoFn(readFn).WithWriteIoFn(writeFn)
	return p
}

func (p *ServerProtocol) WithWriteIoFn(ioFn apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.Response.SetIOWriter(ioFn)
	return p
}

func (p *ServerProtocol) WithReadIoFn(ioFn apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.Request.SetIOReader(ioFn)
	return p
}
func (p *ServerProtocol) AddRequestMiddleware(middlewares ...apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.Request.AddMiddleware(middlewares...)
	return p
}

func (p *ServerProtocol) AddResponseMiddleware(middlewares ...apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.Response.AddMiddleware(middlewares...)
	return p
}

func (p *ServerProtocol) ResponseSuccess(data any) {
	err := p.WriteResponse(data)
	if err != nil {
		p.ResponseFail(err)
	}
}

func (p *ServerProtocol) ReadRequest(dst any) (err error) {
	p.Request.GoStructRef = dst
	p.Request.MiddlewareFuncs.Add(p.Request.GetIOReader())
	err = p.Request.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) WriteResponse(data any) (err error) {
	p.Response.GoStructRef = data
	p.Response.MiddlewareFuncs.Add(p.Response.GetIOWriter())
	err = p.Response.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) ResponseFail(err error) {
	p.Response.ResponseError = err
	err = p.WriteResponse(nil)
	if err != nil {
		panic(err) // 业务本身报错，在写入时还报错，直接panic ，避免循环调用
	}
}

func (c *ServerProtocol) SetContentType(contentType string) *ServerProtocol {
	c.Request.SetHeader("Content-Type", contentType)
	return c
}
func (c *ServerProtocol) SetContentTypeJson() *ServerProtocol {
	contentType := "application/json"
	c.SetContentType(contentType)
	return c
}

type ContextReqeustMessageKeyType string

var ContextReqeustMessageKey ContextReqeustMessageKeyType = "ContextReqeustMessage"

func ContextGetReqeustMessage(ctx context.Context) *apihttpprotocol.Message {
	return ctx.Value(ContextReqeustMessageKey).(*apihttpprotocol.Message)
}

//NewGinSerivceProtocol 这个函数注销，因为在客户端用于生成Android客户端时，不需要这个函数，尽量减少依赖

func NewGinSerivceProtocol(c *gin.Context) *ServerProtocol {
	readFn := func(message *apihttpprotocol.Message) (err error) {
		ioReader := c.Request.Body
		b, err := io.ReadAll(ioReader)
		if err != nil {
			return err
		}
		defer c.Request.Body.Close() // 关闭请求体，防止内存泄漏
		c.Request.Body = io.NopCloser(bytes.NewReader(b))
		for k, v := range c.Request.Header {
			message.SetHeader(k, v[0])
		}
		err = c.BindJSON(message.GoStructRef)
		return err
	}
	writeFn := func(message *apihttpprotocol.Message) (err error) {
		c.JSON(http.StatusOK, message.GoStructRef)
		return nil
	}
	protocol := NewServerProtocol(readFn, writeFn)
	return protocol
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func CodeMessageResponseMiddle(message *apihttpprotocol.Message) error {
	response := &Response{
		Data: message.GoStructRef,
	}
	err := message.ResponseError
	if err != nil {
		response.Code = 1
		response.Message = err.Error()
	}
	businessCode, exists := message.GetBusinessCode()
	if exists {
		response.Code = cast.ToInt(businessCode)
	}
	message.GoStructRef = response
	err = message.Next()
	if err != nil {
		return err
	}
	return nil
}
