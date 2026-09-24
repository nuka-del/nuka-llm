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
	SimpleProperties map[string]Property
	ConplexProproties map[string]any
	Required   []string
}

type Property struct {
	PropertiesType string
	Description string
}
