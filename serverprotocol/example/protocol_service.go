package example

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol/serverprotocol"
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

type CallerServices []CallerService

func (cs CallerServices) GetCallerService(callerId string) (callerService *CallerService, err error) {
	for _, v := range cs {
		if v.CallerServiceId == callerId {
			callerService = &v
		}
	}
	if callerService == nil {
		err = errors.Errorf("配置中callerId(%s)找不到对应的callerService", callerId)
		return nil, err
	}
	return callerService, nil
}

func ProtocolV2ReqeustMiddle(message *apihttpprotocol.Message) (err error) {
	requestParam := &Request{
		Param: message.GoStructRef,
	}
	//message.SetHeader(Http_header_HSB_OPENAPI_CALLERSERVICEID, callerService.CallerServiceId)
	message.GoStructRef = requestParam
	err = message.Next()
	if err != nil {
		return err
	}
	return nil
}
func ProtocolV2ResponseMiddle(message *apihttpprotocol.Message) (err error) {
	requestMessage := serverprotocol.ContextGetReqeustMessage(message.Context)
	request, ok := requestMessage.GoStructRef.(*Request)
	if !ok {
		err = errors.New("请求上下文丢失")
		return err
	}
	respone := &Response{
		Head: Head{
			Version:       request.Head.Version,
			MsgType:       "response",
			Timestamps:    cast.ToString(time.Now().Local().Unix()),
			InvokeId:      request.Head.InvokeId,
			CallerService: request.Head.CallerService,
			GroupNo:       request.Head.GroupNo,
			Interface:     request.Head.Interface,
			Remark:        "respone",
		},
		Data: Data{
			Ret:     "0",
			ErrCode: message.GetBusinessCodeWithDefault("0"),
			ErrStr:  "success",
			Data:    message.GoStructRef,
		},
	}

	if message.ResponseError != nil {
		respone.Data.Ret = "1"
		respone.Data.ErrCode = message.GetBusinessCodeWithDefault("1")
		respone.Data.ErrStr = message.ResponseError.Error()
	}

	message.GoStructRef = respone
	err = message.Next()
	if err != nil {
		return err
	}
	return nil
}

func CheckRequestSignatureMiddle(callerServices CallerServices) func(message *apihttpprotocol.Message) (err error) {
	return func(message *apihttpprotocol.Message) (err error) {
		err = message.Next()
		if err != nil {
			return err
		}
		callerId := message.GetHeader(Http_header_HSB_OPENAPI_CALLERSERVICEID)
		if callerId == "" {
			err = errors.New("http协议头部HTTP_HSB_OPENAPI_CALLERSERVICEID值为空或不存在!")
			return err
		}

		inputSign := message.GetHeader(Http_header_HSB_OPENAPI_SIGNATURE)
		if inputSign == "" {
			err = errors.New("http协议头部HTTP_HSB_OPENAPI_SIGNATURE为空或者不存在!")
			return err
		}
		body := string(message.GetRaw())
		callerService, err := callerServices.GetCallerService(callerId)
		if err != nil {
			return err
		}
		sign := apiSign(body, callerService.CallerServiceKey)
		if sign != inputSign {
			err = fmt.Errorf("签名校验失败，期望值：%s,实际值:%s", sign, inputSign)
			return err
		}
		return nil
	}
}

const (
	Http_header_HSB_OPENAPI_CALLERSERVICEID = "HSB-OPENAPI-CALLERSERVICEID"
	Http_header_HSB_OPENAPI_SIGNATURE       = "HSB-OPENAPI-SIGNATURE"
)

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
