package main

import (
	"time"

	"github.com/owenliang/nacos-reverse-proxy/forward_proxy"
)

func main() {
	forward_proxy.NewForwardProxy()
	for {
		time.Sleep(1 * time.Second)
	}
}
