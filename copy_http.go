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
func CopyRequest(r *http.Request) (*http.Request, error) {
	var bodyCopy io.ReadCloser
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		// 恢复原始 request
		r.Body = io.NopCloser(bytes.NewBuffer(data))
		// 复制用 body
		bodyCopy = io.NopCloser(bytes.NewBuffer(data))
	}

	// 基于原始 request 克隆
	reqCopy := r.Clone(r.Context())
	reqCopy.Body = bodyCopy
	reqCopy.Header = deepCopyHeader(r.Header)
	reqCopy.Trailer = deepCopyHeader(r.Trailer)

	return reqCopy, nil
}

// CopyResponse 深拷贝 http.Response，Body 可重复读取
func CopyResponse(resp *http.Response) (*http.Response, error) {
	var bodyCopy io.ReadCloser
	body := resp.Body
	if body != nil {
		defer body.Close()
		data, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		// 恢复原始 response
		resp.Body = io.NopCloser(bytes.NewBuffer(data))
		// 复制用 body
		bodyCopy = io.NopCloser(bytes.NewBuffer(data))
	}

	respCopy := *resp // 浅拷贝结构体
	respCopy.Body = bodyCopy
	respCopy.Header = deepCopyHeader(resp.Header)
	respCopy.Trailer = deepCopyHeader(resp.Trailer)

	return &respCopy, nil
}
