package apihttpprotocol

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"resty.dev/v3"
)

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

func NewRestyClientProtocol(method string, url string) *Protocol {
	req := restyClientFn().R()
	readFn := func(message *Message) (err error) {
		response, err := req.Execute(method, url)
		if err != nil {
			return err
		}
		b := response.Bytes()
		err = json.Unmarshal(b, message.GoStructRef)
		if err != nil {
			return err
		}
		return nil
	}
	writeFn := func(message *Message) (err error) {
		req.SetHeaders(message.Headers)
		req.SetBody(message.GoStructRef)
		return nil
	}
	protocol := NewClitentProtocol(readFn, writeFn).AddRequestMiddleware(MakeMiddlewareFunc(OrderMin, Stage_befor_send_data, func(message *Message) error {
		curl := req.CurlCmd()
		fmt.Println(curl) // 打印curl命令
		return nil
	}))
	return protocol
}
