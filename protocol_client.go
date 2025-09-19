package apihttpprotocol

import (
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
	"resty.dev/v3"
)

type ClientProtocol struct {
	_Protocol
}

func NewClitentProtocol() *ClientProtocol {
	p := &ClientProtocol{
		_Protocol: newProtocol(),
	}
	return p
}

func (p *ClientProtocol) WithIOFn(reder HandlerFunc[ResponseMessage], writer HandlerFunc[RequestMessage]) *ClientProtocol {
	p.request.SetIOWriter(writer)
	p.response.SetIOReader(reder)
	return p
}

func (p *ClientProtocol) SetLog(log LogI) *ClientProtocol {
	p.request.SetLog(log)
	p.response.SetLog(log)
	return p
}

func (p *ClientProtocol) Request() *RequestMessage {
	return p.request
}

func (p *ClientProtocol) Response() *ResponseMessage {
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
	httpCode := cast.ToInt(c.response.Metadata.GetWithDefault(MetaData_HttpCode, 0))
	return httpCode
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
		//client.EnableGenerateCurlCmd()
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

const (
	MetaData_CurlCmd = "curl_cmd"
)

func NewRestyClientProtocol(method string, url string) *ClientProtocol {
	req := restyClientFn().R()
	req = req.SetMethod(method).SetURL(url) //部分接口不需要设置请求体,不会执行writeFn,又因为输出请求日志在readFn前,必须设置好,请求方法和地址,所以就在外部设置好
	readFn := func(message *ResponseMessage) (err error) {
		response, err := req.Send()
		if err != nil {
			return err
		}
		err = message.SetDuplicateResponse(response.RawResponse)
		if err != nil {
			return err
		}
		body := response.Bytes()
		message.SetRaw(body)
		httpCode := response.StatusCode()
		if httpCode != http.StatusOK {
			err = errors.Errorf("http code:%d,response body:%s", httpCode, string(body))
			return err
		}
		message.Metadata.Set(MetaData_HttpCode, httpCode)
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
	writeFn := func(message *RequestMessage) (err error) {
		req.SetHeaderMultiValues(message.Headers)
		req.SetBody(message.GoStructRef)
		err = message.SetDuplicateRequest(req.RawRequest)
		if err != nil {
			return err
		}
		return nil
	}
	clientProtocol := NewClitentProtocol().WithIOFn(readFn, writeFn)
	return clientProtocol
}

func ResponseMiddleCodeMessageForClient(message *ResponseMessage) (err error) {
	response := &Response{
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
