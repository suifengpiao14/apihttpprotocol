package apihttpprotocol

import (
	"bytes"
	"io"
	"net/http"
)

// deepCopyHeader 深拷贝 http.Header
func deepCopyHeader(h http.Header) http.Header {
	copy := make(http.Header, len(h))
	for k, vv := range h {
		dst := make([]string, len(vv))
		copySlice(dst, vv)
		copy[k] = dst
	}
	return copy
}

// deepCopyTrailer 深拷贝 http.Header (trailer)
func deepCopyTrailer(t http.Header) http.Header {
	if t == nil {
		return nil
	}
	return deepCopyHeader(t)
}

// copySlice is a helper for copying string slices
func copySlice(dst, src []string) {
	for i := range src {
		dst[i] = src[i]
	}
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
	reqCopy.Trailer = deepCopyTrailer(r.Trailer)

	return reqCopy, nil
}

// CopyResponse 深拷贝 http.Response，Body 可重复读取
func CopyResponse(resp *http.Response) (*http.Response, error) {
	var bodyCopy io.ReadCloser
	if resp.Body != nil {
		data, err := io.ReadAll(resp.Body)
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
	respCopy.Trailer = deepCopyTrailer(resp.Trailer)

	return &respCopy, nil
}
