package toolprotocol

import (
	"encoding/json"
)

type ToolList struct {
	tools map[string]Tool
}

func New() *ToolList {
	return &ToolList{
		tools: make(map[string]Tool),
	}
}

func (list *ToolList) Add(t Tool) {
	info := t.Info()
	list.tools[info.Name] = t
}

func (list *ToolList) Start(nameList []string) {
	for _, name := range nameList {
		t, exists := list.tools[name]
		if !exists {
			continue
		}

		t.Start()
	}
}

func (list *ToolList) Definitions() ([]byte, error) {
	infos := make([]ToolInfo, 0, len(list.tools))

	for _, tool := range list.tools {
		infos = append(infos, tool.Info())
	}

	return json.Marshal(infos)
}
