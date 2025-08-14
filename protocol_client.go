package apihttpprotocol

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"resty.dev/v3"
)

type ClientProtocol struct {
	Request  Message
	Response Message
}

func NewClitentProtocol(readFn IOFn, writeFn IOFn) *ClientProtocol {
	p := &ClientProtocol{}
	p = p.WithReadIoFn(readFn).WithReadIoFn(writeFn)
	return p
}

func (c *ClientProtocol) WriteRequest(dta any) (err error) {
	c.Request.GoStructRef = dta
	if err := c.Request.HasIOWriter(); err != nil {
		err = errors.WithMessagef(err, "write request struct %v", c.Request.GoStructRef)
		return err
	}
	err = c.Request.MiddlewareFuncs.Apply(&c.Request)
	if err != nil {
		return err
	}
	return nil
}
func (c *ClientProtocol) ReadResponse(dst ValidateI) (err error) {
	if err := c.Request.HasIOReder(); err != nil {
		err = errors.WithMessagef(err, "read response struct %v", c.Request.GoStructRef)
		return err
	}
	c.Response.GoStructRef = dst
	err = c.Response.MiddlewareFuncs.Apply(&c.Response)
	if err != nil {
		return err
	}
	err = dst.Validate()
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientProtocol) AddRequestMiddleware(middlewares ...MiddlewareFunc) *ClientProtocol {
	c.Request.AddMiddleware(middlewares...)
	return c
}

func (c *ClientProtocol) AddResponseMiddleware(middlewares ...MiddlewareFunc) *ClientProtocol {
	c.Response.AddMiddleware(middlewares...)
	return c
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

func (c *ClientProtocol) WithWriteIoFn(ioFn IOFn) *ClientProtocol {
	c.Request.SetIOWriter(ioFn)
	return c
}

func (c *ClientProtocol) WithReadIoFn(ioFn IOFn) *ClientProtocol {
	c.Response.SetIOReader(ioFn)
	return c
}

var restyClientFn func() *resty.Client = sync.OnceValue(func() *resty.Client {
	client := resty.New()
	// 通用配置
	client.
		SetTimeout(10 * time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second)

	// 可选：设置全局 Header
	//client.SetHeader("User-Agent", "MyApp/1.0")
	client.EnableGenerateCurlCmd()
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		log.Printf("[resty.dev/v3] received signal: %s, closing curl connections...", sig)
		client.Close()
		signal.Stop(c)

	}()
	return client

})

func NewRestyClientProtocol(method string, url string) *ClientProtocol {
	req := restyClientFn().R()
	readFn := func(message *Message) (err error) {
		response, err := req.Execute(method, url)
		if err != nil {
			return err
		}
		b := response.Bytes()
		err = json.Unmarshal(b, message.GoStructRef)
		if err != nil {
			return err
		}
		return nil
	}
	writeFn := func(message *Message) (err error) {
		req.SetHeaders(message.Headers)
		req.SetBody(message.GoStructRef)
		return nil
	}
	clientProtocol := NewClitentProtocol(readFn, writeFn)
	clientProtocol.AddRequestMiddleware(MakeMiddlewareFunc(OrderMin, Stage_befor_send_data, func(message *Message) error {
		curl := req.CurlCmd()
		fmt.Println(curl) // 打印curl命令
		return nil
	}))
	return clientProtocol
}
