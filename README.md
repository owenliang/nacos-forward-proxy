# nacos-forward-proxy（未完成）

为PHP/PYTHON提供HTTP正向代理服务，优先使用NACOS服务发现，其次使用DNS域名解析托底，内置请求重试与熔断器设计。

## 体验

HTTP协议正向代理：

```
curl --proxy http://127.0.0.1:1080 'http://www.baidu.com'  -I
```

HTTPS协议正向代理

```
curl --proxy http://127.0.0.1:1080 'https://www.baidu.com'  -I
```