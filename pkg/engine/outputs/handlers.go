package outputs

import (
	"context"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/apiconfig"
	"github.com/Servflow/servflow/pkg/engine/requestctx"
	"github.com/Servflow/servflow/pkg/engine/responses"
)

// The built-in output handler kinds.
const (
	// HandlerTemplate renders a template against the finished request context.
	// It is what the legacy result expressions (trigger result, MCP tool result,
	// workflow tool returnValue) do, expressed as a handler.
	HandlerTemplate = "template"
	// HandlerConversation extracts a message from the run's conversation thread.
	// It is the handler a workflow uses when its output is simply "what the
	// agent said".
	HandlerConversation = "conversation"
	// HandlerFile returns a file from the request context.
	HandlerFile = "file"
	// HandlerNone produces no output at all, explicitly.
	HandlerNone = "none"
)

func init() {
	Register(HandlerTemplate, newTemplateExtractor)
	Register(HandlerConversation, newConversationExtractor)
	Register(HandlerFile, newFileExtractor)
	Register(HandlerNone, func(map[string]interface{}) (Extractor, error) {
		return ExtractorFunc(func(context.Context) (responses.Result, error) {
			return nil, nil
		}), nil
	})
}

// ---------------------------------------------------------------- template ---

type templateExtractor struct {
	template string
}

// Template returns an extractor that renders tmpl against the request context.
// It is exported so the run entry points can build the same extractor from
// their deprecated result fields without routing a config map through the
// registry.
func Template(tmpl string) Extractor {
	return &templateExtractor{template: tmpl}
}

func newTemplateExtractor(cfg map[string]interface{}) (Extractor, error) {
	tmpl, err := stringField(cfg, "template", true)
	if err != nil {
		return nil, err
	}
	return Template(tmpl), nil
}

// Extract renders the configured template. The rendered value is NOT scrubbed:
// the template is written by the config author, so a secret reaching the output
// through it is deliberate (a workflow that forwards a token to its caller is a
// legitimate shape). Contrast the conversation handler, which reads text the
// author never wrote.
func (e *templateExtractor) Extract(ctx context.Context) (responses.Result, error) {
	if e.template == "" {
		return nil, nil
	}
	out, err := requestctx.ExecuteTemplateString(ctx, e.template)
	if err != nil {
		return nil, fmt.Errorf("rendering output template: %w", err)
	}
	return TextResult{Text: out}, nil
}

// ------------------------------------------------------------ conversation ---

type conversationExtractor struct {
	// role selects which message to return; empty means any role.
	role requestctx.RoleType
}

// Conversation returns an extractor for the last thread message with the given
// role. An empty role matches any role.
func Conversation(role requestctx.RoleType) Extractor {
	return &conversationExtractor{role: role}
}

func newConversationExtractor(cfg map[string]interface{}) (Extractor, error) {
	raw, err := stringField(cfg, "role", false)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		raw = "assistant"
	}
	role, err := parseRole(raw)
	if err != nil {
		return nil, err
	}
	return Conversation(role), nil
}

// Extract returns the text of the last message in the run's conversation thread
// matching the configured role. Tool calls and tool results are skipped: they
// are the agent's working traffic, not its answer. An empty or absent thread
// yields no output rather than an error — a run whose agent never spoke has
// nothing to return.
//
// The text IS scrubbed of secrets resolved during the request. Unlike a
// template, the thread carries model and tool output that no config author
// wrote, so a credential that leaked into a tool result would otherwise be
// handed straight to the caller.
func (e *conversationExtractor) Extract(ctx context.Context) (responses.Result, error) {
	rc, ok := requestctx.FromContext(ctx)
	if !ok {
		return nil, nil
	}
	msgs := rc.Conversation().Messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		content, ok := msgs[i].(requestctx.MessageTypeContent)
		if !ok {
			continue
		}
		if e.role != requestctx.RoleTypeUnknown && content.Role != e.role {
			continue
		}
		text := content.Content
		if rc.HasSecrets() {
			text = rc.Scrub(text)
		}
		return TextResult{Text: text}, nil
	}
	return nil, nil
}

// parseRole maps a configured role name onto a RoleType. The empty name is
// rejected here (the factory defaults it first) so a typo cannot silently widen
// the selection to "any role".
func parseRole(name string) (requestctx.RoleType, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "any", "*":
		return requestctx.RoleTypeUnknown, nil
	case "assistant":
		return requestctx.RoleTypeAssistant, nil
	case "user":
		return requestctx.RoleTypeUser, nil
	case "system":
		return requestctx.RoleTypeSystem, nil
	case "developer":
		return requestctx.RoleTypeDeveloper, nil
	default:
		return requestctx.RoleTypeUnknown, fmt.Errorf("unknown role %q: want one of assistant, user, system, developer, any", name)
	}
}

// -------------------------------------------------------------------- file ---

type fileExtractor struct {
	file apiconfig.FileInput
}

// File returns an extractor for a file held in the request context.
func File(in apiconfig.FileInput) Extractor {
	return &fileExtractor{file: in}
}

func newFileExtractor(cfg map[string]interface{}) (Extractor, error) {
	identifier, err := stringField(cfg, "identifier", true)
	if err != nil {
		return nil, err
	}
	fileType, err := stringField(cfg, "type", false)
	if err != nil {
		return nil, err
	}
	if fileType == "" {
		fileType = apiconfig.FileInputTypeAction
	}
	return File(apiconfig.FileInput{Type: fileType, Identifier: identifier}), nil
}

func (e *fileExtractor) Extract(ctx context.Context) (responses.Result, error) {
	f, err := requestctx.GetFileFromContext(ctx, e.file)
	if err != nil {
		return nil, fmt.Errorf("reading output file %q: %w", e.file.Identifier, err)
	}
	return FileResult{File: f}, nil
}

// ------------------------------------------------------------------ config ---

// stringField reads a string field from a handler config, reporting a typed
// error rather than silently ignoring a mistyped value.
func stringField(cfg map[string]interface{}, key string, required bool) (string, error) {
	v, ok := cfg[key]
	if !ok || v == nil {
		if required {
			return "", fmt.Errorf("missing required field %q", key)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q must be a string, got %T", key, v)
	}
	if required && s == "" {
		return "", fmt.Errorf("field %q must not be empty", key)
	}
	return s, nil
}
