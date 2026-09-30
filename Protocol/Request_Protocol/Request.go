package requestprotocol

import (
	"encoding/json"

	responseprotocol "github.com/nuka-del/nuka-llm/Protocol/Response_Protocol"
	tools "github.com/nuka-del/nuka-llm/Protocol/Tool_Protocol"
)

type Message struct {
	Role             string
	Content          any
	ReasoningContent string
	ToolCalls        []responseprotocol.ToolCall
	ToolCallID       string
}

type Request struct {
	Model       string
	Messages    []Message
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stream      bool
	Stop        []string
	Tools       tools.Tools
}

type Parameter struct {
	name        string
	typeName    string
	description string
	required    bool
}

func NewRequest(model string, messages ...Message) Request {
	return Request{
		Model:    model,
		Messages: append([]Message(nil), messages...),
	}
}

func (r *Request) AddMessage(message Message) *Request {
	if r == nil {
		return nil
	}
	r.Messages = append(r.Messages, message)
	return r
}

func (r *Request) AddSystem(content any) *Request {
	return r.AddMessage(Message{Role: "system", Content: content})
}

func (r *Request) AddUserMessage(content any) *Request {
	return r.AddMessage(Message{Role: "user", Content: content})
}

func (r *Request) AddAssistantMessage(
	content any,
	toolCalls ...responseprotocol.ToolCall,
) *Request {
	return r.AddMessage(Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: append([]responseprotocol.ToolCall(nil), toolCalls...),
	})
}

func (r *Request) AddToolResult(toolCallID string, content any) *Request {
	return r.AddMessage(Message{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    content,
	})
}

func (r *Request) AddFunctionTool(
	name string,
	description string,
	parameters tools.Parameters,
) *Request {
	if r == nil {
		return nil
	}
	r.Tools.ToolList = append(r.Tools.ToolList, tools.Tool{
		ToolType: "function",
		Function: tools.Function{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	})
	return r
}

func NewObjectParameters(properties ...Parameter) tools.Parameters {
	parameterSchema := tools.Parameters{
		ParaType:         "object",
		SimpleProperties: make(map[string]tools.Property, len(properties)),
	}
	for _, property := range properties {
		parameterSchema.SimpleProperties[property.name] = tools.Property{
			PropertiesType: property.typeName,
			Description:    property.description,
		}
		if property.required {
			parameterSchema.Required = append(parameterSchema.Required, property.name)
		}
	}
	return parameterSchema
}

func String(name string, description string) Parameter {
	return Parameter{
		name:        name,
		typeName:    "string",
		description: description,
	}
}

func Number(name string, description string) Parameter {
	return Parameter{
		name:        name,
		typeName:    "number",
		description: description,
	}
}

func Integer(name string, description string) Parameter {
	return Parameter{
		name:        name,
		typeName:    "integer",
		description: description,
	}
}

func Boolean(name string, description string) Parameter {
	return Parameter{
		name:        name,
		typeName:    "boolean",
		description: description,
	}
}

func (p Parameter) Required() Parameter {
	p.required = true
	return p
}

func ContentText(content any) (string, error) {
	if text, ok := content.(string); ok {
		return text, nil
	}
	if content == nil {
		return "", nil
	}

	data, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
