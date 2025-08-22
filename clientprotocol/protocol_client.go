package clientprotocol

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cast"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
	"resty.dev/v3"
)

type ClientProtocol struct {
	Request  apihttpprotocol.Message
	Response apihttpprotocol.Message
}

func NewClitentProtocol(readFn apihttpprotocol.HandlerFunc, writeFn apihttpprotocol.HandlerFunc) *ClientProtocol {
	p := &ClientProtocol{}
	p = p.WithReadIoFn(readFn).WithWriteIoFn(writeFn)
	return p
}

func (c *ClientProtocol) WriteRequest(data any) (err error) {
	c.Request.GoStructRef = data
	c.Request.MiddlewareFuncs.Add(c.Request.GetIOWriter())
	err = c.Request.Run()
	if err != nil {
		return err
	}
	return nil
}
func (c *ClientProtocol) ReadResponse(dst any) (err error) {
	c.Response.GoStructRef = dst
	c.Response.MiddlewareFuncs.Add(c.Response.GetIOReader())
	err = c.Response.Run()
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientProtocol) GetHttpCode() int {
	httpCode := cast.ToInt(c.Response.Metadata.GetWithDefault(apihttpprotocol.MetaData_HttpCode, 0))
	return httpCode
}

func (c *ClientProtocol) AddRequestMiddleware(middlewares ...apihttpprotocol.HandlerFunc) *ClientProtocol {
	c.Request.AddMiddleware(middlewares...)
	return c
}

func (c *ClientProtocol) AddResponseMiddleware(middlewares ...apihttpprotocol.HandlerFunc) *ClientProtocol {
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

func (c *ClientProtocol) WithWriteIoFn(ioFn apihttpprotocol.HandlerFunc) *ClientProtocol {
	c.Request.SetIOWriter(ioFn)
	return c
}

func (c *ClientProtocol) WithReadIoFn(ioFn apihttpprotocol.HandlerFunc) *ClientProtocol {
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
	req = req.SetMethod(method).SetURL(url) //部分接口不需要设置请求体,不会执行writeFn,又因为输出请求日志在readFn前,必须设置好,请求方法和地址,所以就在外部设置好
	readFn := func(message *apihttpprotocol.Message) (err error) {
		curl := req.CurlCmd() //curl依赖 req 变量,所以不独立成middle
		fmt.Println(curl)     // 打印curl命令
		response, err := req.Send()
		if err != nil {
			return err
		}
		message.Metadata.Set(apihttpprotocol.MetaData_HttpCode, response.StatusCode())
		b := response.Bytes()
		err = json.Unmarshal(b, message.GoStructRef)
		if err != nil {
			return err
		}

		err = message.Next()
		if err != nil {
			return err
		}

		return nil
	}
	writeFn := func(message *apihttpprotocol.Message) (err error) {
		req.SetHeaders(message.Headers)
		req.SetBody(message.GoStructRef)
		return nil
	}
	clientProtocol := NewClitentProtocol(readFn, writeFn)
	return clientProtocol
}

func CodeMessageResponseMiddle(message *apihttpprotocol.Message) (err error) {
	response := &apihttpprotocol.Response{
		Data: message.GoStructRef,
	}
	message.GoStructRef = response
	err = message.Next()
	if err != nil {
		return err
	}
	err = response.Validate()
	if err != nil {
		return err
	}
	return nil
}
