package serverprotocol

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
)

type ServerProtocol struct {
	Request  apihttpprotocol.Message
	Response apihttpprotocol.Message
}

func NewServerProtocol(readFn apihttpprotocol.IOFn, writeFn apihttpprotocol.IOFn) *ServerProtocol {
	p := &ServerProtocol{}
	p.WithReadIoFn(readFn).WithWriteIoFn(writeFn)
	return p
}

func (p *ServerProtocol) WithWriteIoFn(ioFn apihttpprotocol.IOFn) *ServerProtocol {
	p.Response.SetIOWriter(ioFn)
	return p
}

func (p *ServerProtocol) WithReadIoFn(ioFn apihttpprotocol.IOFn) *ServerProtocol {
	p.Request.SetIOReader(ioFn)
	return p
}
func (p *ServerProtocol) AddRequestMiddleware(middlewares ...apihttpprotocol.MiddlewareFunc) *ServerProtocol {
	p.Request.AddMiddleware(middlewares...)
	return p
}

func (p *ServerProtocol) AddResponseMiddleware(middlewares ...apihttpprotocol.MiddlewareFunc) *ServerProtocol {
	p.Response.AddMiddleware(middlewares...)
	return p
}

func (p *ServerProtocol) ResponseSuccess(data any) {
	err := p.WriteResponse(data)
	if err != nil {
		p.ResponseFail(err)
	}
}

func (p *ServerProtocol) ReadRequest(dst apihttpprotocol.ValidateI) (err error) {
	if err := p.Request.HasIOReder(); err != nil {
		err = errors.WithMessagef(err, "read request struct %v", p.Request.GoStructRef)
		return err
	}
	p.Request.GoStructRef = dst
	err = p.Request.MiddlewareFuncs.Apply(&p.Request)
	if err != nil {
		return err
	}
	err = dst.Validate()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) WriteResponse(data any) (err error) {
	if err := p.Response.HasIOWriter(); err != nil {
		err = errors.WithMessagef(err, "write response struct %v", p.Request.GoStructRef)
		return err
	}
	p.Response.GoStructRef = data
	err = p.Response.MiddlewareFuncs.Apply(&p.Response)
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) ResponseFail(data any) {
	p.Response.SetMetaData(apihttpprotocol.MetaData_Code, apihttpprotocol.MetaData_Code_Fail)
	err := p.WriteResponse(data)
	if err != nil {
		panic(err) // 业务本身报错，在写入时还报错，直接panic ，避免循环调用
	}
}

func (p *ServerProtocol) SetBusineesCode(code int) *ServerProtocol {
	if p.Response.Metadata == nil {
		p.Response.Metadata = &apihttpprotocol.Metadata{}
	}
	p.Response.SetMetaData(apihttpprotocol.MetaData_Code, code)
	return p
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

//NewGinSerivceProtocol 这个函数注销，因为在客户端用于生成Android客户端时，不需要这个函数，尽量减少依赖

func NewGinSerivceProtocol(c *gin.Context) *ServerProtocol {
	readFn := func(message *apihttpprotocol.Message) (err error) {
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

// ProtocolMiddlewareCodeMessage 封装返回体code/message 协议
func ServerProtocolMiddlewareCodeMessage(p *ServerProtocol) *ServerProtocol {
	protocol := p.AddResponseMiddleware(apihttpprotocol.MakeMiddlewareFuncWriteData(func(message *apihttpprotocol.Message) error {
		code := cast.ToInt(message.GetMetaData(apihttpprotocol.MetaData_Code, apihttpprotocol.MetaData_Code_Success))
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
