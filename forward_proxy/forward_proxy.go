package forward_proxy

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/owenliang/nacos-reverse-proxy/service_discovery"
)

// 配置
type ForwardProxyConfig struct {
	ListenAddr string                              // 代理监听地址
	Sd         service_discovery.IServiceDiscovery // 服务发现
	RetryTimes int
}

// 正向HTTP(S)代理
type ForwardProxy struct {
	server    *http.Server
	dialer    *net.Dialer
	transport http.Transport
	config    *ForwardProxyConfig
}

// HTTPS
func (forwardProxy *ForwardProxy) handleHttpsRequest(rw http.ResponseWriter, req *http.Request) {
	var err error

	// 建立到服务端的TCP连接
	var serverConn net.Conn
	for i := 0; i < forwardProxy.config.RetryTimes; i++ {
		func() { // 监听客户端侧关闭，随即中断服务端侧的请求
			ctx, cancelFunc := context.WithCancel(context.TODO())
			defer cancelFunc()
			go func() {
				select {
				case <-req.Context().Done():
					cancelFunc()
				case <-ctx.Done():
				}
			}()

			var dstHost string
			var ins *service_discovery.ServiceInstance
			// 服务发现
			if ins, err = forwardProxy.config.Sd.SelectInstance(&service_discovery.SelectInstanceOptions{ServiceName: req.Host}); err != nil {
				dstHost = req.Host // 服务发现失败，走域名解析
			} else { // 服务发现成功
				dstHost = fmt.Sprintf("%s:%d", ins.Ip, ins.Port)
			}
			// 建连到服务端
			serverConn, err = forwardProxy.dialer.DialContext(ctx, "tcp", dstHost)
		}()
		if err == nil {
			break
		}
	}
	if err == nil {
		defer serverConn.Close()
	} else {
		return // 连接失败
	}

	// 接管客户端侧的TCP连接
	var clientConn net.Conn
	if hijacker, ok := rw.(http.Hijacker); ok {
		if clientConn, _, err = hijacker.Hijack(); err != nil {
			return
		}
		defer clientConn.Close() // 接管成功，确保离开前关闭
	} else { // 接管失败
		return
	}

	// 回复客户端HTTPS握手
	if _, err = clientConn.Write([]byte("HTTP/1.0 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	// 等待转发完成
	var transferPair = NewTransferPair(clientConn, serverConn)
	transferPair.DoTransfer()
}

func (forwardProxy *ForwardProxy) transferHttpRequest(req *http.Request) (resp *http.Response, respBody []byte, err error) {
	rawHost := req.Host

	// 服务发现
	var ins *service_discovery.ServiceInstance
	if ins, err = forwardProxy.config.Sd.SelectInstance(&service_discovery.SelectInstanceOptions{ServiceName: req.Host}); err == nil {
		req.URL.Host = fmt.Sprintf("%s:%d", ins.Ip, ins.Port)
		req.Header.Set("Host", rawHost)
	}

	// 发送请求
	if resp, err = forwardProxy.transport.RoundTrip(req); err != nil {
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
	for i := 0; i < forwardProxy.config.RetryTimes; i++ {
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
	if req.Method == http.MethodConnect { // HTTPS
		forwardProxy.handleHttpsRequest(rw, req)
	} else { // HTTP
		forwardProxy.handleHttpRequest(rw, req)
	}
}

// 启动代理
func (forwardProxy *ForwardProxy) Run() {
	forwardProxy.server.ListenAndServe()
}

// 新建HTTP正向代理
func NewForwardProxy(forwardProxyConfig *ForwardProxyConfig) (forwardProxy *ForwardProxy, err error) {
	forwardProxy = &ForwardProxy{}
	forwardProxy.dialer = &net.Dialer{}
	forwardProxy.transport = http.Transport{DisableKeepAlives: true}
	forwardProxy.config = forwardProxyConfig

	// 创建HTTP服务
	forwardProxy.server = &http.Server{
		Addr:    forwardProxyConfig.ListenAddr,
		Handler: forwardProxy,
	}
	return
}
