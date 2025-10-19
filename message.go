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

type MetaData map[string]any

func (m *MetaData) GetWithDefault(key string, defau any) any {
	v, exists := m.Get(key)
	if !exists {
		return defau
	}
	return v
}

func (m *MetaData) Get(key string) (value any, exists bool) {
	if m == nil || *m == nil {
		return nil, false
	}
	v, ok := (*m)[key]
	return v, ok
}

func (m *MetaData) Set(key string, value any) {
	if m == nil {
		return
	}
	if *m == nil {
		*m = make(map[string]any)
	}
	(*m)[key] = value
}

func (m *Message[T]) SetMetaData(key string, value any) {
	m.metaData.Set(key, value)
}
func (m *Message[T]) SetHeader(key string, value string) {
	if m.headers == nil {
		m.headers = http.Header{}
	}
	m.headers.Add(key, value)
}

func (m *Message[T]) GetHeader(key string) (value string) {
	if m.headers == nil {
		m.headers = http.Header{}
	}
	return m.headers.Get(key)
}
func (m *Message[T]) SetRequestId(requestId string) *Message[T] {
	m.requestId = requestId
	return m
}
func (m *RequestMessage) GetRequestId() (requestId string) {
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

func (m *ResponseMessage) GetRequestId() (requestId string) {
	defer func() {
		if requestId == "" {
			requestId = "unknown" // 将空值转换为未知，便于调试追踪问题。
		}
	}()
	if m.requestId != "" {
		return ""
	}
	dumpResp, ok := m.GetDuplicateResponse()
	if !ok {
		return ""
	}
	dumpReq := dumpResp.Request
	if dumpReq != nil {
		requestId = dumpReq.Header.Get("X-Request-Id")
	} else {
		requestId = dumpResp.Header.Get("X-Request-Id")
	}
	return requestId
}

func (m *Message[T]) AddMiddleware(middlewares ...HandlerFunc[T]) *Message[T] {
	m.middlewareFuncs.Add(middlewares...)
	return m
}

var ERRIOFnIsNil = errors.New("io function is nil")

// 定义Message结构体（用户提供）
type Message[T any] struct {
	self    *T
	context context.Context // 上下文信息，例如请求ID、用户信息等
	headers http.Header
	//RequestParams   map[string]string
	bodyBtyes       []byte             // 原始请求或响应数据，可用于签名校验等场景
	goStructRef     any                // 可以用于存储请求参数或响应结果
	metaData        MetaData           // 存储一些额外的信息，例如请求ID、用户信息等
	middlewareFuncs MiddlewareFuncs[T] // 中间件调用链
	index           int                // 当前执行的中间件索引，类似Gin的index

	_IOReader HandlerFunc[T]
	_IOWriter HandlerFunc[T]
	log       LogI
	requestId string
}

func (m *Message[T]) Self() *T {
	return m.self
}

// 增加messageString 是因为Message 要尽量少对外暴露的字段，所以增加一个内部结构体来做转换。
type messageString struct {
	Headers     http.Header `json:"headers"`
	GoStructRef any         `json:"goStructRef"`
	MetaData    MetaData    `json:"metaData"`
	RequestId   string      `json:"requestId"`
}

func (m *Message[T]) toStringStruct() messageString {
	return messageString{
		Headers:     m.headers,
		GoStructRef: m.goStructRef,
		MetaData:    m.metaData,
		RequestId:   m.requestId,
	}
}

func (m *Message[T]) String() string {
	mstr := m.toStringStruct()
	b, err := json.Marshal(mstr)
	if err != nil {
		return err.Error()
	}
	s := string(b)
	return s
}

type RequestMessage struct {
	Message[RequestMessage]
	URL              string           `json:"url"`    // 请求URL
	Method           string           `json:"method"` // 请求方法
	responseMessage  *ResponseMessage // 响应消息，用于在中间件中获取原始请求参数(在response里面,这个参数才有值)
	duplicateRequest *http.Request
}

type ResponseMessage struct {
	Message[ResponseMessage]
	ResponseError     error           // 记录返回错误
	requestMessage    *RequestMessage // 请求消息，用于在中间件中获取原始请求参数(在response里面,这个参数才有值)
	duplicateResponse *http.Response
}

func (m *RequestMessage) GetDuplicateRequest() (duplicateRequest *http.Request, exists bool) {
	if m.duplicateRequest == nil {
		return nil, false
	}
	duplicateRequest, err := CopyRequest(m.duplicateRequest) //复制请求，防止后续修改影响原始请求
	if err != nil {
		return nil, false
	}
	return duplicateRequest, true
}

func (m *RequestMessage) SetDuplicateRequest(reqest *http.Request) (err error) {
	duplicateRequest, err := CopyRequest(reqest)
	if err != nil {
		return err
	}
	m.duplicateRequest = duplicateRequest
	return nil
}

type requestMessageString struct {
	messageString
	URL    string `json:"url"`
	Method string `json:"method"`
}

func (m *RequestMessage) toStringStruct() requestMessageString {
	return requestMessageString{
		messageString: m.Message.toStringStruct(),
		URL:           m.URL,
		Method:        m.Method,
	}
}

func (m *RequestMessage) String() string {
	mstr := m.toStringStruct()
	b, err := json.Marshal(mstr)
	if err != nil {
		err = errors.WithMessage(err, "RequestMessage")
		return err.Error()
	}
	s := string(b)
	return s
}

func (m *ResponseMessage) GetDuplicateResponse() (duplicateResponse *http.Response, exists bool) {
	if m.duplicateResponse == nil {
		return nil, false
	}
	duplicateResponse, err := CopyResponse(m.duplicateResponse, m.bodyBtyes)
	if err != nil {
		return nil, false
	}
	return duplicateResponse, true
}

func (m *ResponseMessage) SetDuplicateResponse(response *http.Response, body []byte) (err error) { // 这里显示传入原始的body，便于使用者明确传递，如果直接从m.raw读取，使用者需要先了解逻辑，先设置m.raw,再使用，比较复杂
	if body != nil {
		m.bodyBtyes = body
	}
	duplicateResponse, err := CopyResponse(response, m.bodyBtyes)
	if err != nil {
		return err
	}
	m.duplicateResponse = duplicateResponse
	return nil
}

func (m *ResponseMessage) String() string {
	mstr := m.toStringStruct()
	b, err := json.Marshal(mstr)
	if err != nil {
		err = errors.WithMessage(err, "ResponseMessage")
		return err.Error()
	}
	s := string(b)
	return s
}

func (m *Message[T]) SetLog(log LogI) *Message[T] {
	m.log = log
	return m
}
func (m *Message[T]) GetLog() (l LogI) {
	if m.log == nil {
		m.log = &logDefault{}
	}
	return m.log
}

// func (m *ResponseMessage) SetRequestMessage(requestMsg *RequestMessage) *ResponseMessage {
// 	m.requestMessage = requestMsg
// 	return m
// }

func (m *ResponseMessage) GetRequestMessage() (*RequestMessage, bool) {
	return m.requestMessage, m.requestMessage != nil
}

// func (m *RequestMessage) SetResponseMessage(responseMsg *ResponseMessage) *RequestMessage {
// 	m.responseMessage = responseMsg
// 	return m
// }

func (m *RequestMessage) GetResponseMessage() (*ResponseMessage, bool) {
	return m.responseMessage, m.responseMessage != nil
}

func (m *Message[T]) SetIOReader(ioFn HandlerFunc[T]) *Message[T] {
	m._IOReader = ioFn
	return m
}

func (m *Message[T]) SetRaw(b []byte) {
	m.bodyBtyes = b
}

func (m *Message[T]) GetRaw() []byte {
	if m.bodyBtyes != nil {
		return m.bodyBtyes
	}

	return m.bodyBtyes
}
func (m *Message[T]) GetIOReader() (ioFn HandlerFunc[T]) {
	if m._IOReader == nil {
		panic(ERRIOFnIsNil)
	}
	return m._IOReader
}

func (m *Message[T]) SetIOWriter(ioFn HandlerFunc[T]) *Message[T] {
	m._IOWriter = ioFn
	return m
}
func (m *Message[T]) GetIOWriter() (ioFn HandlerFunc[T]) {
	if m._IOWriter == nil {
		panic(ERRIOFnIsNil)
	}
	return m._IOWriter
}

func (m *Message[T]) Back() *Message[T] {
	m.index--
	return m
}

// // 元数据结构体（示例）
// type Metadata struct {
// 	RequestID string
// 	Timestamp int64
// }

// 定义中间件函数类型，与Gin的HandlerFunc对应
type HandlerFunc[T any] func(message *T) (err error)

type HandlerFuncRequestMessage = HandlerFunc[RequestMessage]
type HandlerFuncResponseMessage = HandlerFunc[ResponseMessage]

// Next 传递控制权给下一个中间件
// 实现逻辑：索引+1并执行下一个中间件
func (m *Message[T]) Next() (err error) {
	m.index++
	if m.index < len(m.middlewareFuncs) {
		fn := m.middlewareFuncs[m.index]
		if fn == nil {
			return m.Next() //如果当前fn为空，则继续执行下一个fn
		}
		err = fn(m.self)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Message[T]) Run() (err error) {
	m.index = -1
	err = m.Next()
	return err
}

type MiddlewareFuncs[T any] []HandlerFunc[T]

type MiddlewareFuncsRequestMessage = MiddlewareFuncs[RequestMessage]
type MiddlewareFuncsResponseMessage = MiddlewareFuncs[ResponseMessage]

func (ms *MiddlewareFuncs[T]) Add(fns ...HandlerFunc[T]) *MiddlewareFuncs[T] {
	if *ms == nil {
		*ms = MiddlewareFuncs[T]{}
	}
	arr := make([]HandlerFunc[T], 0)
	for _, fn := range fns {
		if fn != nil {
			arr = append(arr, fn)
		}
	}
	*ms = append(*ms, arr...)
	return ms
}

func RequestMiddleLog(message *RequestMessage) (err error) {
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
	msg := fmt.Sprintf("url:%s,request: requestId:%s;curlCommand: %s", duplicateReq.URL.String(), requestId, curlCommand.String())
	message.GetLog().Info(msg)

	return nil
}

func ResponseMiddleLog(message *ResponseMessage) (err error) {
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
	req := duplicateRsp.Request
	msg := fmt.Sprintf("url:%s,response: httpCode: %d;body:%s", req.URL.String(), duplicateRsp.StatusCode, string(body))
	message.GetLog().Info(msg)

	return nil
}

func RequestMiddleSetLog(log LogI) HandlerFunc[RequestMessage] {
	return func(message *RequestMessage) (err error) {
		message.SetLog(log)
		err = message.Next()
		return err
	}
}

func ResponseMiddleSetLog(log LogI) HandlerFunc[ResponseMessage] {
	return func(message *ResponseMessage) (err error) {
		message.SetLog(log)
		err = message.Next()
		return err
	}
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
