package service_discovery

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/owenliang/nacos-reverse-proxy/breaker"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// Nacos发现的实例
type NacosInstance struct {
	id      string
	ip      string
	port    uint64
	weight  float64
	cluster string
	service *NacosService
	breaker *breaker.Breaker // 熔断器
}

const (
	NACOS_SERVICE_STATUS_NOT_INIT = 0 // 未初始化
	NACOS_SERVICE_STATUS_LOADING  = 1 // 初次加载中
	NACOS_SERVICE_STATUS_RUNNING  = 2 // 正常服务中
)

// Nacos服务
type NacosService struct {
	mu              sync.Mutex
	loadNotify      chan byte
	serviceName     string
	instances       []*NacosInstance
	instanceMapping map[string]*NacosInstance
	status          int
	nsd             *NacosServiceDiscovery
}

// instance成功率统计
func (nacosService *NacosService) markInstance(id string, success bool) {
	nacosService.mu.Lock()
	instanceMapping := nacosService.instanceMapping
	nacosService.mu.Unlock()

	instance, exist := instanceMapping[id]
	if !exist {
		return
	}

	// 给熔断器更新计数
	if success {
		instance.breaker.RecordSuccess()
	} else {
		instance.breaker.RecordFail()
	}
}

func (nsd *NacosServiceDiscovery) newNacosService(serviceName string) (nacosService *NacosService) {
	nacosService = &NacosService{}
	nacosService.serviceName = serviceName
	nacosService.instances = make([]*NacosInstance, 0)
	nacosService.instanceMapping = map[string]*NacosInstance{}
	nacosService.status = NACOS_SERVICE_STATUS_NOT_INIT
	nacosService.nsd = nsd
	return
}

func (nacosService *NacosService) syncNacosServiceForever() {
	for {
		// 拉hosts列表
		instances, err := nacosService.nsd.nacosClient.SelectInstances(vo.SelectInstancesParam{
			ServiceName: nacosService.serviceName,
			GroupName:   nacosService.nsd.sdConfig.Group,
			HealthyOnly: true,
		})
		// NACOS SDK写的太水了，根本区分不出是没有service还是调用报错。。
		if err != nil {
			instances = make([]model.Instance, 0)
		}

		// 生成host mapping
		instanceMapping := make(map[string]*NacosInstance)
		for _, ins := range instances {
			instanceMapping[ins.InstanceId] = &NacosInstance{
				id:      ins.InstanceId,
				ip:      ins.Ip,
				port:    ins.Port,
				weight:  ins.Weight,
				cluster: ins.ClusterName,
				service: nacosService,
			}
		}

		// 取出现在的host mapping
		oldInstanceMapping := nacosService.instanceMapping

		// 将instance之前的状态数据迁移到新instance对象身上
		instanceList := make([]*NacosInstance, 0, len(instanceMapping))
		for id, ins := range instanceMapping {
			if oldIns, exist := oldInstanceMapping[id]; exist {
				// 拷贝之前instance的熔断器到新实例对象
				ins.breaker = oldIns.breaker
			} else {
				ins.breaker = breaker.NewBreaker(&breaker.Options{
					DisonnectPeriod:      5 * time.Second,
					RecoverySuccessTimes: 100,
					WindowSize:           60,
					DecideToDisconnect: func(bs []*breaker.Bucket) bool { // 熔断策略
						fail := 0
						success := 0
						for _, b := range bs {
							success += b.Success
							fail += b.Fail
						}
						return fail != 0 && success != 0 && success+fail >= 5 && float64(fail) >= float64(success)*1.2
					},
				})
			}
			instanceList = append(instanceList, ins)
		}

		// 替换新的instance列表（todo: 优化一下，没有diff不要替换）
		nacosService.mu.Lock()
		if len(instanceList) > 0 { // 列表为空不覆盖旧数据，托个底
			nacosService.instances = instanceList
			nacosService.instanceMapping = instanceMapping
		}
		if nacosService.status == NACOS_SERVICE_STATUS_LOADING { // 唤醒等待者
			close(nacosService.loadNotify)
			nacosService.status = NACOS_SERVICE_STATUS_RUNNING
		}
		nacosService.mu.Unlock()

		// 1秒刷新1次
		time.Sleep(1 * time.Second)
	}
}

func (nacosService *NacosService) getInstances() (instances []*NacosInstance, err error) {
	nacosService.mu.Lock()
	defer nacosService.mu.Unlock()

	// 触发加载
	if nacosService.status == NACOS_SERVICE_STATUS_NOT_INIT {
		nacosService.status = NACOS_SERVICE_STATUS_LOADING // 进入加载中
		nacosService.loadNotify = make(chan byte)          // 无论异步加载成功/失败，都通知管道
		// 在协程中刷新数据
		go nacosService.syncNacosServiceForever()
	}

	if nacosService.status == NACOS_SERVICE_STATUS_LOADING { // 已经加载中，那么等待它完成，但限制等待时间
		notify := nacosService.loadNotify
		nacosService.mu.Unlock()

		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-notify: // 加载完成
		case <-timer.C: // 超时
		}
		nacosService.mu.Lock()
	}

	// 检查当前状态
	if nacosService.status == NACOS_SERVICE_STATUS_RUNNING {
		instances = nacosService.instances
	} else {
		err = errors.New("服务获取失败")
	}
	return
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

// 服务注册&发现
type NacosServiceDiscovery struct {
	sdConfig    *NacosSDConfig
	nacosClient naming_client.INamingClient

	mu             sync.Mutex
	serviceMapping map[string]*NacosService // 服务名 -> 服务对象
}

// 新建nacos客户端
func NewNacosServiceDiscovery(nacosSDConfig *NacosSDConfig) (nacosServiceDiscovery *NacosServiceDiscovery, err error) {
	nacosServiceDiscovery = &NacosServiceDiscovery{
		sdConfig:       nacosSDConfig,
		serviceMapping: make(map[string]*NacosService),
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
	_, err = nsd.nacosClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          options.Ip,
		Port:        options.Port,
		ServiceName: options.ServiceName,
		Weight:      options.Weight,
		Healthy:     true,
		Enable:      options.Enable,
		Ephemeral:   true,
		ClusterName: nsd.sdConfig.Cluster,
		GroupName:   nsd.sdConfig.Group,
	})
	return
}

// 服务发现节点
func (nsd *NacosServiceDiscovery) SelectInstance(options *SelectInstanceOptions) (instance *ServiceInstance, err error) {
	// 找到nacosService
	nsd.mu.Lock()
	nacosService, exist := nsd.serviceMapping[options.ServiceName]
	if !exist {
		nacosService = nsd.newNacosService(options.ServiceName)
		nsd.serviceMapping[options.ServiceName] = nacosService
	}
	nsd.mu.Unlock()

	// 获取实例列表
	instances, err := nacosService.getInstances()
	if err == nil && len(instances) > 0 {
		// 挑出候选节点
		candidateInstances := make([]*NacosInstance, 0, len(instances))
		for _, ins := range instances {
			if ins.breaker.Ok() {
				candidateInstances = append(candidateInstances, ins)
			}
		}
		// 如果没有健康的，那么所有节点都加入候选
		if len(candidateInstances) == 0 {
			candidateInstances = instances
		}
		// 随机选一个返回
		idx := rand.Int() % len(candidateInstances)
		instance = &ServiceInstance{
			ServiceName: options.ServiceName,
			ID:          instances[idx].id,
			Ip:          instances[idx].ip,
			Port:        instances[idx].port,
		}
	} else {
		err = errors.New("没有可用instance")
	}
	return
}

// 节点"正常+1"
func (nsd *NacosServiceDiscovery) MarkInstanceSuccess(options *MarkInstanceOptions) {
	nsd.mu.Lock()
	service, exist := nsd.serviceMapping[options.ServiceName]
	nsd.mu.Unlock()
	if !exist {
		return
	}
	service.markInstance(options.ID, true)
}

// 节点"异常+1"
func (nsd *NacosServiceDiscovery) MarkInstanceFail(options *MarkInstanceOptions) {
	nsd.mu.Lock()
	service, exist := nsd.serviceMapping[options.ServiceName]
	nsd.mu.Unlock()
	if !exist {
		return
	}
	service.markInstance(options.ID, false)
}
