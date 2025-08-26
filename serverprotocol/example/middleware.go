package example

import (
	"crypto/md5"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	translations "github.com/go-playground/validator/v10/translations/zh"
	"github.com/pkg/errors"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol"
	"gitlab.huishoubao.com/gopackage/apihttpprotocol/serverprotocol"
)

// ValidateHeaderMiddle 验证头部传参，但是不验证签名

var ValidateHeaderMiddle serverprotocol.OptionFunc = func(p *serverprotocol.ServerProtocol) *serverprotocol.ServerProtocol {
	p.Request().AddMiddleware(func(message *apihttpprotocol.Message) (err error) {
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
		return nil
	})
	return p
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

var CallerServicesPool = CallerServices{} // 启用签名时，需要配置CallerServicesPool

// 签名算法
var CheckRequestSignatureMiddle serverprotocol.OptionFunc = func(p *serverprotocol.ServerProtocol) *serverprotocol.ServerProtocol {
	p.Request().AddMiddleware(func(message *apihttpprotocol.Message) (err error) {
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
		caller, err := CallerServicesPool.GetCallerService(callerId)
		if err != nil {
			return err
		}
		body := string(message.GetRaw())
		sign := apiSign(body, caller.CallerServiceKey)
		if sign != inputSign {
			err = fmt.Errorf("签名校验失败，期望值：%s,实际值:%s", sign, inputSign)
			return err
		}
		return nil
	})
	return p
}

// 返回json真实名
func getStructJsonTag(fld reflect.StructField) string {
	if strList := strings.SplitN(fld.Tag.Get("json"), ",", 2); len(strList) > 0 {
		if strList[0] == "-" {
			return fld.Name
		}
		return strList[0]
	}
	return fld.Name
}

var ValidateRequestMiddle serverprotocol.OptionFunc = func(p *serverprotocol.ServerProtocol) *serverprotocol.ServerProtocol {
	p.Request().AddMiddleware(func(message *apihttpprotocol.Message) (err error) {
		err = message.Next() //读取数据后
		if err != nil {
			return err
		}
		validate := validator.New()
		validate.RegisterTagNameFunc(getStructJsonTag)

		err = validate.Struct(message.GoStructRef)
		if errors.Is(err, &validator.InvalidValidationError{}) {
			err = nil // 如果message.GoStructRef 不为结构体，忽略验证，方便支持map[string]any 等格式的请求参数
		}
		if err != nil {
			//验证器注册翻译器
			uni := ut.New(zh.New())
			trans, _ := uni.GetTranslator("zh")
			_ = translations.RegisterDefaultTranslations(validate, trans)

			for _, verr := range err.(validator.ValidationErrors) {
				return errors.New(verr.Translate(trans))
			}
		}
		return nil
	})
	return p
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
