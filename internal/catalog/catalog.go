package catalog

import (
	"encoding/json"
	"sort"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

type ModuleMeta struct {
	Name        string
	Description string
}

type ToolMeta struct {
	Name         string
	Description  string
	Module       string
	Scope        scopes.Scope
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
}

type Registrar struct {
	server  *mcp.Server
	modules map[string]ModuleMeta
	tools   []ToolMeta
}

func NewRegistrar(server *mcp.Server) *Registrar {
	return &Registrar{
		server:  server,
		modules: make(map[string]ModuleMeta),
	}
}

func (r *Registrar) AddModule(name, description string) {
	r.modules[name] = ModuleMeta{Name: name, Description: description}
}

func AddTool[In, Out any](r *Registrar, module string, scope scopes.Scope, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(r.server, tool, handler)
	inputSchema := tool.InputSchema
	if inputSchema == nil {
		if inferred, err := jsonschema.For[In](nil); err == nil {
			inputSchema = inferred
		}
	}
	outputSchema := tool.OutputSchema
	if outputSchema == nil {
		if inferred, err := jsonschema.For[Out](nil); err == nil {
			outputSchema = inferred
		}
	}
	r.tools = append(r.tools, ToolMeta{
		Name:         tool.Name,
		Description:  tool.Description,
		Module:       module,
		Scope:        scope,
		InputSchema:  marshalSchema(inputSchema),
		OutputSchema: marshalSchema(outputSchema),
	})
}

func (r *Registrar) Modules() []ModuleMeta {
	modules := make([]ModuleMeta, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})
	return modules
}

func (r *Registrar) Tools() []ToolMeta {
	tools := make([]ToolMeta, len(r.tools))
	copy(tools, r.tools)
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

func marshalSchema(schema any) json.RawMessage {
	if schema == nil {
		return nil
	}
	raw, err := json.Marshal(schema)
	if err != nil || string(raw) == "null" {
		return nil
	}
	clone := make([]byte, len(raw))
	copy(clone, raw)
	return clone
}
