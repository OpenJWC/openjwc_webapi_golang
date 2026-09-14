package tui

import (
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"strconv"
)

// userTable 只负责具名用户投影的展示列，授权编辑不读取这些字符串。
func userTable(response admin.Response) admin.Response {
	response.Columns = []string{"ID", "用户名", "邮箱", "启用", "资讯", "问答", "投稿", "设备上限"}
	response.Rows = make([][]string, 0, len(response.Users))
	for _, user := range response.Users {
		response.Rows = append(response.Rows, []string{
			user.ID, user.Username, user.Email,
			strconv.FormatBool(user.Active), strconv.FormatBool(user.Browse),
			strconv.FormatBool(user.Chat), strconv.FormatBool(user.Submit),
			strconv.Itoa(user.MaxDevices),
		})
	}
	return response
}

// fillPermissions 从具名权限投影填写表单，不依赖 SQL 或展示列的位置。
func (form *form) fillPermissions(user admin.UserPermissions) {
	values := map[string]string{
		"id": user.ID, "active": strconv.FormatBool(user.Active),
		"browse": strconv.FormatBool(user.Browse), "chat": strconv.FormatBool(user.Chat),
		"submit": strconv.FormatBool(user.Submit), "max_devices": strconv.Itoa(user.MaxDevices),
	}
	for index := range form.fields {
		if value, found := values[form.fields[index].key]; found {
			form.fields[index].value = value
		}
	}
}
