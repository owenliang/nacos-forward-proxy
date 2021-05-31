package service_discovery

import (
	"fmt"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	serviceName := "liangdong"
	ip := "127.0.0.1"
	var port uint64 = 8888

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
			ServiceName: serviceName,
			Ip:          ip,
			Port:        port,
			Weight:      1,
			Enable:      true,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer nsd.UnRegisterService(&UnRegisterServiceOptions{
			ServiceName: serviceName,
			Ip:          ip,
			Port:        port,
		})

		// 睡5秒
		time.Sleep(5 * time.Second)
		// 更新状态
		nsd.UpdateService(&UpdateServiceOptions{
			ServiceName: serviceName,
			Ip:          ip,
			Port:        port,
			Weight:      5,
			Enable:      true,
		})
		// 再睡30秒
		time.Sleep(31 * time.Second)
	}
}

func TestDiscovery(t *testing.T) {
	sdConfig := &NacosSDConfig{
		Namespace:  "myns",
		Cluster:    "main",
		Group:      "default",
		NacosNodes: []NacosNode{{"127.0.0.1", 8848}},
	}

	if nsd, err := NewNacosServiceDiscovery(sdConfig); err != nil {
		t.Fatal(err)
	} else {
		for t := 0; t < 2; t++ {
			go func() {
				for i := 0; i < 30; i++ {
					ins, err := nsd.SelectInstance(&SelectInstanceOptions{ServiceName: "a.yuerblog.cc"})
					fmt.Println(ins, err)
					time.Sleep(1 * time.Second)
				}
			}()
		}

		time.Sleep(35 * time.Second)
	}
}
