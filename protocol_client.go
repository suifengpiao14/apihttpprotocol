package apihttpprotocol

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
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
	c.request.middlewareFuncs.Add(c.request.GetIOWriter())
	err = c.request.Run()
	if err != nil {
		return err
	}
	return nil
}
func (c *ClientProtocol) _ReadResponse(dst any) (err error) {
	c.response.GoStructRef = dst
	c.response.middlewareFuncs.Add(c.response.GetIOReader())
	err = c.response.Run()
	if err != nil {
		return err
	}
	return nil
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

func newClient(timeout time.Duration) (client *http.Client) {
	client = &http.Client{
		Transport: sharedTransport,
		Timeout:   timeout,
	}
	return client
}

const (
	MetaData_CurlCmd = "curl_cmd"
)

type ResponseError struct {
	HttpCode    int
	CurlCommand string
	Body        string
}

func (re ResponseError) Error() string {
	s := fmt.Sprintf(`httpCode:%d,body:%s`, re.HttpCode, re.Body)
	return s
}

func NewClientProtocol(method string, url string) *ClientProtocol {
	client := newClient(10 * time.Second)
	var req *http.Request
	readFn := func(message *ResponseMessage) (err error) {
		requestMessage, ok := message.GetRequestMessage()
		if !ok {
			err = errors.Errorf("requestMessage is nil")
			return err
		}
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		var body []byte
		if response.Body != nil {
			defer response.Body.Close()
			body, err = io.ReadAll(response.Body)
			if err != nil {
				return err
			}
		}
		message.SetRaw(body) //保存网络返回

		err = message.SetDuplicateResponse(response, body)
		if err != nil {
			return err
		}
		httpCode := response.StatusCode
		message.HttpCode = httpCode
		if httpCode != http.StatusOK {
			responseError := ResponseError{
				HttpCode:    httpCode,
				CurlCommand: requestMessage.CurlCommand(),
				Body:        string(body),
			}
			return responseError
		}

		if message.GoStructRef != nil {
			if len(body) > 0 {
				err = json.Unmarshal(body, message.GoStructRef)
				if err != nil {
					responseError := ResponseError{
						HttpCode:    httpCode,
						CurlCommand: requestMessage.CurlCommand(),
						Body:        fmt.Sprintf("response body is not valid json,body:%s", string(body)),
					}
					return responseError
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
		req, err = message.ToRequest()
		if err != nil {
			return err
		}
		err = message.SetDuplicateRequest(req)
		if err != nil {
			return err
		}
		return nil
	}
	clientProtocol := NewClitentProtocol().WithIOFn(readFn, writeFn)
	clientProtocol.Request().URL = url
	clientProtocol.Request().Method = method
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
// func httpRequestToResty(client *resty.Client, req *http.Request) (*resty.Request, error) {
// 	r := client.R()
// 	// 设置 Method 和 URL
// 	r.SetMethod(req.Method)
// 	r.SetURL(req.URL.String())
// 	r.SetHeaderMultiValues(req.Header)
// 	bodyReadCloser := req.Body
// 	// 设置 Body
// 	if bodyReadCloser != nil {
// 		defer bodyReadCloser.Close()
// 		body, err := io.ReadAll(bodyReadCloser)
// 		if err != nil {
// 			return nil, err
// 		}
// 		r.SetBody(body)
// 		// 重置原 request Body，防止后续重复读取失败
// 		req.Body = io.NopCloser(bytes.NewReader(body))
// 	}

// 	r = r.SetCookies(req.Cookies())
// 	return r, nil
// }
