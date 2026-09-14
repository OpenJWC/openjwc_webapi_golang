package license

import _ "embed"

// ThirdParty 将运行时第三方许可证随独立二进制分发。
//
//go:embed third_party.txt
var ThirdParty string

// Crawler 保留移植栏目配置来源仓库的许可证。
//
//go:embed crawler.txt
var Crawler string
