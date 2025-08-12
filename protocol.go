// Package apihttpprotocol 提供统一的接口协议编解码、中间件处理能力
package apihttpprotocol

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/pkg/errors"
)

type IOFn = ApplyFn

const (
	MetaData_Code = "code"
)

var (
	MetaData_Code_Success = 0
	MetaData_Code_Fail    = 1
)

type Metadata map[string]any

func (m *Metadata) Get(key string, defau any) any {
	if m == nil || *m == nil {
		return defau
	}
	v, ok := (*m)[key]
	if !ok {
		return defau
	}
	return v
}

func (m *Metadata) Set(key string, value any) {
	if m == nil {
		return
	}
	(*m)[key] = value
}

// 通用请求/响应结构体

type Message struct {
	Context     context.Context
	Headers     map[string]string
	GoStructRef any
	raw         string // 记录原始报文，方便中间件、日志等使用
	Metadata    *Metadata

	MiddlewareFuncs MiddlewareFuncs
	ReadIOFn        IOFn
	WriteIOFn       IOFn
}

func (m *Message) GetRaw() string {
	if m.raw != "" {
		return m.raw
	}
	if m.GoStructRef == nil {
		return ""
	}
	b, err := json.Marshal(m.GoStructRef)
	if err != nil {
		panic(err)
	}
	m.raw = string(b)
	return m.raw
}
func (m *Message) SetRaw(raw string) {
	m.raw = raw
}

func (m *Message) SetMetaData(key string, value any) {
	if m.Metadata == nil {
		m.Metadata = &Metadata{}
	}
	m.Metadata.Set(key, value)
}
func (m *Message) SetHeader(key string, value string) {
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	m.Headers[key] = value
}

func (m *Message) GetHeader(key string) (value string) {
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	value = m.Headers[key]
	return value
}

func (m *Message) GetMetaData(key string, defau any) any {
	if m.Metadata == nil {
		return defau
	}
	return m.Metadata.Get(key, defau)
}

// 协议封装

type ProtocolMiddleware = func(p *Protocol) *Protocol
type Protocol struct {
	Request  Message
	Response Message
}

func (p *Protocol) Apply(fn ProtocolMiddleware) *Protocol {
	p = fn(p)
	return p
}

type ServerProtocol struct {
	Protocol
}

func NewServerProtocol(readFn IOFn, writeFn IOFn) *ServerProtocol {
	p := &Protocol{}
	p = p.WithServerIoFn(readFn, writeFn)
	protocol := &ServerProtocol{
		Protocol: *p,
	}
	return protocol
}

type ClientProtocol struct {
	Protocol
}

func NewClitentProtocol(readFn IOFn, writeFn IOFn) *ClientProtocol {
	p := &Protocol{}
	p = p.WithClientIoFn(readFn, writeFn)
	protocol := &ClientProtocol{
		Protocol: *p,
	}
	return protocol
}

func (c *ClientProtocol) SetContentType(contentType string) *ClientProtocol {
	c.Request.SetHeader("Content-Type", contentType)
	return c
}
func (c *ClientProtocol) SetContentTypeJson() *ClientProtocol {
	contentType := "application/json"
	c.SetContentType(contentType)
	return c
}

//NewGinSerivceProtocol 这个函数注销，因为在客户端用于生成Android客户端时，不需要这个函数，尽量减少依赖

// func NewGinSerivceProtocol(c *gin.Context) *Protocol {
// 	readFn := func(message *Message) (err error) {
// 		err = c.BindJSON(message.GoStructRef)
// 		return err
// 	}
// 	writeFn := func(message *Message) (err error) {
// 		c.JSON(http.StatusOK, message.GoStructRef)
// 		return nil
// 	}
// 	protocol := NewServerProtocol(readFn, writeFn)
// 	return protocol
// }

func (p *Protocol) WithServerIoFn(readIOFn IOFn, writeIOFn IOFn) *Protocol {
	p.Request.ReadIOFn = readIOFn
	p.Response.WriteIOFn = writeIOFn
	return p
}
func (p *Protocol) WithClientIoFn(readIOFn IOFn, writeIOFn IOFn) *Protocol {
	p.Response.ReadIOFn = readIOFn
	p.Request.WriteIOFn = writeIOFn
	return p
}

func (p *Protocol) WithRequestReadIoFn(ioFn IOFn) *Protocol {
	p.Request.ReadIOFn = ioFn
	return p
}
func (p *Protocol) WithRequestWriteIoFn(ioFn IOFn) *Protocol {
	p.Request.WriteIOFn = ioFn
	return p
}

func (p *Protocol) WithResponseReadIoFn(ioFn IOFn) *Protocol {
	p.Response.ReadIOFn = ioFn
	return p
}
func (p *Protocol) WithResponseWriteIoFn(ioFn IOFn) *Protocol {
	p.Response.WriteIOFn = ioFn
	return p
}

func (p *Protocol) SetBusineesCode(code int) *Protocol {
	if p.Response.Metadata == nil {
		p.Response.Metadata = &Metadata{}
	}
	p.Response.SetMetaData(MetaData_Code, code)
	return p
}

var ERRIOFnIsNil = errors.New("io function is nil")

func (p *Protocol) ReadRequestWithEmptyValidate(readStructRef any) (err error) {
	readStructr := ValidateEmpty{
		ReaderStruct: readStructRef,
	}
	err = p.ReadRequest(&readStructr)
	return err
}
func (p *Protocol) ReadRequest(readStructRef ValidateI) (err error) {
	readIOFn := p.Request.ReadIOFn
	if readIOFn == nil {
		err = errors.WithMessagef(ERRIOFnIsNil, "read request struct %v", p.Request.GoStructRef)
		return err
	}
	p.Request.GoStructRef = readStructRef
	err = p.Request.MiddlewareFuncs.Apply(&p.Request)
	if err != nil {
		return err
	}
	err = readIOFn(&p.Request)
	if err != nil {
		return err
	}
	err = readStructRef.Validate()
	if err != nil {
		return err
	}
	return nil
}

func (p *Protocol) WriteResponse(writeStruct any) (err error) {
	writeIOFn := p.Response.WriteIOFn
	if writeIOFn == nil {
		err := errors.WithMessagef(ERRIOFnIsNil, "write response struct %v", p.Response.GoStructRef)
		return err
	}
	p.Response.GoStructRef = writeStruct
	err = p.Response.MiddlewareFuncs.Apply(&p.Response)
	if err != nil {
		return err
	}
	writeIOFn(&p.Response) // 写入响应数据，但不返回错误信息
	return nil
}

func (p *Protocol) ResponseSuccess(data any) {
	err := p.WriteResponse(data)
	if err != nil {
		p.ResponseFail(err)
	}
}
func (p *Protocol) ResponseFail(data any) {
	p.Response.SetMetaData(MetaData_Code, MetaData_Code_Fail)

	err := p.WriteResponse(data)
	if err != nil {
		panic(err) // 业务本身报错，在写入时还报错，直接panic ，避免循环调用
	}
}

// ValidateI 定义了 Error 方法，用于在读取结构体后获取可能的错误信息。(比如返回体errCode 校验,request 的参数校验等)
type ValidateI interface {
	Validate() error
}
type ValidateEmpty struct {
	ReaderStruct any
}

func (r *ValidateEmpty) Validate() error { return nil }

func (p *Protocol) WriteRequest(writeStruct any) (err error) {
	writeIOFn := p.Request.WriteIOFn
	if writeIOFn == nil {
		err = errors.WithMessagef(ERRIOFnIsNil, "write request struct %v", p.Request.GoStructRef)
		return err
	}
	p.Request.GoStructRef = writeStruct
	err = p.Request.MiddlewareFuncs.Apply(&p.Request)
	if err != nil {
		return err
	}
	err = writeIOFn(&p.Request)
	if err != nil {
		return err
	}
	return nil
}

func (p *Protocol) ReadResponse(readStructRef ValidateI) (err error) {
	readIOFn := p.Response.ReadIOFn
	if readIOFn == nil {
		err = errors.WithMessagef(ERRIOFnIsNil, "read response struct %v", p.Response.GoStructRef)
		return err
	}
	p.Response.GoStructRef = readStructRef
	err = p.Response.MiddlewareFuncs.Apply(&p.Response)
	if err != nil {
		return err
	}
	err = readIOFn(&p.Response)
	if err != nil {
		return err
	}
	err = readStructRef.Validate()
	if err != nil {
		return err
	}
	return nil
}

// 中间件结构

type Stage string

const (
	Stage_befor_send_data Stage = "stage_beforSend_data"
	Stage_write_data      Stage = "stage_write_data"
	Stage_read_data       Stage = "stage_read_data"

	OrderMax = 999999
	OrderMin = 1
)

// 默认中间件执行顺序，order 越小，越先执行

func (s Stage) Order() int {
	m := map[Stage]int{
		Stage_befor_send_data: OrderMax,
		Stage_write_data:      OrderMin,
		Stage_read_data:       OrderMin,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return 0
}

type ApplyFn func(message *Message) error
type MiddlewareFunc struct {
	Order int
	Stage Stage
	Fn    ApplyFn
}

func MakeMiddlewareFunc(order int, stage Stage, fn ApplyFn) MiddlewareFunc {
	return MiddlewareFunc{
		Order: order,
		Stage: stage,
		Fn:    fn,
	}
}
func MakeMiddlewareFuncWriteData(fn ApplyFn) MiddlewareFunc {
	return MiddlewareFunc{
		Order: OrderMin,
		Stage: Stage_write_data,
		Fn:    fn,
	}
}
func MakeMiddlewareFuncReadData(fn ApplyFn) MiddlewareFunc {
	return MiddlewareFunc{
		Order: OrderMin,
		Stage: Stage_write_data,
		Fn:    fn,
	}
}

type MiddlewareFuncs []MiddlewareFunc

func (ms MiddlewareFuncs) Sort() {
	slices.SortFunc(ms, func(a, b MiddlewareFunc) int {
		if d := b.Stage.Order() - a.Stage.Order(); d != 0 {
			return d
		}
		return b.Order - a.Order
	})
}

func (m MiddlewareFunc) Apply(p *Message) error {
	if m.Fn == nil {
		return nil
	}
	return m.Fn(p)
}

func (ms MiddlewareFuncs) Apply(p *Message) error {
	ms.Sort()
	for _, m := range ms {
		err := m.Fn(p)
		if err != nil {
			return err
		}
	}
	return nil
}
func (ms *MiddlewareFuncs) Add(fns ...MiddlewareFunc) *MiddlewareFuncs {
	if *ms == nil {
		*ms = MiddlewareFuncs{}
	}
	*ms = append(*ms, fns...)
	return ms
}

// 中间件设置简化方法

func (r *Protocol) AddRequestMiddleware(fns ...MiddlewareFunc) *Protocol {
	r.Request.MiddlewareFuncs.Add(fns...)
	return r
}

func (r *Protocol) AddResponseMiddleware(fns ...MiddlewareFunc) *Protocol {
	r.Response.MiddlewareFuncs.Add(fns...)

	return r
}
