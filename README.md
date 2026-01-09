# demo-go-tiny

一个超迷你、开箱即用的 Go HTTP 服务，专为 **K8s & APISIX** 功能验证设计。

---

## 一、特性速览

| 接口 | 方法 | 描述 | 示例 |
|---|---|---|---|
| `/ping` | GET | 探活 | `curl http://demo.local/ping` |
| `/echo` | GET/POST | 回显 Query + Body | `curl http://demo.local/echo -d hello=world` |
| `/ip` | GET | 获取客户端真实 IP（兼容 X-Real-Ip / X-Forwarded-For） | `curl http://demo.local/ip` |
| `/env` | GET | 查看 Pod 名称、节点名、版本、启动时间 | `curl http://demo.local/env` |
| `/delay?ms=500` | GET | 模拟延迟（ms 可改） | `curl http://demo.local/delay?ms=500` |
| `/mem?mb=100&ms=10000` | GET | 模拟内存占用（MB 可改，可设置保持时长ms） | `curl http://demo.local/mem?ms=20000&mb=100` |
| `/cpu?ms=2000&cores=2&percent=80` | GET | 模拟CPU占用（可控制时间、核心数和占用百分比） | `curl http://demo.local/cpu?ms=5000&cores=1&percent=100` |
| `/` | GET | 列出所有路由 | `curl http://demo.local/` |

---

## 二、本地运行

```bash
go run main.go
# 默认端口 8080
```

---

## 三、容器化

Dockerfile（多阶段构建，镜像 < 10 MB）

```bash
docker build -t demo-go-tiny:1.0 .
docker run -p 8080:8080 demo-go-tiny:1.0
```

---

## 四、K8s 一键部署

```bash
kubectl apply -f deployment.yaml
```

| 环境变量 | 来源 | 说明 |
|---|---|---|
| `POD_NAME` | Downward API | Pod 名称 |
| `NODE_NAME` | Downward API | 所在节点 |
| `VERSION` | 手动注入 | 镜像版本 |
| `PORT` | 可选 | 监听端口，默认 8080 |

**健康探针**已内置：`/ping`

---

## 五、APISIX 路由示例

```yaml
apiVersion: apisix.apache.org/v2
kind: ApisixRoute
metadata:
  name: demo-route
spec:
  http:
  - name: demo
    match:
      hosts: [demo.local]
      paths: ["/*"]
    backends:
    - serviceName: demo-go-tiny
      servicePort: 80
    plugins:
    - name: limit-req
      enable: true
      config:
        rate: 100
        burst: 50
        key: remote_addr
```

---

## 六、常用测试命令

```bash
# 探活
curl http://demo.local/ping

# 回显
curl http://demo.local/echo -d 'hello=world'

# 真实 IP（经过代理）
curl -H 'X-Real-Ip: 1.2.3.4' http://demo.local/ip

# 延迟 500ms
curl http://demo.local/delay?ms=500

# 占 200 MiB 内存
curl http://demo.local/mem?mb=200

# 占 100 MiB 内存，保持10秒（前5秒缓步提升，后5秒保持）
curl http://demo.local/mem?mb=100&ms=10000

# CPU测试：使用1个核心，100%占用率，持续3秒
curl http://demo.local/cpu?ms=3000&cores=1&percent=100

# CPU测试：使用所有核心，50%占用率，持续5秒
curl http://demo.local/cpu?ms=5000&percent=50

# CPU测试：使用2个核心，80%占用率，持续10秒
curl http://demo.local/cpu?ms=10000&cores=2&percent=80
```

---

## 七、用途清单

☑ 替代线上业务做**灰度/限流/熔断**测试  
☑ 验证 **K8s 健康探针**与**滚动发布**  
☑ 验证 **APISIX** 路由、插件、流量拆分  
☑ 压测时模拟延迟/内存飙高场景  

---

## 八、License

MIT - 随便用，出问题不负责 :)