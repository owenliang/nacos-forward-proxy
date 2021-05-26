package service_discovery

import (
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// 服务注册&发现
type NacosServiceDiscovery struct {
	sdConfig    *NacosSDConfig
	nacosClient naming_client.INamingClient
}

// Nacos服务端IP地址
type NacosNode struct {
	Ip   string
	Port uint64
}

// nacos配置
type NacosSDConfig struct {
	Namespace  string
	Cluster    string
	Group      string
	NacosNodes []NacosNode
}

// 新建nacos客户端
func NewNacosServiceDiscovery(nacosSDConfig *NacosSDConfig) (nacosServiceDiscovery *NacosServiceDiscovery, err error) {
	nacosServiceDiscovery = &NacosServiceDiscovery{
		sdConfig: nacosSDConfig,
	}

	// 连接Nacos
	sc := make([]constant.ServerConfig, 0)
	for _, node := range nacosSDConfig.NacosNodes {
		sc = append(sc, *constant.NewServerConfig(node.Ip, node.Port))
	}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId(nacosSDConfig.Namespace),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
	)
	if nacosServiceDiscovery.nacosClient, err = clients.NewNamingClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc}); err != nil {
		return
	}
	return
}

// 注册
func (nsd *NacosServiceDiscovery) RegisterService(options *RegisterServiceOptions) (err error) {
	_, err = nsd.nacosClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          options.Ip,
		Port:        options.Port,
		ServiceName: options.ServiceName,
		Weight:      options.Weight,
		Healthy:     true,
		Enable:      true,
		Ephemeral:   true,
		ClusterName: nsd.sdConfig.Cluster,
		GroupName:   nsd.sdConfig.Group,
	})
	return
}

// 取消注册
func (nsd *NacosServiceDiscovery) UnRegisterService(options *UnRegisterServiceOptions) (err error) {
	_, err = nsd.nacosClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          options.Ip,
		Port:        options.Port,
		Cluster:     nsd.sdConfig.Cluster,
		ServiceName: options.ServiceName,
		GroupName:   nsd.sdConfig.Group,
		Ephemeral:   true,
	})
	return
}

// 更新服务信息
func (nsd *NacosServiceDiscovery) UpdateService(options *UpdateServiceOptions) (err error) {
	return
}

// 服务发现节点
func (nsd *NacosServiceDiscovery) SelectInstance(options *SelectInstanceOptions) (instance *ServiceInstance, err error) {
	return
}
