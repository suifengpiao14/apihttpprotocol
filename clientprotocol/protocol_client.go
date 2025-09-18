package clientprotocol

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/suifengpiao14/apihttpprotocol"
	"resty.dev/v3"
)

type ClientProtocol struct {
	request  *apihttpprotocol.Message
	response *apihttpprotocol.Message
}

func NewClitentProtocol() *ClientProtocol {
	p := &ClientProtocol{
		request: &apihttpprotocol.Message{
			Context: context.Background(),
		},
		response: &apihttpprotocol.Message{},
	}
	p.response.SetRequestMessage(p.request)
	return p
}

func (p *ClientProtocol) WithIOFn(reder, writer apihttpprotocol.HandlerFunc) *ClientProtocol {
	p.request.SetIOWriter(writer)
	p.response.SetIOReader(reder)
	return p
}

func (p *ClientProtocol) SetLog(log apihttpprotocol.LogI) *ClientProtocol {
	p.request.SetLog(log)
	p.response.SetLog(log)
	return p
}

func (p *ClientProtocol) Request() *apihttpprotocol.Message {
	return p.request
}

func (p *ClientProtocol) Response() *apihttpprotocol.Message {
	return p.response
}

func (c *ClientProtocol) Do(requestData any, resp any) (err error) {
	err = c._WriteRequest(requestData)
	if err != nil {
		return err
	}
	err = c._ReadResponse(resp)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientProtocol) _WriteRequest(data any) (err error) {
	c.request.GoStructRef = data
	c.request.MiddlewareFuncs.Add(c.request.GetIOWriter())
	err = c.request.Run()
	if err != nil {
		return err
	}
	return nil
}
func (c *ClientProtocol) _ReadResponse(dst any) (err error) {
	c.response.GoStructRef = dst
	c.response.MiddlewareFuncs.Add(c.response.GetIOReader())
	err = c.response.Run()
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientProtocol) GetHttpCode() int {
	httpCode := cast.ToInt(c.response.Metadata.GetWithDefault(apihttpprotocol.MetaData_HttpCode, 0))
	return httpCode
}

type Option = apihttpprotocol.Option[ClientProtocol]

type OptionFunc = apihttpprotocol.OptionFunc[ClientProtocol]

func (p *ClientProtocol) Apply(options ...Option) *ClientProtocol {
	for _, option := range options {
		p = option.Apply(p)
	}
	return p
}

func (c *ClientProtocol) SetHeader(key string, value string) *ClientProtocol {
	c.request.SetHeader(key, value)

	return c
}

var restyClientFn func() *resty.Client = sync.OnceValue(func() *resty.Client {
	return RestyClientWithSignalClose(nil)
})

// RestyClientWithSignalClose 信号关闭客户端连接,防止泄露资源
func RestyClientWithSignalClose(client *resty.Client) *resty.Client {
	if client == nil {
		client = resty.New()
		// 通用配置
		client.
			SetTimeout(10 * time.Second).
			SetRetryCount(2).
			SetRetryWaitTime(2 * time.Second).
			SetRetryMaxWaitTime(10 * time.Second)

		// 可选：设置全局 Header
		//client.SetHeader("User-Agent", "MyApp/1.0")
		client.EnableGenerateCurlCmd()
	}
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		log.Printf("[resty.dev/v3] received signal: %s, closing curl connections...", sig)
		client.Close()
		signal.Stop(c)

	}()
	return client

}

func NewRestyClientProtocol(method string, url string) *ClientProtocol {
	req := restyClientFn().R()
	req = req.SetMethod(method).SetURL(url) //部分接口不需要设置请求体,不会执行writeFn,又因为输出请求日志在readFn前,必须设置好,请求方法和地址,所以就在外部设置好
	readFn := func(message *apihttpprotocol.Message) (err error) {
		curl := req.CurlCmd() //curl依赖 req 变量,所以不独立成middle
		if curl != "" {
			message.GetLog().Info(curl)
		}
		response, err := req.Send()
		if err != nil {
			return err
		}
		body := response.Bytes()
		httpCode := response.StatusCode()
		if httpCode != http.StatusOK {
			err = errors.Errorf("http code:%d,response body:%s", httpCode, string(body))
			return err
		}
		message.Metadata.Set(apihttpprotocol.MetaData_HttpCode, httpCode)
		if message.GoStructRef != nil {
			if len(body) > 0 {
				err = json.Unmarshal(body, message.GoStructRef)
				if err != nil {
					return err
				}
			}
		}
		err = message.Next()
		if err != nil {
			return err
		}

		return nil
	}
	writeFn := func(message *apihttpprotocol.Message) (err error) {
		req.SetHeaderMultiValues(message.Headers)
		req.SetBody(message.GoStructRef)
		return nil
	}
	clientProtocol := NewClitentProtocol().WithIOFn(readFn, writeFn)
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
