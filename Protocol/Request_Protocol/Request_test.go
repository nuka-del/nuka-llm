package requestprotocol

import "testing"

func TestRequestBuilderBuildsMessagesAndFunctionTool(t *testing.T) {
	request := NewRequest("deepseek-v4-pro")
	request.AddSystem("Be concise.")
	request.AddUserMessage("Check the weather in Hangzhou.")
	request.AddFunctionTool(
		"get_weather",
		"Get current weather for a city.",
		NewObjectParameters(
			String("location", "City name").Required(),
			Number("days", "Forecast length in days"),
		),
	)

	if request.Model != "deepseek-v4-pro" {
		t.Errorf("Model = %q", request.Model)
	}
	if len(request.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2", len(request.Messages))
	}
	if request.Messages[0].Role != "system" || request.Messages[1].Role != "user" {
		t.Errorf("message roles = %q, %q", request.Messages[0].Role, request.Messages[1].Role)
	}
	if len(request.Tools.ToolList) != 1 {
		t.Fatalf("ToolList length = %d, want 1", len(request.Tools.ToolList))
	}
	functionTool := request.Tools.ToolList[0]
	if functionTool.ToolType != "function" || functionTool.Function.Name != "get_weather" {
		t.Errorf("function tool = %#v", functionTool)
	}
	parameters := functionTool.Function.Parameters
	if parameters.ParaType != "object" {
		t.Errorf("ParaType = %q, want object", parameters.ParaType)
	}
	if parameters.SimpleProperties["location"].PropertiesType != "string" {
		t.Errorf("location property = %#v", parameters.SimpleProperties["location"])
	}
	if parameters.SimpleProperties["days"].PropertiesType != "number" {
		t.Errorf("days property = %#v", parameters.SimpleProperties["days"])
	}
	if len(parameters.Required) != 1 || parameters.Required[0] != "location" {
		t.Errorf("Required = %#v, want [location]", parameters.Required)
	}
}
