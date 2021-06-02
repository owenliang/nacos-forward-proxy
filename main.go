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

	// 探测应用健康，完成服务注册，退出前取消服务注册，退出前确保应用先退出。
	for {
		time.Sleep(1 * time.Second)
	}
}
