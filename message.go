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

func (m *Message) AddMiddleware(middlewares ...MiddlewareFunc) *Message {
	m.MiddlewareFuncs.Add(middlewares...)
	return m
}

// SetIOReader 设置读函数，优先级最高，第一个执行函数,从网络中读取数据，从这个函数开始执行协议中间件链
func (m *Message) SetIOReader(readFn ApplyFn) *Message {
	m.MiddlewareFuncs.Add(MakeMiddlewareFunc(OrderMin, Stage_io_read_data, readFn))
	return m
}

// SetIOWriter 设置写函数，优先级最低，最后一个执行函数,这个函数执行完后，数据流脱离protocol进入网络，其它协议中间件没有执行机会
func (m *Message) SetIOWriter(readFn ApplyFn) *Message {
	m.MiddlewareFuncs.Add(MakeMiddlewareFunc(OrderMax, Stage_io_write_data, readFn))
	return m
}

func (m *Message) HasIOReder() (err error) {
	_, err = m.MiddlewareFuncs.GetBySateMust(Stage_io_read_data)
	if err != nil {
		err = errors.WithMessage(err, "IOReder required")
		return err
	}
	return nil
}
func (m *Message) HasIOWriter() (err error) {
	_, err = m.MiddlewareFuncs.GetBySateMust(Stage_io_write_data)
	if err != nil {
		err = errors.WithMessage(err, "IOWriter required")
		return err
	}
	return nil
}

var ERRIOFnIsNil = errors.New("io function is nil")

// ValidateI 定义了 Error 方法，用于在读取结构体后获取可能的错误信息。(比如返回体errCode 校验,request 的参数校验等)
type ValidateI interface {
	Validate() error
}
type ValidateEmpty struct {
	ReaderStruct any
}

func (r *ValidateEmpty) Validate() error { return nil }

// 中间件结构

type Stage string

const (
	Stage_befor_send_data Stage = "stage_beforSend_data"
	Stage_io_write_data   Stage = "stage_io_write_data"
	Stage_io_read_data    Stage = "stage_io_read_data"

	OrderMax = 999999
	OrderMin = 1
)

// 默认中间件执行顺序，order 越小，越先执行

func (s Stage) Order() int {
	m := map[Stage]int{
		Stage_befor_send_data: OrderMax,
		Stage_io_write_data:   OrderMin,
		Stage_io_read_data:    OrderMin,
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
		Stage: Stage_io_write_data,
		Fn:    fn,
	}
}
func MakeMiddlewareFuncReadData(fn ApplyFn) MiddlewareFunc {
	return MiddlewareFunc{
		Order: OrderMin,
		Stage: Stage_io_write_data,
		Fn:    fn,
	}
}

type MiddlewareFuncs []MiddlewareFunc

func (ms MiddlewareFuncs) Sort() {
	slices.SortFunc(ms, func(a, b MiddlewareFunc) int {
		if d := a.Stage.Order() - b.Stage.Order(); d != 0 {
			return d
		}
		return a.Order - b.Order
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

var ERRMiddlewareNotFound = errors.New("middleware not found")

func (ms *MiddlewareFuncs) GetBySateMust(stage Stage) (middle MiddlewareFunc, err error) {
	for _, m := range *ms {
		if m.Stage == stage {
			return m, nil
		}
	}
	err = errors.WithMessagef(ERRMiddlewareNotFound, "middleware not found for stage %v", stage)
	return middle, err

}
