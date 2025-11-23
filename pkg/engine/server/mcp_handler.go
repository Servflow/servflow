package server

import (
	"context"
	"errors"
	"fmt"
	"text/template"
	"time"

	apiconfig "github.com/Servflow/servflow/pkg/definitions"
	"github.com/Servflow/servflow/pkg/engine/plan"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

func (e *Engine) createMCPHandler(config *apiconfig.APIConfig) error {
	logger := e.logger.With(zap.String("type", "mcp"), zap.String("tool", config.McpTool.Name))
	//generate plan
	planner := plan.NewPlannerV2(plan.PlannerConfig{
		Actions:    config.Actions,
		Conditions: config.Conditionals,
	}, logger)

	p, err := planner.Plan()
	if err != nil {
		return fmt.Errorf("could not generate plan: %v", err)
	}

	if e.mcpServer == nil {
		e.mcpServer = server.NewMCPServer(
			"Servflow MCP",
			mcpServerVersion,
			server.WithPromptCapabilities(false),
			server.WithToolCapabilities(true),
			server.WithResourceCapabilities(false, false),
		)
	}

	options := []mcp.ToolOption{
		mcp.WithDescription(config.McpTool.Description),
	}
	for _, a := range config.McpTool.Args {
		switch a.Type {
		case "string":
			options = append(options, mcp.WithString(a.Name, mcp.Required()))
		case "number":
			options = append(options, mcp.WithNumber(a.Name, mcp.Required()))
		default:
			return fmt.Errorf("unsupported tool type: %s", a.Type)
		}
	}

	e.mcpServer.AddTool(mcp.NewTool(config.McpTool.Name, options...), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		logger := logger.With(zap.String("toolName", config.McpTool.Name))

		reqCtx, ok := requestctx.FromContext(ctx)
		if !ok {
			logger.Error("could not get request context")
			return nil, fmt.Errorf("error handling request")
		}

		// TODO add unit test for this
		reqCtx.AddRequestTemplateFunctions(template.FuncMap{
			"tool_param": func(key string) string {
				args := request.GetArguments()
				r, ok := args[key].(string)
				if !ok {
					return ""
				}

				return r
			},
		})

		resp, err := p.Execute(ctx, config.McpTool.Start, config.McpTool.Result)
		if err != nil {
			logger.Error("error executing planner", zap.Error(err))
			return nil, errors.New("error executing request")
		}

		// TODO support other types
		response := mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Text: string(resp.Body),
					Type: "text",
				},
			},
		}
		timeTaken := time.Since(start)
		logger.Debug("Response timeTaken", zap.String("timeTaken", timeTaken.String()))

		return &response, nil
	})

	logger.Debug("registered mcp handler")

	return nil
}
