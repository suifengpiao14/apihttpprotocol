package apihttpprotocol

import (
	"bytes"
	"io"
	"net/http"
)

// deepCopyHeader 深拷贝 http.Header
func deepCopyHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	copyHeader := make(http.Header, len(h))
	for k, vv := range h {
		dst := make([]string, len(vv))
		copy(dst, vv)
		copyHeader[k] = dst
	}
	return copyHeader
}

// CopyRequest 深拷贝 http.Request，Body 可重复读取
func CopyRequest(r *http.Request) (copyRequest *http.Request, err error) {
	if r == nil {
		return nil, nil
	}
	// 基于原始 request 克隆
	reqCopy := r.Clone(r.Context())

	reqCopy.Header = r.Header.Clone()
	reqCopy.Trailer = deepCopyHeader(r.Trailer)
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return reqCopy, err // CopyResponse 时会忽略err，但是需要reqCopy ，所以这里同步返回reqCopy
		}
		// 恢复原始 request
		r.Body = io.NopCloser(bytes.NewBuffer(data))
		// 复制用 body
		reqCopy.Body = io.NopCloser(bytes.NewBuffer(data))
	}

	return reqCopy, nil
}

// CopyResponse 深拷贝 http.Response，Body 可重复读取
func CopyResponse(resp *http.Response, body []byte) (copyResponse *http.Response, err error) {
	if resp == nil {
		return nil, nil
	}
	respCopy := *resp // 浅拷贝结构体
	respCopy.Header = deepCopyHeader(resp.Header)
	respCopy.Trailer = deepCopyHeader(resp.Trailer)
	respCopy.Request, _ = CopyRequest(resp.Request) // request 可能已经被读取，所以需要忽略错误
	// if err != nil {
	// 	return nil, err
	// }
	if body != nil {
		respCopy.Body = io.NopCloser(bytes.NewBuffer(body))
		return &respCopy, nil // 如果有 body，则直接返回
	}

	var bodyCopy io.ReadCloser
	bodyReader := resp.Body
	if bodyReader != nil {
		defer bodyReader.Close()
		var data []byte
		data, err = io.ReadAll(bodyReader)
		if err != nil {
			return nil, err
		}
		// 恢复原始 response
		resp.Body = io.NopCloser(bytes.NewBuffer(data))
		// 复制用 body
		bodyCopy = io.NopCloser(bytes.NewBuffer(data))
	}
	respCopy.Body = bodyCopy

	return &respCopy, nil
}
