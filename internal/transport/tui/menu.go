package tui

import "github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"

// menuSpec 定义一个管理页面及可用动作。
type menuSpec struct {
	title   string
	action  admin.Action
	hint    string
	actions map[string]actionSpec
}

// menus 是封闭的管理导航与表单定义。
var menus = []menuSpec{
	{title: "服务管理", action: admin.ActionServiceStatus, hint: "s 启动 · x 停止 · R 重启 · a 安装 · D 卸载",
		actions: map[string]actionSpec{
			"s": {action: admin.ActionServiceStart, title: "启动用户级服务"},
			"x": {action: admin.ActionServiceStop, title: "停止用户级服务"},
			"R": {action: admin.ActionServiceRestart, title: "重启用户级服务"},
			"a": {action: admin.ActionServiceInstall, title: "安装并启用用户级服务"},
			"D": {action: admin.ActionServiceUninstall, title: "卸载用户级服务（保留数据）"},
		}},
	{
		title: "概览", action: admin.ActionStats,
		hint: "b 备份 · c 校验 · e 重建索引 · i 迁移旧用户",
		actions: map[string]actionSpec{
			"e": {action: admin.ActionRebuildIndex, title: "从资讯表重建全文索引"},
			"b": {action: admin.ActionBackup, title: "创建已验证备份", target: true},
			"c": {action: admin.ActionCheck, title: "数据库完整性检查"},
			"i": {action: admin.ActionImportUsers, title: "只读导入旧 SQLite 用户态", target: true},
		},
	},
	{
		title: "注册审核", action: admin.ActionRegistrations,
		hint: "e 审核选中申请",
		actions: map[string]actionSpec{
			"e": {
				action: admin.ActionReviewRegistration, title: "审核注册申请", target: true,
				fields: []field{
					{key: "decision", label: "结果 approved / rejected", value: "approved"},
					{key: "review", label: "审核意见"},
				},
			},
		},
	},
	{
		title: "用户权限", action: admin.ActionUsers,
		hint: "e 配置权限 · d 删除用户",
		actions: map[string]actionSpec{
			"e": {
				action: admin.ActionPermissions, title: "用户权限设置", target: true,
				fields: []field{
					{key: "active", label: "启用 true / false", value: "true"},
					{key: "browse", label: "浏览资讯 true / false", value: "true"},
					{key: "chat", label: "智能问答 true / false", value: "false"},
					{key: "submit", label: "投稿 true / false", value: "true"},
					{key: "max_devices", label: "设备上限 1～100", value: "3"},
				},
			},
			"d": {action: admin.ActionDeleteUser, title: "永久删除用户及设备会话", target: true},
		},
	},
	{
		title: "API Key", action: admin.ActionKeys,
		hint: "a 新建 · b 关联用户 · e 启停 · d 删除",
		actions: map[string]actionSpec{
			"b": {
				action: admin.ActionLinkKey, title: "关联用户并清除旧设备绑定", target: true,
				fields: []field{{key: "user_id", label: "关联用户 ID（继承该用户权限）"}},
			},
			"a": {
				action: admin.ActionCreateKey, title: "创建 API Key",
				fields: []field{
					{key: "owner", label: "所有者"},
					{key: "max_devices", label: "设备上限 1～100", value: "3"},
				},
			},
			"e": {
				action: admin.ActionKeyStatus, title: "API Key 启停", target: true,
				fields: []field{{key: "active", label: "启用 true / false", value: "true"}},
			},
			"d": {action: admin.ActionDeleteKey, title: "永久删除 API Key", target: true},
		},
	},
	{
		title: "资讯", action: admin.ActionNotices,
		hint: "Enter 查看全文 · d 删除 · c 立即爬取 · S 生成日报",
		actions: map[string]actionSpec{
			"d": {action: admin.ActionDeleteNotice, title: "永久删除资讯", target: true},
			"c": {action: admin.ActionCrawl, title: "运行资讯爬虫"},
			"S": {action: admin.ActionDigest, title: "生成指定日期日报", target: true},
		},
	},
	{
		title: "投稿审核", action: admin.ActionSubmissions,
		hint: "Enter 查看全文 · e 审核选中投稿",
		actions: map[string]actionSpec{
			"e": {
				action: admin.ActionReviewSubmission, title: "审核并发布投稿", target: true,
				fields: []field{
					{key: "decision", label: "结果 approved / rejected", value: "approved"},
					{key: "review", label: "审核意见"},
				},
			},
		},
	},
	{
		title: "系统设置", action: admin.ActionSettings,
		hint: "e 编辑 · d 重置全部设置",
		actions: map[string]actionSpec{
			"e": {
				action: admin.ActionSetSetting, title: "修改系统设置", target: true,
				fields: []field{{key: "value", label: "值（Ctrl+u 清空）"}},
			},
			"d": {action: admin.ActionResetSettings, title: "重置全部设置（含模型凭据）"},
		},
	},
	{
		title: "日报", action: admin.ActionReports,
		hint: "Enter 查看全文 · S 按日期生成",
		actions: map[string]actionSpec{
			"S": {action: admin.ActionDigest, title: "生成日报（目标填写 YYYY-MM-DD）", target: true},
		},
	},
	{title: "爬虫任务", action: admin.ActionCrawlProgress, hint: "c 启动后台爬虫 · x 取消当前任务 · r 刷新进度",
		actions: map[string]actionSpec{
			"c": {action: admin.ActionCrawl, title: "启动服务拥有的爬虫任务"},
			"x": {action: admin.ActionCrawlCancel, title: "取消当前爬虫任务", target: true},
		}},

	{
		title: "运行日志", action: admin.ActionLogs,
		hint:    "最近 200 条安全摘要；完整结构化日志发送到 stderr",
		actions: map[string]actionSpec{},
	},
	{
		title: "审计日志", action: admin.ActionAudit,
		hint:    "仅记录管理动作，不记录密钥或提示词",
		actions: map[string]actionSpec{},
	},
}
