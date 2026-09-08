package toolprotocol

type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func NewToolInfo(name string, description string) ToolInfo {
	return ToolInfo{
		Name:        name,
		Description: description,
	}
}

type Tool interface {
	Info() ToolInfo
	Start() (string, error)
}
