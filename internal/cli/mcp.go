package cli

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type readInput struct {
	Operation string `json:"operation"`
	Path      string `json:"path,omitempty"`
	Filter    string `json:"filter,omitempty"`
	Number    int    `json:"number,omitempty"`
	Human     bool   `json:"human,omitempty"`
}

type queueInput struct {
	JobPath    string            `json:"job_path"`
	Parameters map[string]string `json:"parameters,omitempty"`
	WriteOptIn bool              `json:"write_opt_in,omitempty"`
	Human      bool              `json:"human,omitempty"`
}

func NewMCPServer(deps Deps) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "jkins", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "jkins_read", Description: "Read Jenkins jobs, builds, or console logs. Operations: job_list, job_get, build_get, build_log. Job list requires filter or path; build reads require path and number."}, func(_ context.Context, _ *mcp.CallToolRequest, input readInput) (*mcp.CallToolResult, any, error) {
		args := []string{}
		if input.Human {
			args = append(args, "--human")
		}
		switch input.Operation {
		case "job_list":
			if (input.Filter == "") == (input.Path == "") {
				return mcpError("job_list requires exactly one of filter or path"), nil, nil
			}
			args = append(args, "job", "list")
			if input.Filter != "" {
				args = append(args, "--filter", input.Filter)
			} else {
				args = append(args, "--path", input.Path)
			}
		case "job_get":
			args = append(args, "job", "get", input.Path)
		case "build_get", "build_log":
			verb := "get"
			if input.Operation == "build_log" {
				verb = "log"
			}
			args = append(args, "build", verb, input.Path, strconv.Itoa(input.Number))
		default:
			return mcpError("unknown read operation"), nil, nil
		}
		return mcpRun(deps, args...), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "jkins_help", Description: "Show jkins command help."}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return mcpRun(deps, "help"), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "jkins_status", Description: "Show stored credential presence and user only."}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return mcpRun(deps, "auth", "status"), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "jkins_build_queue", Description: "Preview an explicit job target and parameters by default. Set write_opt_in=true to queue exactly one build; deployment targets must be explicit."}, func(_ context.Context, _ *mcp.CallToolRequest, input queueInput) (*mcp.CallToolResult, any, error) {
		args := []string{}
		if input.Human {
			args = append(args, "--human")
		}
		args = append(args, "build", "queue", input.JobPath)
		keys := make([]string, 0, len(input.Parameters))
		for key := range input.Parameters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			args = append(args, "--param", key+"="+input.Parameters[key])
		}
		if input.WriteOptIn {
			args = append(args, "--execute")
		}
		return mcpRun(deps, args...), nil, nil
	})
	return server
}

func mcpError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}, IsError: true}
}

func mcpRun(deps Deps, args ...string) *mcp.CallToolResult {
	var stdout, stderr bytes.Buffer
	deps.Out, deps.Err = &stdout, &stderr
	code := Run(args, deps)
	content := stdout.String()
	if code != 0 {
		content = stderr.String()
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: content}}, IsError: code != 0}
}

type noCloseWriter struct{ io.Writer }

func (noCloseWriter) Close() error { return nil }

func runMCP(deps Deps) int {
	if deps.In == nil {
		deps.In = strings.NewReader("")
	}
	transport := &mcp.IOTransport{Reader: io.NopCloser(deps.In), Writer: noCloseWriter{deps.Out}}
	if err := NewMCPServer(deps).Run(context.Background(), transport); err != nil {
		return fail(deps.Err, "mcp serve", err)
	}
	return 0
}
