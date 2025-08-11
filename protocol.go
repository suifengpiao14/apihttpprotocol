// Package apihttpprotocol 提供统一的接口协议编解码、中间件处理能力
package apihttpprotocol

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/pkg/errors"
)

type IOFn func(message *Message) (err error)

// 通用请求/响应结构体

type Message struct {
	Header      http.Header
	GoStructRef any
	raw         string // 记录原始报文，方便中间件、日志等使用

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

// 协议封装

type Protocol struct {
	Request  Message
	Response Message
}

func NewServerProtocol(readFn IOFn, writeFn IOFn) *Protocol {
	protocol := &Protocol{
		Request: Message{
			ReadIOFn: readFn,
		},
		Response: Message{
			WriteIOFn: writeFn,
		},
	}
	return protocol
}

func NewClitentProtocol(readFn IOFn, writeFn IOFn) *Protocol {
	protocol := &Protocol{
		Request: Message{
			WriteIOFn: writeFn,
		},
		Response: Message{
			ReadIOFn: readFn,
		},
	}
	return protocol
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

func (p *Protocol) WithRequestRederIoFn(ioFn IOFn) *Protocol {
	p.Request.ReadIOFn = ioFn
	return p
}
func (p *Protocol) WithRequestWriteIoFn(ioFn IOFn) *Protocol {
	p.Request.WriteIOFn = ioFn
	return p
}

func (p *Protocol) WithResponseRederIoFn(ioFn IOFn) *Protocol {
	p.Response.ReadIOFn = ioFn
	return p
}
func (p *Protocol) WithResponseWriteIoFn(ioFn IOFn) *Protocol {
	p.Response.WriteIOFn = ioFn
	return p
}

var ERRIOFnIsNil = errors.New("io function is nil")

func (p *Protocol) ReadRequest(readStructRef ReaderStructI) (err error) {
	readIOFn := p.Request.ReadIOFn
	if readIOFn == nil {
		err = errors.WithMessagef(ERRIOFnIsNil, "read request struct %v", p.Request.GoStructRef)
		return err
	}
	p.Request.GoStructRef = readStructRef
	err = readIOFn(&p.Request)
	if err != nil {
		return err
	}
	err = readStructRef.Error()
	if err != nil {
		return err
	}
	return nil
}

func (p *Protocol) WriteResponse(writeStruct any) {
	writeIOFn := p.Request.WriteIOFn
	if writeIOFn == nil {
		err := errors.WithMessagef(ERRIOFnIsNil, "write response struct %v", p.Request.GoStructRef)
		panic(err)
	}
	p.Request.GoStructRef = writeStruct
	writeIOFn(&p.Request) // 写入响应数据，但不返回错误信息

}

// ReaderStructI 定义了 Error 方法，用于在读取结构体后获取可能的错误信息。(比如返回体errCode 校验,request 的参数校验等)
type ReaderStructI interface {
	Error() error
}
type ReaderStructAny struct {
	ReaderStruct any
}

func (r *ReaderStructAny) Error() error { return nil }

func (p *Protocol) WriteRequest(writeStruct any) (err error) {
	writeIOFn := p.Request.WriteIOFn
	if writeIOFn == nil {
		err = errors.WithMessagef(ERRIOFnIsNil, "write request struct %v", p.Request.GoStructRef)
		return err
	}
	p.Request.GoStructRef = writeStruct
	err = writeIOFn(&p.Request)
	if err != nil {
		return err
	}
	return nil
}

func (p *Protocol) ReadResponse(readStructRef ReaderStructI) (err error) {
	readIOFn := p.Response.ReadIOFn
	if readIOFn == nil {
		err = errors.WithMessagef(ERRIOFnIsNil, "read response struct %v", p.Request.GoStructRef)
		return err
	}
	p.Response.GoStructRef = readStructRef
	err = readIOFn(&p.Request)
	if err != nil {
		return err
	}
	err = readStructRef.Error()
	if err != nil {
		return err
	}
	return nil
}

// 中间件结构

type Stage string

const (
	Stage_befor_send_data Stage = "stage_beforSend_data"
	Stage_set_data        Stage = "set_data"
	Stage_recive_data     Stage = "recive_data"

	OrderMax = 999999
	OrderMin = 1
)

func (s Stage) Order() int {
	m := map[Stage]int{
		Stage_befor_send_data: OrderMax,
		Stage_set_data:        OrderMin,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return 0
}

type MiddlewareFunc struct {
	Order int
	Stage Stage
	Fn    func(message *Message) error
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
	for _, m := range ms {
		err := m.Fn(p)
		if err != nil {
			return err
		}
	}
	return nil
}

// 中间件设置简化方法

func (r *Protocol) WithRequestMiddleware(fns ...MiddlewareFunc) *Protocol {
	r.Request.MiddlewareFuncs = append(r.Request.MiddlewareFuncs, fns...)
	return r
}

func (r *Protocol) WithResponseMiddleware(fns ...MiddlewareFunc) *Protocol {
	r.Response.MiddlewareFuncs = append(r.Response.MiddlewareFuncs, fns...)
	return r
}
