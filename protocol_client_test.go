package apihttpprotocol

import (
	"testing"
)

func TestNewRestyClientProtocol(t *testing.T) {
	client1 := NewRestyClientProtocol("GET", "http://127.0.0.1:8080/api/v1/test")
	var req any
	client1._WriteRequest(req)
	client2 := NewRestyClientProtocol("POST", "http://127.0.0.1:8080/api/v1/test")
	client2._WriteRequest(req)
	t.Logf("%+v\n%+v\n", client1, client2)
}
