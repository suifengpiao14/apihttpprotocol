package example

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
)

/*
 {
   "_head":{
     "_version":"0.01",
     "_msgType":"request",
     "_timestamps":"1523330331",
     "_invokeId":"book1523330331358",
     "_callerServiceId":"110001",
     "_groupNo":"1",
     "_interface":"efence.admin.efenceUpdate",
     "_remark":""
   },
   "_param":{
 // 业务参数
 }
 }

{
    "_head": {
        "_interface": "heatmap.api.createPoint",
        "_msgType": "response",
        "_remark": "",
        "_version": "0.01",
        "_timestamps": "1602488688",
        "_invokeId": "HsbYouPinMallAdminAgent16024886882750",
        "_callerServiceId": "110026",
        "_groupNo": "1"
    },
    "_data": {
        "_ret": "0",
        "_errCode": "0",
        "_errStr": "success",
        "_data":{
             // 业务数据
        }
    }
}
*/

type Head struct {
	Version       string `json:"_version"`
	MsgType       string `json:"_msgType"`
	Timestamps    string `json:"_timestamps"`
	InvokeId      string `json:"_invokeId"`
	CallerService string `json:"_callerServiceId"`
	GroupNo       string `json:"_groupNo"`
	Interface     string `json:"_interface"`
	Remark        string `json:"_remark"`
}

type Request struct {
	Head  Head `json:"_head"`
	Param any  `json:"_param"`
}

func (r Request) Validate() error { return nil }

func (r Request) String() string {
	b, _ := json.Marshal(r)
	s := string(b)
	return s
}

type Response struct {
	Head Head `json:"_head"`
	Data Data `json:"_data"`
}
type BusinessError struct {
	Ret     string `json:"_ret"`
	ErrCode string `json:"_errCode"`
	ErrStr  string `json:"_errStr"`
}

func (e BusinessError) Error() error {
	b, _ := json.Marshal(e)
	s := string(b)
	err := errors.New(s)
	return err

}

func (r Response) Validate() error {
	if r.Data.ErrCode != "0" {
		return fmt.Errorf("error:%s", r.Data.ErrStr)
	}
	return nil
}

type Data struct {
	Ret     string `json:"_ret"`
	ErrCode string `json:"_errCode"`
	ErrStr  string `json:"_errStr"`
	Data    any    `json:"_data"`
}

type CallerService struct {
	CallerServiceId  string `json:"callerServiceId"`
	CallerServiceKey string `json:"callerServiceKey"`
}

// Protocol2Client 二层协议 用于发送二层协议请求包、接收二层协议响应包 用于客户端
type Protocol2Client struct {
	Protocol2[Protocol2Client]
}

// Protocol2Server 二层协议 用于接收二层协议请求包、返回二层协议响应包 用于服务端
type Protocol2Server struct {
	Protocol2[Protocol2Server]
}

func NewProtocol2Client() *Protocol2Client {
	p := &Protocol2Client{}
	p.Protocol2 = NewProtocol2(p)
	return p
}

func NewProtocol2Server() *Protocol2Server {
	p := &Protocol2Server{}
	p.Protocol2 = NewProtocol2(p)
	return p
}

type Protocol2Type interface {
	Protocol2Client | Protocol2Server
}

type Protocol2[T Protocol2Type] struct {
	self          *T
	Protocol      apihttpprotocol.Protocol
	ApiPath       string
	RequestParam  Request
	ResponseParam Response
	callerService CallerService
}

func NewProtocol2[T Protocol2Type](self *T) Protocol2[T] {
	return Protocol2[T]{self: self}
}

func (p *Protocol2[T]) WithCallerService(callerService CallerService) *T {
	p.callerService = callerService
	p.RequestParam.Head.CallerService = callerService.CallerServiceId
	p.Protocol.Request.Header.Add(Http_header_HSB_OPENAPI_CALLERSERVICEID, p.callerService.CallerServiceId)

	return p.self
}

func (p *Protocol2[T]) WithInterface(_interface string) *T {
	p.RequestParam.Head.Interface = _interface
	return p.self
}

func (p *Protocol2[T]) WithApiPath(apiPath string) *T {
	p.ApiPath = apiPath
	_interface := strings.Trim(strings.ReplaceAll(apiPath, "/", "."), ".")
	if p.RequestParam.Head.Interface == "" {
		p.RequestParam.Head.Interface = _interface
	}
	return p.self

}

func (p Protocol2Client) WriteRequest(param any) (err error) {
	p.RequestParam.Param = param
	err = p.Protocol.WriteRequest(p.RequestParam)
	if err != nil {
		return err
	}
	return nil
}

func (p Protocol2Client) ReadResponse(dst ResponseI) (err error) {
	p.ResponseParam.Data.Data = dst
	err = p.Protocol.ReadResponse(p.ResponseParam)
	if err != nil {
		return err
	}
	err = dst.Error()
	if err != nil {
		return err
	}
	return nil
}

func (p Protocol2Server) ReadRequest(param apihttpprotocol.ValidateI) (err error) {
	p.RequestParam.Param = param
	err = p.Protocol.ReadRequest(p.RequestParam)
	if err != nil {
		return err
	}
	return nil
}

func (p Protocol2Server) PacketResponse(param any) {
	p.ResponseParam.Data.Data = param
	p.Protocol.WriteResponse(p.ResponseParam)

}

const (
	Http_header_HSB_OPENAPI_CALLERSERVICEID = "HSB-OPENAPI-CALLERSERVICEID"
	Http_header_HSB_OPENAPI_SIGNATURE       = "HSB-OPENAPI-SIGNATURE"
)

func (p *Protocol2Client) UseSignature() *Protocol2Client {
	p.Protocol.WithRequestMiddleware(apihttpprotocol.MiddlewareFunc{
		Order: 1,
		Stage: apihttpprotocol.Stage_befor_send_data,
		Fn: func(message *apihttpprotocol.Message) error {
			sign := apiSign(p.RequestParam.String(), p.callerService.CallerServiceKey)
			p.Protocol.Request.Header.Add(Http_header_HSB_OPENAPI_SIGNATURE, sign)
			return nil
		},
	})
	return p
}

func (p *Protocol2Server) UseCheckSignature() *Protocol2Server {
	p.Protocol.WithRequestMiddleware(apihttpprotocol.MiddlewareFunc{
		Order: apihttpprotocol.OrderMax,
		Stage: apihttpprotocol.Stage_recive_data,
		Fn: func(param *apihttpprotocol.Message) (err error) {
			callerId := param.Header.Get(Http_header_HSB_OPENAPI_CALLERSERVICEID)
			if callerId == "" {
				err = errors.New("http协议头部HTTP_HSB_OPENAPI_CALLERSERVICEID值为空或不存在!")
				return err
			}

			inputSign := param.Header.Get(Http_header_HSB_OPENAPI_SIGNATURE)
			if inputSign == "" {
				err = errors.New("http协议头部HTTP_HSB_OPENAPI_SIGNATURE为空或者不存在!")
				return err
			}
			sign := apiSign(param.GetRaw(), p.callerService.CallerServiceKey)
			if sign != inputSign {
				err = fmt.Errorf("签名校验失败，期望值：%s,实际值:%s", sign, inputSign)
				return err
			}
			return nil
		},
	})
	return p
}

func apiSign(req string, key string) string {
	signStr := req + "_" + key
	digestBytes := md5.Sum([]byte(signStr))
	md5Str := fmt.Sprintf("%x", digestBytes)
	return md5Str
}

func NewSerivceProtocol(c *gin.Context, callerServiceId string, callerServiceKey string) Protocol2Server {
	request := Request{
		Head: Head{
			Version: SerivceProtocol_version,
			MsgType: SerivceProtocol_msgType_request,
			// Timestamps:    cast.ToString(time.Now().Unix()), //这个参数在实际请求时生成
			// InvokeId:      uuid.New().String(),//这个参数在实际请求时生成
			CallerService: "",
			GroupNo:       SerivceProtocol_GroupNo,
			Interface:     "",
			Remark:        "",
		},
	}
	response := Response{}
	protocol := NewGinSerivceProtocol(c).WithRequestMiddleware(apihttpprotocol.MiddlewareFunc{
		Order: 1,
		Stage: apihttpprotocol.Stage_set_data,
		Fn: func(message *apihttpprotocol.Message) error {
			request.Head.Timestamps = cast.ToString(time.Now().Unix()) //这个参数在实际请求时生成
			request.Head.InvokeId = uuid.New().String()                //这个参数在实际请求时生成
			message.GoStructRef = request
			return nil
		},
	})
	s := Protocol2Server{
		Protocol2: Protocol2[Protocol2Server]{
			callerService: CallerService{
				CallerServiceId:  callerServiceId,
				CallerServiceKey: callerServiceKey,
			},
			Protocol:      *protocol,
			RequestParam:  request,
			ResponseParam: response,
		},
	}
	return s
}

const (
	SerivceProtocol_version          = "0.01"
	SerivceProtocol_msgType_request  = "request"
	SerivceProtocol_msgType_response = "response"
	SerivceProtocol_GroupNo          = "1"
)

// 错误处理结构

type HttpError struct {
	HttpStatus string `json:"httpStatus"`
	HttpBody   string `json:"httpBody"`
}

func (e HttpError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// 响应必须实现 Error() 方法判断是否业务失败

type ResponseI interface {
	Error() error
}

func NewGinSerivceProtocol(c *gin.Context) *apihttpprotocol.Protocol {
	readFn := func(message *apihttpprotocol.Message) (err error) {
		err = c.BindJSON(message.GoStructRef)
		return err
	}
	writeFn := func(message *apihttpprotocol.Message) (err error) {
		c.JSON(http.StatusOK, message.GoStructRef)
		return nil
	}
	protocol := apihttpprotocol.NewServerProtocol(readFn, writeFn)
	return protocol
}
