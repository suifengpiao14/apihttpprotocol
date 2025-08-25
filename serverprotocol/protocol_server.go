package serverprotocol

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
)

type ServerProtocol struct {
	request  *apihttpprotocol.Message
	response *apihttpprotocol.Message
}

func NewServerProtocol() *ServerProtocol {
	p := &ServerProtocol{
		request: &apihttpprotocol.Message{
			Context: context.Background(),
		},
		response: &apihttpprotocol.Message{},
	}
	p.response.SetRequestMessage(p.request)
	return p
}

func (p *ServerProtocol) WithIOFn(reder, writer apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.request.SetIOReader(reder)
	p.response.SetIOWriter(writer)
	return p
}

type Option interface {
	Apply(p *ServerProtocol) *ServerProtocol
}

type OptionFunc func(p *ServerProtocol) *ServerProtocol

func (f OptionFunc) Apply(p *ServerProtocol) *ServerProtocol {
	return f(p)
}

func (p *ServerProtocol) Apply(options ...Option) *ServerProtocol {
	for _, option := range options {
		p = option.Apply(p)
	}
	return p
}

//ApplyRequestMiddleware 添加请求中间件，这个函数属于底层函数,供中间件封装使用，不建议直接调用，业务开发建议使用 ServerProtocol.Apply()

func (p *ServerProtocol) ApplyRequestMiddleware(middlewares ...apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.request.AddMiddleware(middlewares...)
	return p
}

// ApplyResponseMiddleware 添加响应中间件，这个函数属于底层函数,供中间件封装使用，不建议直接调用，业务开发建议使用 ServerProtocol.Apply()
func (p *ServerProtocol) ApplyResponseMiddleware(middlewares ...apihttpprotocol.HandlerFunc) *ServerProtocol {
	p.response.AddMiddleware(middlewares...)
	return p
}

func (p *ServerProtocol) ResponseSuccess(data any) {
	err := p.writeResponse(data)
	if err != nil {
		p.ResponseFail(err)
	}
}

func (p *ServerProtocol) ReadRequest(dst any) (err error) {
	p.request.GoStructRef = dst
	p.request.MiddlewareFuncs.Add(p.request.GetIOReader())
	err = p.request.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) writeResponse(data any) (err error) {
	p.response.GoStructRef = data
	p.response.MiddlewareFuncs.Add(p.response.GetIOWriter())
	err = p.response.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) ResponseFail(err error) {
	p.response.ResponseError = err
	err = p.writeResponse(nil)
	if err != nil {
		panic(err) // 业务本身报错，在写入时还报错，直接panic ，避免循环调用
	}
}

const (
	ContentTypeJson = "application/json"
)

func (c *ServerProtocol) SetResponseHeader(key string, value string) *ServerProtocol {
	c.response.SetHeader(key, value)
	return c
}

type ContextReqeustMessageKeyType string

//NewGinSerivceProtocol 这个函数注销，因为在客户端用于生成Android客户端时，不需要这个函数，尽量减少依赖

func NewGinReadWriteMiddleware(c *gin.Context) (readFn, writeFn apihttpprotocol.HandlerFunc) {
	readFn = func(message *apihttpprotocol.Message) (err error) {
		ioReader := c.Request.Body
		b, err := io.ReadAll(ioReader)
		if err != nil {
			return err
		}
		defer c.Request.Body.Close() // 关闭请求体，防止内存泄漏
		message.SetRaw(b)
		//c.Request.Body = io.NopCloser(bytes.NewReader(b))
		for k, v := range c.Request.Header {
			message.SetHeader(k, v[0])
		}
		err = json.Unmarshal(b, &message.GoStructRef)
		return err
	}
	writeFn = func(message *apihttpprotocol.Message) (err error) {
		c.JSON(http.StatusOK, message.GoStructRef)
		return nil
	}
	return readFn, writeFn
}

func NewGinHander[I, O any](proto ServerProtocol, handler func(in I) (out O, err error)) func(c *gin.Context) {
	return func(c *gin.Context) {
		proto.WithIOFn(NewGinReadWriteMiddleware(c))
		var in I
		err := proto.ReadRequest(&in)
		if err != nil {
			proto.ResponseFail(err)
			return
		}
		out, err := handler(in)
		if err != nil {
			proto.ResponseFail(err)
			return
		}
		proto.ResponseSuccess(out)
	}
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
