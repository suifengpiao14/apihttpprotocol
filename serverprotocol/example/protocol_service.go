package example

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol/serverprotocol"
)

func NewProtocolv2(c *gin.Context) *serverprotocol.ServerProtocol {
	p := serverprotocol.NewServerProtocol().WithIOFn(serverprotocol.NewGinReadWriteMiddleware(c))
	p.Apply(ValidateHeaderMiddle, ProtocolV2ReqeustMiddle, ProtocolV2ResponseMiddle)
	return p
}

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
	InvokeId      string `json:"_invokeId" validate:"required"`
	CallerService string `json:"_callerServiceId"`
	GroupNo       string `json:"_groupNo"`
	Interface     string `json:"_interface"`
	Remark        string `json:"_remark"`
}

type Request struct {
	Head  Head `json:"_head"`
	Param any  `json:"_param"`
}

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
	BusinessError
	Data any `json:"_data"`
}

var ProtocolV2ReqeustMiddle serverprotocol.OptionFunc = func(p *serverprotocol.ServerProtocol) *serverprotocol.ServerProtocol {
	return p.ApplyRequestMiddleware(func(message *apihttpprotocol.Message) (err error) {
		requestParam := &Request{
			Param: message.GoStructRef,
		}
		message.GoStructRef = requestParam
		err = message.Next()
		if err != nil {
			return err
		}
		return nil
	})
}

var ProtocolV2ResponseMiddle serverprotocol.OptionFunc = func(p *serverprotocol.ServerProtocol) *serverprotocol.ServerProtocol {
	return p.ApplyResponseMiddleware(func(message *apihttpprotocol.Message) (err error) {
		requestMessage := message.GetRequestMessage()
		if requestMessage == nil {
			err = errors.New("请求上下文丢失")
			return err
		}
		request, ok := requestMessage.GoStructRef.(*Request)
		if !ok {
			err = errors.New("请求上下文类型不正确")
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
				BusinessError: BusinessError{
					Ret:     "0",
					ErrCode: message.GetBusinessCodeWithDefault("0"),
					ErrStr:  "success",
				},
				Data: message.GoStructRef,
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
	})
}
