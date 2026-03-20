# 代码目录说明

```shell
console-service
├── cmd                         # 程序主入口
│   ├── config                  # 配置解析目录
│   │   ├── runcfg.go           # 服务运行配置对象
│   └── main.go                 # 程序启动入口
├── docs                         # 文档目录
├── go.mod
├── go.sum
├── install                     # k8s 部署文件
│   ├── console-service-deployment.yaml
│   ├── console-service-service.yaml
│   ├── console-service-namespace.yaml
│   └── install.sh
├── LICENSE
└── pkg                         # 核心代码实现目录
    ├── app-management
    │   └── example_dir1 # 资源类型目录，如k8s资源，监控，日志等注册路由目录
    │       └── v1beta1        # 版本控制
    │           ├── handler.go  # restfull 接口处理
    │           ├── register.go # restfull 接口注册
    ├── client                  # 资源操作客户端
    │   └── k8s
    │       ├── k8scfg.go       # k8s 配置对象
    │       └── k8sclient.go    # k8s 客户端对象
    ├── server                  # http.Server 相关处理
    ├── marketplace                # restful 接口处理的具体实现目录
    │   ├── service1 #  
    │   │   └── v1beta1        # 版本控制
    └── zlog                 # log工具封装
```