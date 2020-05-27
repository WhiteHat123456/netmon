# NetMon
A generic bandwidth smart controller for solusvm.

```bash
wget -qO- https://github.com/netmon20/netmon/releases/download/v1.0/linux-amd64-netmon > netmon && chmod +x netmon
```

本监控工具可以动态监控 SolusVM 开通的 QEMU 小鸡（虚拟化不限），并对过度使用流量者加以限制。本工具的原理为系统的 `tc` 限流，因此目前只建议在母鸡上使用。

本监控工具自带管理界面（默认端口 `8358` 默认用户 `netmon` 默认密码 `netmon20`，如果想暴露至公网，请修改密码以免被入侵）。

在管理界面中，你可以随时监控所有小鸡的实时带宽和统计流量，并进行限速或者白名单等操作。

关于不同带宽上限的可以灵活调整启动命令来适应不同场景，具体配置请查看参数。

最后，这个工具诞生的初衷是为了减少滥用，而不是疯狂超售，因此请合理使用。

### 构建

首先你需要安装 `golang`，若你不想自己编译，可以直接在 [release](https://github.com/netmon20/netmon/releases) 页面下载编译好的二进制文件。

```bash
# go get -u github.com/go-bindata/go-bindata/...
# go-bindata -fs -prefix "static/" static/

# 上面两步不需要了，因为静态资源已经打包完成
go build -o netmon .
```

就是这么简单。

### 额外配置

#### 默认启动

```bash
./netmon -ifce=kvm -qos
```

上面代表监控所有 kvm 小鸡并开启 QoS

#### 参数表

```bash
Usage of ./netmon:
  -ifce string
        Network interface prefix (String: null)
  -intv int
        Updating interval (Second: 3s) (default 3)
  -limit int
        Limit the bandwidth to (Minute: Min) (default -1)
  -log int
        Log level (Number: 3) (default 3)
  -max int
        Maximum scale of bandwidth (KBit: 80Mbps) (default 81920)
  -min int
        Minimun scale of bandwidth (KBit: 8Mbps) (default 8192)
  -pass string
        Password of panel [Username: netmon] (String: netmon20) (default "netmon20")
  -port int
        Binding port (Number: 8358) (default 8358)
  -ptime int
        Overrun machine will be limited for this long (Minute: 10m) (default 10)
  -qos
        Run QOS (Bool: false)
  -time int
        Machine overrun exceeds this period of time will receive a limit (Second: 45s) (default 45)
```
