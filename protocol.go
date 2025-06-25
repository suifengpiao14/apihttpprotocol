// Package apihttpprotocol 提供统一的接口协议编解码、中间件处理能力
package apihttpprotocol

import (
	"encoding/json"
	"net/http"
	"slices"
)

// 通用请求/响应结构体

type Message struct {
	Header          http.Header
	GoStructRef     any
	Raw             string
	MiddlewareFuncs MiddlewareFuncs
}

func (m Message) Packet(param any) (string, error) {
	m.GoStructRef = param
	m1, err := m.MiddlewareFuncs.Apply(m)
	if err != nil {
		return "", err
	}
	if m1.GoStructRef == nil {
		return "", nil
	}
	b, err := json.Marshal(m1.GoStructRef)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (m Message) UnPacket(dst any) error {
	m.GoStructRef = dst
	m1, err := m.MiddlewareFuncs.Apply(m)
	if err != nil {
		return err
	}
	if m1.Raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(m1.Raw), dst)
}

// 协议封装

type Protocol struct {
	Request  Message
	Response Message
}

func NewProtocol() Protocol {
	return Protocol{}
}

// 中间件结构

type Stage string

const (
	StageBuilder         Stage = "builder"
	StageSetData         Stage = "set_data"
	StageResponseBuilder Stage = "recive_data"

	OrderMax = 999999
	OrderMin = 1
)

func (s Stage) Order() int {
	m := map[Stage]int{
		StageBuilder: OrderMax,
		StageSetData: OrderMin,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return 0
}

type MiddlewareFunc struct {
	Order int
	Stage Stage
	Fn    func(message Message) (Message, error)
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

func (m MiddlewareFunc) Apply(p Message) (Message, error) {
	if m.Fn == nil {
		return p, nil
	}
	return m.Fn(p)
}

func (ms MiddlewareFuncs) Apply(p Message) (Message, error) {
	for _, m := range ms {
		var err error
		p, err = m.Fn(p)
		if err != nil {
			return p, err
		}
	}
	return p, nil
}

// 中间件设置简化方法

func (r Protocol) WithRequestMiddleware(fns ...MiddlewareFunc) Protocol {
	r.Request.MiddlewareFuncs = append(r.Request.MiddlewareFuncs, fns...)
	return r
}

func (r Protocol) WithResponseMiddleware(fns ...MiddlewareFunc) Protocol {
	r.Response.MiddlewareFuncs = append(r.Response.MiddlewareFuncs, fns...)
	return r
}
