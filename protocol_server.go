package apihttpprotocol

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type ServerProtocol struct {
	_Protocol
}

func NewServerProtocol() *ServerProtocol {
	p := newProtocol()
	sp := ServerProtocol{
		_Protocol: p,
	}
	return &sp
}

func (p *ServerProtocol) WithIOFn(reder HandlerFuncRequestMessage, writer HandlerFuncResponseMessage) *ServerProtocol {
	p.Request().SetIOReader(reder)
	p.Response().SetIOWriter(writer)
	return p
}

func (p *ServerProtocol) ResponseSuccess(data any) {
	err := p.writeResponse(data)
	if err != nil {
		p.ResponseFail(err)
	}
}

func (p *ServerProtocol) ReadRequest(dst any) (err error) {
	request := p.Request()
	request.goStructRef = dst
	request.middlewareFuncs.Add(request.GetIOReader())
	err = request.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) writeResponse(data any) (err error) {
	response := p.Response()
	response.goStructRef = data
	response.middlewareFuncs.Add(response.GetIOWriter())
	err = response.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) ResponseFail(err error) {
	response := p.Response()
	response.ResponseError = err
	err = p.writeResponse(nil)
	if err != nil {
		panic(err) // 业务本身报错，在写入时还报错，直接panic ，避免循环调用
	}
}

const (
	ContentTypeJson = "application/json"
)

func (p *ServerProtocol) SetResponseHeader(key string, value string) *ServerProtocol {
	response := p.Response()
	response.SetHeader(key, value)
	return p
}

type ContextReqeustMessageKeyType string

//NewGinSerivceProtocol 这个函数注销，因为在客户端用于生成Android客户端时，不需要这个函数，尽量减少依赖

func NewGinReadWriteMiddleware(c *gin.Context) (readFn HandlerFuncRequestMessage, writeFn HandlerFuncResponseMessage) {
	readFn = func(message *RequestMessage) (err error) {
		err = message.SetDuplicateRequest(c.Request)
		if err != nil {
			return err
		}
		ioReader := c.Request.Body
		b, err := io.ReadAll(ioReader)
		if err != nil {
			return err
		}
		defer c.Request.Body.Close() // 关闭请求体，防止内存泄漏
		for k, v := range c.Request.Header {
			message.SetHeader(k, v[0])
		}
		err = json.Unmarshal(b, &message.goStructRef)
		return err
	}
	writeFn = func(message *ResponseMessage) (err error) {
		c.JSON(http.StatusOK, message.goStructRef)
		message.SetDuplicateResponse(c.Request.Response, nil)
		return nil
	}
	return readFn, writeFn
}

func NewGinHander[I any, O any](protoFn func() *ServerProtocol, handler func(in I) (out O, err error)) func(c *gin.Context) {
	return func(c *gin.Context) {
		proto := protoFn() //每次请求需要重新创建协议对象，防止并发安全问题
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

func NewGinHanderCommand[I any](protoFn func() *ServerProtocol, handler func(in I) (err error)) func(c *gin.Context) {
	return func(c *gin.Context) {
		proto := protoFn() //每次请求需要重新创建协议对象，防止并发安全问题
		proto.WithIOFn(NewGinReadWriteMiddleware(c))
		var in I
		err := proto.ReadRequest(&in)
		if err != nil {
			proto.ResponseFail(err)
			return
		}
		err = handler(in)
		if err != nil {
			proto.ResponseFail(err)
			return
		}
		proto.ResponseSuccess(nil)
	}
}

var (
	Business_Code_Success = "0"
	Business_Code_Fail    = "1"
)

type Response struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func (rsp *Response) Validate() (err error) {
	if rsp.Code != Business_Code_Success {
		if rsp.Message == "" {
			rsp.Message = fmt.Sprintf("%v", rsp.Data)
		}
		err = errors.Errorf("response err code:%s,message:%s", rsp.Code, rsp.Message)
		return err
	}
	return nil
}

var (
	BusinessCode = "businessCode"
)

func ResponseMiddleCodeMessageForServer(message *ResponseMessage) error {
	response := &Response{
		Code:    Business_Code_Success,
		Message: "success",
		Data:    message.goStructRef,
	}
	err := message.ResponseError
	if err != nil {
		response.Code = Business_Code_Fail
		response.Message = err.Error()
	}
	businessCode, exists := message.metaData.Get(BusinessCode)
	if exists {
		response.Code = cast.ToString(businessCode)
	}
	message.goStructRef = response
	err = message.Next()
	if err != nil {
		return err
	}
	return nil
}
