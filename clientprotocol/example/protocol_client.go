package example

import (
	"crypto/md5"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol/clientprotocol"
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
	Protocol      clientprotocol.ClientProtocol
	ApiPath       string
	RequestParam  Request
	ResponseParam Response
	callerService CallerService
}

func NewProtocol2Client() *Protocol2Client {
	p := &Protocol2Client{}
	return p
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

const (
	Http_header_HSB_OPENAPI_CALLERSERVICEID = "HSB-OPENAPI-CALLERSERVICEID"
	Http_header_HSB_OPENAPI_SIGNATURE       = "HSB-OPENAPI-SIGNATURE"
)

func (p *Protocol2Client) UseSignature() *Protocol2Client {
	p.Protocol.AddRequestMiddleware(apihttpprotocol.MiddlewareFunc{
		Order: 1,
		Stage: apihttpprotocol.Stage_befor_send_data,
		Fn: func(message *apihttpprotocol.Message) error {
			sign := apiSign(p.RequestParam.String(), p.callerService.CallerServiceKey)
			p.Protocol.Request.SetHeader(Http_header_HSB_OPENAPI_SIGNATURE, sign)
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
