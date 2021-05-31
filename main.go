package main

import (
	"github.com/owenliang/nacos-reverse-proxy/forward_proxy"
	"github.com/owenliang/nacos-reverse-proxy/service_discovery"
)

func main() {
	var err error

	// nacos服务发现
	sd, err := service_discovery.NewNacosServiceDiscovery(&service_discovery.NacosSDConfig{
		Namespace:  "myns",
		Cluster:    "default",
		Group:      "default",
		NacosNodes: []service_discovery.NacosNode{{"127.0.0.1", 8848}},
	})
	if err != nil {
		panic(err)
	}

	// 正向代理
	proxy, err := forward_proxy.NewForwardProxy(&forward_proxy.ForwardProxyConfig{
		ListenAddr: ":1080",
		RetryTimes: 3,
		Sd:         sd,
	})
	if err != nil {
		panic(err)
	}

	proxy.Run()
}
