// Package apihttpprotocol 提供统一的接口协议编解码、中间件处理能力
package apihttpprotocol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

const (
	MetaData_HttpCode  = "httpCode"
	MetaData_RequestID = "requestID"
)

var (
	MetaData_Response_Business_Code_Success = "0"
	MetaData_Response_Business_Code_Fail    = "1"
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
	m.Metadata.Set(MetaData_RequestID, requestId)
	return m
}
func (m *Message) GetRequestId() (requestId string) {
	requestID := cast.ToString(m.Metadata.GetWithDefault(MetaData_RequestID, "unknown"))
	return requestID
}

func (m *Message) AddMiddleware(middlewares ...HandlerFunc) *Message {
	m.MiddlewareFuncs.Add(middlewares...)
	return m
}

var ERRIOFnIsNil = errors.New("io function is nil")

// 定义Message结构体（用户提供）
type Message struct {
	Context              context.Context
	Headers              http.Header
	RequestParams        map[string]string
	raw                  []byte // 原始请求或响应数据，可用于签名校验等场景
	GoStructRef          any    // 可以用于存储请求参数或响应结果
	Metadata             Metadata
	MiddlewareFuncs      MiddlewareFuncs // 中间件调用链
	index                int             // 当前执行的中间件索引，类似Gin的index
	URL                  string          // 请求URL
	Method               string          // 请求方法
	_IOReader            HandlerFunc
	_IOWriter            HandlerFunc
	ResponseError        error   // 记录返回错误
	responseBusinessCode *string // 业务码，用于区分成功或失败，"0"表示成功，"1"表示失败,需要区分没设置，还是设置""等场景
	//requestMessage       *Message // 请求消息，用于在中间件中获取原始请求参数
}

func (m *Message) SetBusinessCode(businessCode string) {
	if businessCode == "" {
		return
	}
	m.responseBusinessCode = &businessCode
}

//	func (m *Message) GetRequestMessage() *Message {
//		return m.requestMessage
//	}
func (m *Message) GetBusinessCode() (businessCode string, exists bool) {
	if m.responseBusinessCode == nil {
		return "", false
	}
	return *m.responseBusinessCode, true
}

func (m *Message) GetBusinessCodeWithDefault(defaultCode string) string {
	if m.responseBusinessCode == nil {
		return defaultCode
	}
	return *m.responseBusinessCode
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
