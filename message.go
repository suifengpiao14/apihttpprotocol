// Package apihttpprotocol 提供统一的接口协议编解码、中间件处理能力
package apihttpprotocol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"moul.io/http2curl"
)

const (
	MetaData_HttpCode = "httpCode"
	//MetaData_RequestID = "requestID"
)

type Metadata map[string]any

func (m *Metadata) GetWithDefault(key string, defau any) any {
	v, exists := m.Get(key)
	if !exists {
		return defau
	}
	return v
}

func (m *Metadata) Get(key string) (value any, exists bool) {
	if m == nil || *m == nil {
		return nil, false
	}
	v, ok := (*m)[key]
	return v, ok
}

func (m *Metadata) Set(key string, value any) {
	if m == nil {
		return
	}
	if *m == nil {
		*m = make(map[string]any)
	}
	(*m)[key] = value
}

func (m *Message) SetMetaData(key string, value any) {
	m.Metadata.Set(key, value)
}
func (m *Message) SetHeader(key string, value string) {
	if m.Headers == nil {
		m.Headers = http.Header{}
	}
	m.Headers.Add(key, value)
}

func (m *Message) GetHeader(key string) (value string) {
	if m.Headers == nil {
		m.Headers = http.Header{}
	}
	return m.Headers.Get(key)
}
func (m *Message) SetRequestId(requestId string) *Message {
	m.requestId = requestId
	return m
}
func (m *Message) GetRequestId() (requestId string) {
	defer func() {
		if requestId == "" {
			requestId = "unknown" // 将空值转换为未知，便于调试追踪问题。
		}
	}()
	if m.requestId != "" {
		return ""
	}
	dumpReq, ok := m.GetDuplicateRequest()
	if !ok {
		return ""
	}
	requestId = dumpReq.Header.Get("X-Request-Id")
	return requestId
}

func (m *Message) AddMiddleware(middlewares ...HandlerFunc) *Message {
	m.MiddlewareFuncs.Add(middlewares...)
	return m
}

var ERRIOFnIsNil = errors.New("io function is nil")

// 定义Message结构体（用户提供）
type Message struct {
	Context context.Context
	Headers http.Header
	//RequestParams   map[string]string
	raw               []byte // 原始请求或响应数据，可用于签名校验等场景
	GoStructRef       any    // 可以用于存储请求参数或响应结果
	Metadata          Metadata
	MiddlewareFuncs   MiddlewareFuncs // 中间件调用链
	index             int             // 当前执行的中间件索引，类似Gin的index
	URL               string          // 请求URL
	Method            string          // 请求方法
	_IOReader         HandlerFunc
	_IOWriter         HandlerFunc
	ResponseError     error    // 记录返回错误
	requestMessage    *Message // 请求消息，用于在中间件中获取原始请求参数(在response里面,这个参数才有值)
	log               LogI
	duplicateRequest  *http.Request
	duplicateResponse *http.Response
	requestId         string
}

func (m *Message) GetDuplicateRequest() (duplicateRequest *http.Request, exists bool) {
	if m.duplicateRequest == nil {
		return nil, false
	}
	return m.duplicateRequest, true
}

func (m *Message) GetDuplicateResponse() (duplicateResponse *http.Response, exists bool) {
	if m.duplicateResponse == nil {
		return nil, false
	}
	return m.duplicateResponse, true
}

func (m *Message) SetDuplicateRequest(reqest *http.Request) (err error) {
	duplicateRequest, err := CopyRequest(reqest)
	if err != nil {
		return err
	}
	m.duplicateRequest = duplicateRequest
	return nil
}

func (m *Message) SetDuplicateResponse(response *http.Response) (err error) {
	duplicateResponse, err := CopyResponse(response)
	if err != nil {
		return err
	}
	m.duplicateResponse = duplicateResponse
	return nil
}

func (m *Message) SetLog(log LogI) *Message {
	m.log = log
	return m
}
func (m *Message) GetLog() (l LogI) {
	if m.log == nil {
		m.log = &logDefault{}
	}
	return m.log
}

func (m *Message) SetRequestMessage(requestMsg *Message) *Message {
	m.requestMessage = requestMsg
	return m
}

func (m *Message) GetRequestMessage() (*Message, bool) {
	return m.requestMessage, m.requestMessage != nil
}

func (m *Message) SetIOReader(ioFn HandlerFunc) *Message {
	m._IOReader = ioFn
	return m
}
func (m *Message) SetRaw(b []byte) {
	m.raw = b
}
func (m *Message) GetRaw() []byte {
	return m.raw
}
func (m *Message) GetIOReader() (ioFn HandlerFunc) {
	if m._IOReader == nil {
		panic(ERRIOFnIsNil)
	}
	return m._IOReader
}

func (m *Message) SetIOWriter(ioFn HandlerFunc) *Message {
	m._IOWriter = ioFn
	return m
}
func (m *Message) GetIOWriter() (ioFn HandlerFunc) {
	if m._IOWriter == nil {
		panic(ERRIOFnIsNil)
	}
	return m._IOWriter
}

func (m *Message) Back() *Message {
	m.index--
	return m
}

// // 元数据结构体（示例）
// type Metadata struct {
// 	RequestID string
// 	Timestamp int64
// }

// 定义中间件函数类型，与Gin的HandlerFunc对应
type HandlerFunc func(message *Message) (err error)

// Next 传递控制权给下一个中间件
// 实现逻辑：索引+1并执行下一个中间件
func (m *Message) Next() (err error) {
	m.index++
	if m.index < len(m.MiddlewareFuncs) {
		err = m.MiddlewareFuncs[m.index](m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) Run() (err error) {
	m.index = -1
	err = m.Next()
	return err
}

type MiddlewareFuncs []HandlerFunc

func (ms *MiddlewareFuncs) Add(fns ...HandlerFunc) *MiddlewareFuncs {
	if *ms == nil {
		*ms = MiddlewareFuncs{}
	}
	*ms = append(*ms, fns...)
	return ms
}

func RequestMiddleLog(message *Message) (err error) {
	err = message.Next() //读取数据后
	if err != nil {
		return err
	}
	duplicateReq, ok := message.GetDuplicateRequest()
	if !ok {
		return nil
	}
	curlCommand, err1 := http2curl.GetCurlCommand(duplicateReq)
	if err1 != nil {
		message.GetLog().Error("http2curl.GetCurlCommand", err1)
	}
	requestId := message.GetRequestId()
	msg := fmt.Sprintf("request: requestId:%s;curlCommand: %s", requestId, curlCommand.String())
	message.GetLog().Info(msg)

	return nil
}

func ResponseMiddleLog(message *Message) (err error) {
	err = message.Next() //读取数据后
	if err != nil {
		return err
	}
	duplicateRsp, ok := message.GetDuplicateResponse()
	if !ok {
		return nil
	}
	var body []byte
	if duplicateRsp.Body != nil {
		defer duplicateRsp.Body.Close()
		body, _ = io.ReadAll(duplicateRsp.Body)
	}
	msg := fmt.Sprintf("response: httpCode: %d;body:%s", duplicateRsp.StatusCode, string(body))
	message.GetLog().Info(msg)

	return nil
}

func MiddleSetLog(log LogI) HandlerFunc {
	return func(message *Message) (err error) {
		message.SetLog(log)
		return message.Next()
	}
}

var ERRMiddlewareNotFound = errors.New("middleware not found")

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func (rsp *Response) Validate() (err error) {
	if rsp.Code != 0 {
		if rsp.Message == "" {
			rsp.Message = fmt.Sprintf("%v", rsp.Data)
		}
		err = errors.Errorf("response err code:%d,message:%s", rsp.Code, rsp.Message)
		return err
	}
	return nil
}

// 错误处理结构

type HttpError struct {
	HttpStatus string `json:"httpStatus"`
	HttpBody   string `json:"httpBody"`
}

func (e HttpError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

type LogI interface {
	Debug(v ...any)
	Info(v ...any)
	Warn(v ...any)
	Error(v ...any)
}

type logDefault struct {
}

func (l logDefault) Debug(v ...any) {
	fmt.Println(v...)
}
func (l logDefault) Info(v ...any) {
	fmt.Println(v...)
}

func (l logDefault) Warn(v ...any) {
	fmt.Println(v...)
}
func (l logDefault) Error(v ...any) {
	fmt.Println(v...)
}

// 忽略日志记录的中间件,可用于屏蔽大型日志记录
type LogIgnore struct {
}

func (l LogIgnore) Debug(v ...any) {

}
func (l LogIgnore) Info(v ...any) {

}

func (l LogIgnore) Warn(v ...any) {

}
func (l LogIgnore) Error(v ...any) {
}
