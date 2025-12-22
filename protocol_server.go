package apihttpprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
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
	request.GoStructRef = dst
	request.middlewareFuncs.Add(request.GetIOReader())
	err = request.Run()
	if err != nil {
		return err
	}
	return nil
}

func (p *ServerProtocol) writeResponse(data any) (err error) {
	response := p.Response()
	response.GoStructRef = data
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

var (
	ContentTypeForceJson = true // 强制json，如果请求头中没有指定ContentType为application/json,则认为是application/json (兼容历史)

)

func readInput(req *http.Request, dst any) (err error) {

	ioReader := req.Body
	var body []byte
	if ioReader != nil {
		body, err = io.ReadAll(ioReader)
		if err != nil {
			return err
		}
		ioReader.Close()                               // 关闭请求体，防止内存泄漏
		req.Body = io.NopCloser(bytes.NewReader(body)) // 重新赋值，共后续form读取二次读取
	}

	err = req.ParseForm()
	if err != nil {
		return err
	}
	m := map[string]string{}

	for k, v := range req.Form {
		m[k] = v[0] // 先简单处理，获取第一个，后续再优化处理数组情况
	}
	if len(m) > 0 {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		err = json.Unmarshal(b, dst)
		if err != nil {
			return err

		}
	}

	if len(body) > 0 {
		contentType := req.Header.Get("Content-Type") // 这里为了支持 ContentTypeForceJson ,所以先读取
		if strings.Contains(contentType, ContentTypeJson) || ContentTypeForceJson {
			err = json.Unmarshal(body, &dst) // 如果url上有和body参数同名的，会使用body的参数覆盖url上的同名参数
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func NewGinReadWriteMiddleware(c *gin.Context) (readFn HandlerFuncRequestMessage, writeFn HandlerFuncResponseMessage) {
	var contentType string
	readFn = func(message *RequestMessage) (err error) {
		err = message.SetDuplicateRequest(c.Request)
		if err != nil {
			return err
		}
		req := c.Request
		err = readInput(req, message.GoStructRef)
		if err != nil {
			return nil
		}

		for k, v := range c.Request.Header {
			message.SetHeader(k, v[0])
		}

		return err
	}
	writeFn = func(message *ResponseMessage) (err error) {
		duplicateResponse := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
		}
		if message.requestMessage != nil {
			duplicateResponse.Request, _ = message.requestMessage.GetDuplicateRequest()
		}
		requestId := message.GetRequestId()
		c.Header("X-Request-Id", requestId)
		duplicateResponse.Header.Add("X-Request-Id", requestId)
		var b []byte
		if message.GoStructRef != nil {
			if strings.Contains(contentType, ContentTypeJson) || ContentTypeForceJson {
				b, err = json.Marshal(message.GoStructRef)
				if err != nil {
					return err
				}
			}
		}

		body := string(b)

		c.String(http.StatusOK, body)

		if body != "" {
			duplicateResponse.Body = io.NopCloser(bytes.NewReader([]byte(body)))
		}

		message.SetDuplicateResponse(duplicateResponse, nil)
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
// BusinessCode = "businessCode"
)

func ResponseMiddleCodeMessageForServer(message *ResponseMessage) error {
	response := &Response{
		Code:    message.GetBusinessCode(),
		Message: message.GetBusinessMessage(),
		Data:    message.GoStructRef,
	}
	message.GoStructRef = response
	err := message.Next()
	if err != nil {
		return err
	}
	return nil
}
