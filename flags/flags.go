package flags

import (
	"errors"
	"flag"
	"strconv"
	"strings"

	"github.com/owenliang/nacos-reverse-proxy/service_discovery"
)

var (
	Namespace  string
	Group      string
	Cluster    string
	Nodes      string
	ListenAddr string
	RetryTimes int

	NacosNodes []service_discovery.NacosNode
)

func init() {
	flag.StringVar(&Namespace, "namespace", "", "nacos namespace")
	flag.StringVar(&Group, "group", "", "nacos group")
	flag.StringVar(&Cluster, "cluster", "", "nacos cluster")
	flag.StringVar(&Nodes, "nodes", "", "nacos nodes")
	flag.StringVar(&ListenAddr, "listen", "", "proxy listen address")
	flag.IntVar(&RetryTimes, "retry", 3, "retry times for http proxy")
	flag.Parse()
}

func Check() (err error) {
	if Namespace == "" || Group == "" || Cluster == "" || Nodes == "" || ListenAddr == "" || RetryTimes == 0 {
		err = errors.New("命令行参数为空")
		return
	}
	nodes := strings.Split(Nodes, ",")
	for _, node := range nodes {
		fields := strings.Split(node, ":")
		// if len(fields) != 2
		ip := fields[0]
		port := fields[1]
		var nport int
		if nport, err = strconv.Atoi(port); err != nil {
			return
		}
		NacosNodes = append(NacosNodes, service_discovery.NacosNode{Ip: ip, Port: uint64(nport)})
	}
	if len(NacosNodes) == 0 {
		err = errors.New("nacos nodes empty")
		return
	}
	return
}
