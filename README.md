# nacos-forward-proxy（under develop)

关于服务注册&发现，并不是所有的技术栈都能通过对接SDK的方式接入进来，例如php/python/nodejs等。

因此，实现一个独立于应用程序的外部代理服务，间接为应用实现注册中心的对接与流量转发是一个不错的想法。

## 浅谈servicemesh思路

首先看向servicemesh方案，典型就是以envoy作为数据面，自研控制面对接到自己的注册中心，实现服务注册&发现。

对于mesh方案来说，透明流量劫持与流量协议识别是envoy的强大所在，自研一个类似envoy的东西几乎必要，因此大家多会选择为envoy订制开发控制面，就像istio一样。

在每个容器内运行envoy，同时遵循envoy标准从外界获取控制面配置，控制面组件则选择自研，只要能感知service endpoint变化并同步给envoy即可，跨集群可以将配置同步到自己的注册中心来散播。

envoy采用iptables流量劫持与流量识别的sidecar网格思路，能够应对任何协议的通讯流量管控，的确是很强大的，但性能损耗和技术复杂度是客观存在的，需要投入不少研发工作量，一般企业主肯定会权衡一下投入产出。

## proxy思路

本项目提供一种proxy正向代理的实现思路，功能有限但实现简单，成本低易操作。

proxy提供http/https正向代理功能，应用客户端通常支持配置http代理选项，侵入性低可以接受，也无需实现复杂的透明流量拦截与协议识别。

当客户端调用proxy时，proxy提取请求中的http host并通过服务注册中心（这里是nacos）选择一个健康的后端ip:port完成请求转发（对于找不到的服务则走DNS解析），这就代替应用完成了“服务发现”特性。

proxy持续反向健康探测应用的存活状态，并向服务注册中心同步节点状态；

## proxy与kubernetes

proxy部署到k8s需要注意和应用容器之间的启动次序和流量无损问题，这里面有2个客观现实：
* POD内的container是按顺序启动的，只有前一个container启动并执行完postStart Hook后才会继续启动后续container。
* POD内的container是同时退出的，先并发的执行每个container的preStop hook，再杀死各个容器。

因此，我们需要作如下的设计考虑，

启动顺序(YAML中容器配置顺序）：
* proxy容器先启动，在它的postStart hook脚本中，需要检查并等待proxy代理端口监听成功后再返回。
* 业务容器接着启动。
* proxy持续健康检查业务容器，探测成功则注册至nacos，否则摘除。

退出顺序：
* proxy容器收到退出信号后，立即取消服务注册，然后开始探活应用端口直到应用退出，随后自杀。
* 业务容器在preStop里需要sleep一定时间（和原先等待endpoints摘除一样)，目的是确保proxy容器摘除了注册，然后再杀死业务容器即可。

## 体验

在本地启动nacos server，默认配置即可。

启动proxy：

```
go run main.go -cluster default -group default -namespace default -nodes 127.0.0.1:8848 -listen :1080
```

验证HTTP协议正向代理：

```
curl --proxy http://127.0.0.1:1080 'http://www.baidu.com' 
```

验证HTTPS协议正向代理：

```
curl --proxy http://127.0.0.1:1080 'https://www.baidu.com' 
```