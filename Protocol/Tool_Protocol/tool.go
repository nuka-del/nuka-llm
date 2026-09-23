package toolprotocol

type Tool struct {
	ToolType string
	Function Function
}

type Function struct {
	Name        string
	Description string
	Parameters  Parameters
}

type Parameters struct {
	ParaType   string
	Properties map[string]Property
	Required   []string
}

type Property struct {
	PropertiesType string
}
