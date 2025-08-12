package apihttpprotocol

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

//NewGinSerivceProtocol 这个函数注销，因为在客户端用于生成Android客户端时，不需要这个函数，尽量减少依赖

func NewGinSerivceProtocol(c *gin.Context) *ServerProtocol {
	readFn := func(message *Message) (err error) {
		err = c.BindJSON(message.GoStructRef)
		return err
	}
	writeFn := func(message *Message) (err error) {
		c.JSON(http.StatusOK, message.GoStructRef)
		return nil
	}
	protocol := NewServerProtocol(readFn, writeFn)
	return protocol
}
