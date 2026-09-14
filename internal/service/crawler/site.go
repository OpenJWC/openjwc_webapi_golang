package crawler

// category 定义官方站点的栏目路径与标签。
type category struct {
	label string
	path  string
}

// site 保存白名单站点及其正文选择规则。
type site struct {
	host       string
	bodyClass  string
	categories []category
}

// sites 追踪 OpenJWC/JwcCrawler 的三个官方资讯来源。
var sites = map[string]site{
	"jwc": {
		host:      "jwc.seu.edu.cn",
		bodyClass: "Article_Content",
		categories: []category{
			{label: "最新动态", path: "/zxdt/list.htm"},
			{label: "教务信息", path: "/jwxx/list.htm"},
			{label: "学籍管理", path: "/xjgl/list.htm"},
			{label: "教学研究", path: "/jxyj/list.htm"},
			{label: "实践教学", path: "/sjjx/list.htm"},
			{label: "国际交流", path: "/gjjl/list.psp"},
			{label: "文化素质教育", path: "/cbxx/list.htm"},
		},
	},
	"cs": {
		host:      "cs.seu.edu.cn",
		bodyClass: "wp_articlecontent",
		categories: []category{
			{label: "学院新闻", path: "/news/list.htm"},
			{label: "通知公告", path: "/49342/list.htm"},
			{label: "学术活动", path: "/xshd_53564/list.htm"},
		},
	},
	"xsxy": {
		host:      "xsxy.seu.edu.cn",
		bodyClass: "wp_articlecontent",
		categories: []category{
			{label: "新闻动态", path: "/57140/list.htm"},
			{label: "通知公告", path: "/57141/list.htm"},
			{label: "人才培养", path: "/57151/list.htm"},
		},
	},
}
