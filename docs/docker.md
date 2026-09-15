# 容器部署

服务是单一静态二进制，SQLite、迁移、时区与许可证全部内嵌，因此容器内无需任何运行时依赖。镜像基于 `gcr.io/distroless/static-debian12:nonroot`，以 UID/GID `65532` 运行。

## 构建与运行

```bash
docker build --build-arg VERSION=0.3.0 -t openjwc:local .
# 或
docker compose up -d --build
curl http://127.0.0.1:8080/healthz   # {"status":"ok"}
```

`compose.yaml` 默认把端口映射到回环地址，并将数据写入命名卷 `openjwc-data`。

## 首次初始化

注册审核、用户权限、模型设置都只能通过 TUI 完成，容器内用 `exec` 进入同一用户空间：

```bash
docker compose exec -it openjwc openjwc   # 无参数默认进入 TUI
```

管理 socket `admin.sock` 权限为 `0600`，因此必须使用与容器相同的用户（默认 `nonroot`），不要用 `--user root`。

## 数据持久化与权限

数据目录必须存在、是真实目录且权限精确为 `0700`；数据库文件为 `0600`。两种挂载方式：

- 命名卷：Docker 首次创建卷时继承镜像内目录的属主与权限，开箱可用。
- 绑定挂载：宿主机目录必须归 `65532:65532` 且 `0700`，否则启动报“数据目录必须为权限 0700 的真实目录”。

```bash
sudo install -d -m 0700 -o 65532 -g 65532 /srv/openjwc
```

```yaml
services:
  openjwc:
    volumes:
      - /srv/openjwc:/var/lib/openjwc
```

## 网络与 TLS

- 容器内监听 `0.0.0.0:8080`，`compose.yaml` 默认只发布到 `127.0.0.1:8080`。
- 公网部署应配置 `OPENJWC_TLS_CERT`、`OPENJWC_TLS_KEY`（挂载 PEM 文件并填写容器内路径），或由可信反向代理终止 TLS。
- 登录限流按直连来源地址计算，不信任任意 `X-Forwarded-For`，反向代理部署时注意来源地址保持。

## 外部爬虫

`OPENJWC_CRAWLER_PROGRAMS` 接受 1～16 个程序的 JSON 数组，只接受显式名称、规范化绝对路径与参数，不经过 shell。程序本体及其运行时必须一并放入镜像，路径与容器内实际位置一致。

## 版本、日志与备份

```bash
docker run --rm openjwc:local --version
docker compose logs -f            # 结构化 JSON 日志输出到 stderr
```

升级用 `docker compose up -d --build`，命名卷数据保留。在线备份通过 TUI 的「概览」→ `b` 完成，目标目录同样需要 `0700`；命名卷由 Docker 管理，建议把备份复制到卷外独立存储。

## 已知限制

- 容器内不使用 `openjwc service install` 等 systemd 管理命令，生命周期交给容器编排。
- 镜像未定义 `HEALTHCHECK`（distroless 无 curl/wget），请由编排层探测 `/healthz`。
