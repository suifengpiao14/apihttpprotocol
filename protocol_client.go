package apihttpprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
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

func (p *ClientProtocol) WithIOFn(reder HandlerFuncResponseMessage, writer HandlerFuncRequestMessage) *ClientProtocol {
	p.request.SetIOWriter(writer.HandlerFunc())
	p.response.SetIOReader(reder.HandlerFunc())
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

var sharedTransport = &http.Transport{
	MaxIdleConns:        2000,
	MaxIdleConnsPerHost: 1000,
	IdleConnTimeout:     90 * time.Second,
}
var newRestyClient = sync.OnceValue(func() (client *resty.Client) {

	client = resty.New()
	// 通用配置
	client.
		SetTimeout(10 * time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second)
	client = client.SetTransport(sharedTransport) // 设置共享的传输层,确保连接池可以被复用
	return client
})

func getRestyClient() *resty.Client {
	ctx := context.Background()
	return newRestyClient().Clone(ctx)
}

const (
	MetaData_CurlCmd = "curl_cmd"
)

func NewClientProtocol(method string, url string) *ClientProtocol {
	client := getRestyClient()
	var req *resty.Request
	readFn := func(message *ResponseMessage) (err error) {
		response, err := req.Send()
		if err != nil {
			return err
		}
		body := response.Bytes()
		err = message.SetDuplicateResponse(response.RawResponse, body)
		if err != nil {
			return err
		}
		httpCode := response.StatusCode()
		if httpCode != http.StatusOK {
			err = errors.Errorf("request_url:%s http code:%d,response body:%s", url, httpCode, string(body))
			return err
		}
		message.Metadata.Set(MetaData_HttpCode, httpCode)
		if message.GoStructRef != nil {
			if len(body) > 0 {
				if ok := json.Valid(body); !ok {
					err = errors.Errorf("request_url:%s response body is not valid json:%s", url, string(body))
					return err
				}
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
		var buf *bytes.Buffer
		var httpReq *http.Request
		if message.GoStructRef != nil {
			b, err := json.Marshal(message.GoStructRef)
			if err != nil {
				return err
			}
			buf = bytes.NewBuffer(b)
			httpReq, err = http.NewRequest(method, url, buf)
			if err != nil {
				return err
			}
		} else {
			httpReq, err = http.NewRequest(method, url, nil)
			if err != nil {
				return err
			}
		}
		httpReq.Header = message.Headers

		req, err = httpRequestToResty(client, httpReq)
		if err != nil {
			return err
		}
		err = message.SetDuplicateRequest(httpReq) // resty.Request 在req.Send()之前，取不到req.Request 值，这里暂时先构造
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

// httpRequestToResty 将已有的 *http.Request 转换成 *resty.Request
func httpRequestToResty(client *resty.Client, req *http.Request) (*resty.Request, error) {
	r := client.R()
	// 设置 Method 和 URL
	r.SetMethod(req.Method)
	r.SetURL(req.URL.String())
	r.SetHeaderMultiValues(req.Header)
	bodyReadCloser := req.Body
	// 设置 Body
	if bodyReadCloser != nil {
		defer bodyReadCloser.Close()
		body, err := io.ReadAll(bodyReadCloser)
		if err != nil {
			return nil, err
		}
		r.SetBody(body)
		// 重置原 request Body，防止后续重复读取失败
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	r = r.SetCookies(req.Cookies())
	return r, nil
}
