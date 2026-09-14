package tui

import (
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"time"
)

// field 描述一个管理表单字段及当前值。
type field struct {
	key    string
	label  string
	value  string
	secret bool
}

// actionSpec 描述管理动作需要的参数及标题。
type actionSpec struct {
	action admin.Action
	title  string
	fields []field
	target bool
}

// form 是仅归属于当前 TUI 的可变编辑状态。
type form struct {
	insert    bool
	multiline bool
	input     textinput.Model
	area      textarea.Model
	spec      actionSpec
	fields    []field
	cursor    int
}

// newForm 从选中记录填充目标，所有变更均增加最终确认字段。
func newForm(spec actionSpec, row []string) *form {
	result := &form{spec: spec, fields: append([]field(nil), spec.fields...)}
	if spec.target {
		value := ""
		if len(row) > 0 {
			value = row[0]
		}
		if spec.action == admin.ActionBackup || spec.action == admin.ActionRestore || spec.action == admin.ActionImportUsers {
			value = ""
		}
		if spec.action == admin.ActionDigest {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				value = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}
		}
		result.fields = append([]field{{key: "id", label: "目标 ID / 设置名 / 路径", value: value}}, result.fields...)
	}
	if spec.action == admin.ActionSetSetting && len(row) >= 2 {
		for index := range result.fields {
			if result.fields[index].key == "value" {
				if row[0] == "llm_api_key" {
					result.fields[index].secret = true
				} else {
					result.fields[index].value = row[1]
				}
			}
		}
	}
	result.fields = append(result.fields, field{key: "confirm", label: "输入 yes 确认执行"})
	return result
}

// request 将当前表单转换为类型化管理命令，不传输确认文本。
func (form *form) request() admin.Request {
	request := admin.Request{Action: form.spec.action, Values: make(map[string]string)}
	for _, field := range form.fields {
		switch field.key {
		case "id":
			request.ID = field.value
		case "confirm":
		default:
			request.Values[field.key] = field.value
		}
	}
	return request
}
