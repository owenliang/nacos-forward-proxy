package service_discovery

// 服务注册
type RegisterServiceOptions struct {
	ServiceName string
	Ip          string
	Port        uint64
	Weight      float64
	Enable      bool
}

// 取消注册
type UnRegisterServiceOptions struct {
	ServiceName string
	Ip          string
	Port        uint64
}

// 状态更新
type UpdateServiceOptions struct {
	ServiceName string
	Ip          string
	Port        uint64
	Weight      float64
	Enable      bool
}

// 服务发现
type SelectInstanceOptions struct {
	ServiceName string
}

// 服务节点
type ServiceInstance struct {
	Ip   string
	Port uint64
}

// 服务注册/发现接口
type IServiceDiscovery interface {
	// 注册
	RegisterService(options *RegisterServiceOptions) (err error)

	// 取消注册
	UnRegisterService(options *UnRegisterServiceOptions) (err error)

	// 更新服务信息
	UpdateService(options *UpdateServiceOptions) (err error)

	// 服务发现节点
	SelectInstance(options *SelectInstanceOptions) (instance *ServiceInstance, err error)
}
