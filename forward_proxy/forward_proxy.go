package forward_proxy

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
)

// 正向HTTP(S)代理
type ForwardProxy struct {
	server *http.Server
}

// HTTPS
func (forwardProxy *ForwardProxy) handleHttpsRequest(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK) // 协商完成

	// TODO: TLS原生流量转发
}

func (forwardProxy *ForwardProxy) transferHttpRequest(req *http.Request) (resp *http.Response, respBody []byte, err error) {
	// TODO：服务发现在此展开

	// 发送请求
	if resp, err = http.DefaultTransport.RoundTrip(req); err != nil {
		return
	}
	// 读取应答
	defer resp.Body.Close()
	if respBody, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}
	return
}

// 拷贝应答
func (forwardProxy *ForwardProxy) copyResponse(dst http.ResponseWriter, src *http.Response, body []byte) {
	// 拷贝header
	for key, values := range src.Header {
		for _, v := range values {
			dst.Header().Add(key, v)
		}
	}
	dst.WriteHeader(src.StatusCode) // 状态码
	dst.Write(body)                 // 消息体
}

// HTTP
func (forwardProxy *ForwardProxy) handleHttpRequest(rw http.ResponseWriter, req *http.Request) {
	var err error

	// 读取body
	var reqBody []byte
	if req.Body != nil {
		if reqBody, err = ioutil.ReadAll(req.Body); err != nil {
			return
		}
	}

	// 客户端已离开?
	var clientLeave bool

	// 重试3次
	for i := 0; i < 3; i++ {
		// 应答
		var resp *http.Response
		var respBody []byte

		// 转发请求
		func() {
			// 监听客户端侧关闭，随即中断服务端侧的请求
			ctx, cancelFunc := context.WithCancel(context.TODO())
			defer cancelFunc()
			go func() {
				select {
				case <-req.Context().Done():
					clientLeave = true
					cancelFunc()
				case <-ctx.Done():
				}
			}()

			// 构造转发请求
			remoteReq := req.Clone(ctx)
			if reqBody == nil {
				remoteReq.Body = nil
			} else {
				remoteReq.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
			}
			// 发送请求到服务端，获取应答
			resp, respBody, err = forwardProxy.transferHttpRequest(remoteReq)
		}()

		// 客户端离开了, 那么就这样吧
		if clientLeave {
			return
		}
		// 服务端侧有错误, 继续重试
		if err != nil {
			continue
		}
		// 请求成功，转发应答
		forwardProxy.copyResponse(rw, resp, respBody)
		return
	}
	// 所有重试均失败
	rw.WriteHeader(500)
}

// 请求入口
func (forwardProxy *ForwardProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	fmt.Println("原始请求:", req.Method, req.URL)

	if req.Method == http.MethodConnect { // HTTPS
		forwardProxy.handleHttpsRequest(rw, req)
	} else { // HTTP
		forwardProxy.handleHttpRequest(rw, req)
	}
}

// 新建HTTP正向代理
func NewForwardProxy() (forwardProxy *ForwardProxy, err error) {
	forwardProxy = &ForwardProxy{}

	// 启动HTTP服务
	forwardProxy.server = &http.Server{
		Addr:    "0.0.0.0:1080",
		Handler: forwardProxy,
	}
	go forwardProxy.server.ListenAndServe()
	return
}
