package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ricelines/matrix-mcp/internal/catalog"
	"github.com/ricelines/matrix-mcp/internal/config"
	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
	"github.com/ricelines/matrix-mcp/internal/modules"
	"github.com/ricelines/matrix-mcp/internal/scopes"
)

const (
	resourceModules = "matrix://modules"
	resourceScopes  = "matrix://scopes"
)

type Server struct {
	server *mcp.Server
	tools  []catalog.ToolMeta
	mods   []catalog.ModuleMeta
	scopes scopes.Set
}

func NewFromConfig(ctx context.Context, cfg config.Config) (*Server, error) {
	matrix, err := matrixclient.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return New(matrix, cfg.Scopes), nil
}

func New(matrix matrixclient.API, active scopes.Set) *Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "matrix-mcp", Version: "0.1.0"}, nil)
	registrar := catalog.NewRegistrar(server)
	modules.RegisterAll(registrar, matrix, active)

	result := &Server{server: server, tools: registrar.Tools(), mods: registrar.Modules(), scopes: active}
	result.registerResources()
	return result
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.server
	}, nil)
}

func (s *Server) Raw() *mcp.Server {
	return s.server
}

func (s *Server) registerResources() {
	s.server.AddResource(&mcp.Resource{
		URI:         resourceModules,
		Name:        "Matrix MCP modules",
		Description: "Top-level module index for recursive tool discovery.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return textResource(req.Params.URI, s.renderModulesRoot()), nil
	})

	s.server.AddResource(&mcp.Resource{
		URI:         resourceScopes,
		Name:        "Matrix MCP scopes",
		Description: "Active and available scope configuration.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return textResource(req.Params.URI, s.renderScopes()), nil
	})

	for _, module := range s.mods {
		moduleName := module.Name
		if !s.moduleHasTools(moduleName) {
			continue
		}
		s.server.AddResource(&mcp.Resource{
			URI:         "matrix://module/" + moduleName,
			Name:        "Matrix MCP module " + moduleName,
			Description: "Detailed information for one module, including its tools.",
			MIMEType:    "text/markdown",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			body, err := s.renderModule(moduleName)
			if err != nil {
				return nil, err
			}
			return textResource(req.Params.URI, body), nil
		})
	}

	for _, tool := range s.tools {
		toolName := tool.Name
		s.server.AddResource(&mcp.Resource{
			URI:         "matrix://tool/" + toolName,
			Name:        "Matrix MCP tool " + toolName,
			Description: "Detailed information for one directly callable tool.",
			MIMEType:    "text/markdown",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			body, err := s.renderTool(toolName)
			if err != nil {
				return nil, err
			}
			return textResource(req.Params.URI, body), nil
		})
	}

	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "matrix://module/{module}",
		Name:        "Matrix MCP module detail",
		Description: "Detailed information for one module, including its tools.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		module, err := parseTemplateValue(req.Params.URI, "module")
		if err != nil {
			return nil, err
		}
		body, err := s.renderModule(module)
		if err != nil {
			return nil, err
		}
		return textResource(req.Params.URI, body), nil
	})

	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "matrix://tool/{name}",
		Name:        "Matrix MCP tool detail",
		Description: "Detailed information for one directly callable tool.",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		name, err := parseTemplateValue(req.Params.URI, "name")
		if err != nil {
			return nil, err
		}
		body, err := s.renderTool(name)
		if err != nil {
			return nil, err
		}
		return textResource(req.Params.URI, body), nil
	})
}

func (s *Server) moduleHasTools(name string) bool {
	for _, tool := range s.tools {
		if tool.Module == name {
			return true
		}
	}
	return false
}

func textResource(uri, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: text}}}
}

func parseTemplateValue(rawURI, host string) (string, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", err
	}
	if parsed.Host != host {
		return "", mcp.ResourceNotFoundError(rawURI)
	}
	value := strings.TrimPrefix(parsed.Path, "/")
	if value == "" {
		return "", mcp.ResourceNotFoundError(rawURI)
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return "", err
	}
	return decoded, nil
}

func (s *Server) renderModulesRoot() string {
	var out strings.Builder
	out.WriteString("# Matrix MCP modules\n\n")
	out.WriteString("Use `matrix://module/<name>` to drill into a module, then call tools directly by name once you know what you want.\n\n")
	for _, module := range s.mods {
		toolCount := 0
		for _, tool := range s.tools {
			if tool.Module == module.Name {
				toolCount++
			}
		}
		if toolCount == 0 {
			continue
		}
		fmt.Fprintf(&out, "- `%s`: %s Resource: `matrix://module/%s`\n", module.Name, module.Description, module.Name)
	}
	return out.String()
}

func (s *Server) renderScopes() string {
	var out strings.Builder
	out.WriteString("# Matrix MCP scopes\n\n")
	out.WriteString("## Active scopes\n")
	for _, name := range s.scopes.Names() {
		fmt.Fprintf(&out, "- `%s`\n", name)
	}
	out.WriteString("\n## Default scope expansion\n")
	for _, name := range scopes.DefaultNames() {
		fmt.Fprintf(&out, "- `%s`\n", name)
	}
	out.WriteString("\n## Available scopes\n")
	for _, info := range scopes.Available() {
		fmt.Fprintf(&out, "- `%s`: %s\n", info.Name, info.Description)
	}
	return out.String()
}

func (s *Server) renderModule(name string) (string, error) {
	var module *catalog.ModuleMeta
	for i := range s.mods {
		if s.mods[i].Name == name {
			module = &s.mods[i]
			break
		}
	}
	if module == nil {
		return "", mcp.ResourceNotFoundError("matrix://module/" + name)
	}

	tools := make([]catalog.ToolMeta, 0)
	for _, tool := range s.tools {
		if tool.Module == name {
			tools = append(tools, tool)
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

	var out strings.Builder
	fmt.Fprintf(&out, "# Module `%s`\n\n%s\n\n", module.Name, module.Description)
	out.WriteString("## Tools\n")
	for _, tool := range tools {
		fmt.Fprintf(&out, "- `%s`\n", tool.Name)
		fmt.Fprintf(&out, "  Scope: `%s`\n", tool.Scope)
		fmt.Fprintf(&out, "  Purpose: %s\n", tool.Description)
		fmt.Fprintf(&out, "  Detail: `matrix://tool/%s`\n", url.PathEscape(tool.Name))
	}
	return out.String(), nil
}

func (s *Server) renderTool(name string) (string, error) {
	for _, tool := range s.tools {
		if tool.Name != name {
			continue
		}
		var out strings.Builder
		fmt.Fprintf(&out, "# Tool `%s`\n\n", tool.Name)
		fmt.Fprintf(&out, "- Module: `%s`\n", tool.Module)
		fmt.Fprintf(&out, "- Scope: `%s`\n", tool.Scope)
		fmt.Fprintf(&out, "- Purpose: %s\n\n", tool.Description)
		out.WriteString("## Direct call\n")
		fmt.Fprintf(&out, "Call this tool directly via `tools/call` with `name=%q`.\n\n", tool.Name)
		out.WriteString("## Input schema\n```json\n")
		out.WriteString(prettySchema(tool.InputSchema))
		out.WriteString("\n```\n")
		if len(tool.OutputSchema) > 0 {
			out.WriteString("\n## Output schema\n```json\n")
			out.WriteString(prettySchema(tool.OutputSchema))
			out.WriteString("\n```\n")
		}
		return out.String(), nil
	}
	return "", mcp.ResourceNotFoundError("matrix://tool/" + name)
}

func prettySchema(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
