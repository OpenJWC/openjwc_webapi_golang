# 服务部署与命令行

## 用户级部署（默认）

后台运行统一由 systemd 负责，不创建第二套守护进程或 PID 文件。`serve` **始终前台运行**，用于开发调试和 unit 的 ExecStart。

先将二进制放到稳定且可执行的位置，再完成一次安装：

```bash
openjwc -h
# 必要时由管理员明确授权，CLI 不会自动执行 sudo：
sudo loginctl enable-linger "$USER"
openjwc service install --now
openjwc status
openjwc status --json
openjwc stop
openjwc start
openjwc restart
openjwc service uninstall
```

`install` 启用服务但默认不立即启动，`--now` 才立即启动。缺少 user manager 或无法确认 linger 时拒绝宣称持久常驻已配置。`start` 不隐式安装。

用户 unit 为 `$XDG_CONFIG_HOME/systemd/user/openjwc.service`（默认 `~/.config/systemd/user/`）；私有部署记录为 `$XDG_CONFIG_HOME/openjwc/service.json`。安装保存绝对二进制路径、数据目录、监听地址、TLS 路径与显式外部爬虫配置。外部爬虫必须使用绝对路径，协议见 [crawler-protocol.md](crawler-protocol.md)。**不要移动或删除已部署的二进制或外部爬虫。** 后续 start/status/tui/recover 使用已保存的配置，不依赖 SSH 会话将环境变量传递给 systemd。前台 `serve` 则直接读取环境变量。

卸载停止服务、取消自启动并删除校验通过的自有 unit；保留部署配置、账号、数据库和备份。重复安装相同配置、重复卸载是幂等操作。外来同名 unit、内容摘要不符的 unit 或无法确认的来源不会被覆盖。改变二进制路径需先显式卸载再安装；改变启动配置需先卸载，再通过显式环境变量重新安装（未显式给出的字段保留部署配置），不会自动搬迁数据库。不要手工改写部署记录的内部字段。

TUI 首页可查看、安装、启停用户级服务；服务未运行仍可进入该页面。没有自动降级为非 systemd 后台进程。

## 系统级部署

系统级服务使用固定 `openjwc` 账号和 `/var/lib/openjwc` 数据目录。创建账号和授权由操作者明确执行：

```bash
sudo install -m 0755 bin/openjwc /usr/local/bin/openjwc
# 已有账号时不要重复创建：
sudo useradd --system --home-dir /var/lib/openjwc --shell /usr/sbin/nologin openjwc
sudo /usr/local/bin/openjwc service install --system --now
sudo openjwc status --system
sudo openjwc tui --system
sudo openjwc stop --system
sudo openjwc service uninstall --system
```

系统 unit 位于 `/etc/systemd/system/openjwc.service`，部署记录位于 `/etc/openjwc/service.json`。TUI 不执行系统级变更或静默提权；使用显式 CLI 完成授权操作。系统 unit 的 ProtectHome 会禁止从用户 home 启动服务，因此二进制应安装到 `/usr/local/bin` 等系统路径。证书和私钥必须能由服务账号读取。

`deploy/openjwc.service` 是手工部署参考，CLI 实际生成带明确配置的 unit。旧手工部署没有 CLI 的部署记录，不能自动接管；请先备份旧 unit 并明确处理冲突。

## 状态与输出约定

- `status` 区分未安装、停止、启动中、关闭中、运行未就绪、正常、失败及未知状态。
- 就绪同时要求 systemd MainPID 与该配置的私有 RPC 进程 ID 一致；不以 socket 文件存在证明健康。
- `start/restart` 等待就绪；`stop` 等待停止，均有超时；不根据未经验证的 PID 杀进程。
- 退出码：0 成功或就绪；1 操作/探测失败；2 参数用法错误；3 服务未就绪。
- `-h`、`--help`、子命令帮助、`--version` 不要求有效服务配置。
- 帮助的命令和选项共用按最长命令计算的说明列宽，不再手工补空格。
- 交互终端帮助和退出摘要显示 OpenJWC 块状字标：灰色 OPEN、薄荷色 JWC 和右下投影。字标上方固定留出一行；当前九行窄体点阵压缩为 38×5 个终端字符，避免旧五行点阵呈现为 43×3 时过宽、过扁。80 列起双宽显示，42 列以下或 TERM=dumb 使用普通名称。
- 交互式帮助以薄荷色突出 Usage、分节和提示标签，以灰色显示命令列，以粗体突出可复制命令；内容和说明保持终端默认色。重定向帮助没有 ANSI 控制码或字标；`--no-banner` 同时关闭帮助和退出字标，但不关闭其他交互配色。非空 NO_COLOR 禁用全部帮助和字标配色，保留单色块字；未声明 256 色或 COLORTERM 能力的终端也使用单色。
- TUI 退出恢复终端后显示字标、会话耗时、服务状态和重新进入命令，不回显密钥；服务状态来自有界查询，不能确认时提示运行 status。系统级会话的重新进入命令保留 --system。

字标视觉参考 OpenCode 的[退出展示实现](https://github.com/sst/opencode/blob/dev/packages/tui/src/util/presentation.ts)：Unicode 半块拼字、双色投影，以及在[恢复终端之后输出摘要](https://github.com/sst/opencode/blob/dev/packages/tui/src/app.tsx)。已核对其 MIT 许可；本项目独立绘制五行 OPENJWC 点阵，以两个像素编码一个终端字符，不复制上游字标或源码，也不引入 OpenTUI/TypeScript 依赖。

真实 systemd 安装和运行需要在目标环境验收。本轮测试使用假命令、临时部署路径及 unit 语法检查，不代表已经安装到当前主机。
