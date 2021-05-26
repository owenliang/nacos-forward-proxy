package service_discovery

import (
	"testing"
	"time"
)

func TestMain(t *testing.T) {
	sdConfig := &NacosSDConfig{
		Namespace:  "myns",
		Cluster:    "main",
		Group:      "default",
		NacosNodes: []NacosNode{{"127.0.0.1", 8848}},
	}
	if nsd, err := NewNacosServiceDiscovery(sdConfig); err != nil {
		t.Fatal(err)
	} else {
		err = nsd.RegisterService(&RegisterServiceOptions{
			ServiceName: "liangdong",
			Ip:          "127.0.0.1",
			Port:        8888,
			Weight:      1,
			Enable:      true,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer nsd.UnRegisterService(&UnRegisterServiceOptions{
			ServiceName: "liangdong",
			Ip:          "127.0.0.1",
			Port:        8888,
		})
	}
	time.Sleep(30 * time.Second)
}
