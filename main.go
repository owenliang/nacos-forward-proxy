package main

import (
	"time"

	"github.com/owenliang/nacos-reverse-proxy/flags"

	"github.com/owenliang/nacos-reverse-proxy/forward_proxy"
	"github.com/owenliang/nacos-reverse-proxy/service_discovery"
)

func main() {
	var err error

	// 参数检查
	if err = flags.Check(); err != nil {
		panic(err)
	}

	// nacos服务发现
	sd, err := service_discovery.NewNacosServiceDiscovery(&service_discovery.NacosSDConfig{
		Namespace:  flags.Namespace,
		Cluster:    flags.Cluster,
		Group:      flags.Group,
		NacosNodes: flags.NacosNodes,
	})
	if err != nil {
		panic(err)
	}

	// 正向代理
	proxy, err := forward_proxy.NewForwardProxy(&forward_proxy.ForwardProxyConfig{
		ListenAddr: flags.ListenAddr,
		RetryTimes: flags.RetryTimes,
		Sd:         sd,
	})
	if err != nil {
		panic(err)
	}
	go proxy.Run()

	// TODO: 提供服务注册/反注册的admin接口
	for {
		time.Sleep(1 * time.Second)
	}
}
