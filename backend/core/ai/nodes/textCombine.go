package nodes

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
)

type TextCombineNode struct {
	data map[string]any
}

func NewTextCombineNode(data map[string]any) *TextCombineNode {
	return &TextCombineNode{
		data: data,
	}
}

func (t *TextCombineNode) Invoke(ctx context.Context, input map[string]any) (map[string]any, error) {
	//这就是节点的实现逻辑
	//这里输入的数据有两个获取途径，一个是通过data字段，一个是通过input字段
	//data字段是节点的配置数据，input字段是节点的输入数据
	var templateStr string
	if value, ok := input["template"]; ok {
		templateStr = fieldValueToString(value)
	} else if value, ok := t.data["template"]; ok {
		templateStr = fieldValueToString(value)
	}
	if templateStr == "" {
		return nil, fmt.Errorf("textCombine template is empty")
	}
	//变量
	variablesMap := make(map[string]any)
	hasVariables := false
	if len(input) > 0 {
		for key, value := range input {
			if key != "template" {
				variablesMap[key] = fmt.Sprintf("%v", value)
				hasVariables = true
			}
		}
	}
	if !hasVariables {
		if rawVariables, ok := t.data["variables"]; ok {
			switch arr := rawVariables.(type) {
			case []any:
				for _, v := range arr {
					if m, ok := v.(map[string]any); ok {
						fieldName := fieldValueToString(m["fieldName"])
						if fieldName != "" {
							variablesMap[fieldName] = m["fieldValue"]
						}
					}
				}
			case map[string]any:
				for k, v := range arr {
					variablesMap[k] = fieldValueToString(v)
				}
			}
		}
	}
	//渲染模版
	var buf bytes.Buffer
	tmpl, err := template.New("TextCombine").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("parse template error: %v", err)
	}
	err = tmpl.Execute(&buf, variablesMap)
	if err != nil {
		return nil, fmt.Errorf("execute template error: %v", err)
	}
	result := map[string]any{
		"output": buf.String(),
	}
	for k, v := range variablesMap {
		result[k] = v
	}
	return result, nil
}

func fieldValueToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case map[string]any:
		if fieldValue, ok := v["fieldValue"]; ok {
			return fieldValueToString(fieldValue)
		}
	case nil:
		return ""
	}
	return fmt.Sprintf("%v", value)
}
