# nacos-forward-proxy（未完成）

为PHP/PYTHON提供HTTP正向代理服务，将在每个应用容器中运行。

为应用代理HTTP请求，基于NACOS服务发现IP:PORT，进行带重试和熔断的请求调用。

健康探测容器内的应用端口存活性，代替应用完成NACOS服务注册。

开发状态：

* 提供HTTP/HTTPS正向代理：100%
* 代理时基于NACOS服务发现并DNS托底：100%
* 服务发现的熔断器：100%
* 代替应用服务注册：0%

## 体验

HTTP协议正向代理：

```
curl --proxy http://127.0.0.1:1080 'http://www.baidu.com' 
```

HTTPS协议正向代理：

```
curl --proxy http://127.0.0.1:1080 'https://www.baidu.com' 
```