// Package apihttpprotocol 提供统一的接口协议编解码、中间件处理能力
package apihttpprotocol

import (
	"net/http"
	"reflect"
	"slices"
)

// 通用请求/响应结构体

type Message struct {
	Header          http.Header
	GoStructRef     any
	Raw             string
	MiddlewareFuncs MiddlewareFuncs
}

func (m *Message) Packet() (string, error) {
	err := m.MiddlewareFuncs.Apply(m)
	if err != nil {
		return "", err
	}
	return m.Raw, nil
}

func (m *Message) UnPacket(dst any) error {
	m.GoStructRef = dst
	err := m.MiddlewareFuncs.Apply(m)
	if err != nil {
		return err
	}
	return nil
}

// 协议封装

type Protocol struct {
	Request  Message
	Response Message
}

func (p *Protocol) GetRequestStructRef() any {
	if p.Request.GoStructRef == nil {
		return nil
	}
	rt := reflect.TypeOf(p.Request.GoStructRef)
	if rt.Kind() == reflect.Ptr {
		return p.Request.GoStructRef
	}
	return &p.Request.GoStructRef
}
func (p *Protocol) GetResponseStruct() any {
	return p.Response.GoStructRef
}
func (p *Protocol) WithRequestRaw(raw string) *Protocol {
	p.Request.Raw = raw
	return p
}

func NewProtocol() *Protocol {
	return &Protocol{}
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
