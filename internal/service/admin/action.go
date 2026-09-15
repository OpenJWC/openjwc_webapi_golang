package admin

// Action 定义本地管理协议的封闭动作集合，线协议值保持稳定。
type Action string

const (
	ActionCrawlProgress Action = "crawl-progress"
	ActionCrawlCancel   Action = "crawl-cancel"

	ActionServiceStatus    Action = "service-status"
	ActionServiceStart     Action = "service-start"
	ActionServiceStop      Action = "service-stop"
	ActionServiceRestart   Action = "service-restart"
	ActionServiceInstall   Action = "service-install"
	ActionServiceUninstall Action = "service-uninstall"

	ActionUsers              Action = "users"
	ActionRegistrations      Action = "registrations"
	ActionReviewRegistration Action = "review-registration"
	ActionPermissions        Action = "permissions"
	ActionDeleteUser         Action = "delete-user"
	ActionKeys               Action = "keys"
	ActionCreateKey          Action = "create-key"
	ActionKeyStatus          Action = "key-status"
	ActionDeleteKey          Action = "delete-key"
	ActionSettings           Action = "settings"
	ActionSetSetting         Action = "set-setting"
	ActionResetSettings      Action = "reset-settings"
	ActionNotices            Action = "notices"
	ActionDeleteNotice       Action = "delete-notice"
	ActionSubmissions        Action = "submissions"
	ActionReviewSubmission   Action = "review-submission"
	ActionStats              Action = "stats"
	ActionAudit              Action = "audit"
	ActionCheck              Action = "check"
	ActionBackup             Action = "backup"
	ActionCrawl              Action = "crawl"
	ActionCrawlerConfig      Action = "crawler-config"
	ActionSetCrawlerConfig   Action = "set-crawler-config"
	ActionDigest             Action = "digest"
	ActionImportUsers        Action = "import-users"
	ActionReports            Action = "reports"
	ActionLinkKey            Action = "link-key"
	ActionRebuildIndex       Action = "rebuild-index"
	ActionLogs               Action = "logs"
	ActionNoticeDetail       Action = "notice-detail"
	ActionSubmissionDetail   Action = "submission-detail"
	ActionReportDetail       Action = "report-detail"
	ActionRestore            Action = "restore"
)

// Valid 判断动作是否允许通过在线管理入口执行，离线恢复另行校验。
func (action Action) Valid() bool {
	switch action {
	case ActionCrawlProgress, ActionCrawlCancel, ActionUsers, ActionRegistrations, ActionReviewRegistration, ActionPermissions, ActionDeleteUser,
		ActionKeys, ActionCreateKey, ActionKeyStatus, ActionDeleteKey, ActionSettings, ActionSetSetting,
		ActionResetSettings, ActionNotices, ActionDeleteNotice, ActionSubmissions, ActionReviewSubmission,
		ActionStats, ActionAudit, ActionCheck, ActionBackup, ActionCrawl, ActionDigest, ActionImportUsers,
		ActionReports, ActionLinkKey, ActionRebuildIndex, ActionLogs, ActionNoticeDetail,
		ActionSubmissionDetail, ActionReportDetail, ActionCrawlerConfig, ActionSetCrawlerConfig:
		return true
	default:
		return false
	}
}
