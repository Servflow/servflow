package requestctx

import (
	"context"
	"strings"
	"testing"
)

// FromContextConversation fetches the conversation Start installed in ctx.
func FromContextConversation(t *testing.T, ctx context.Context) *Conversation {
	t.Helper()
	rc, err := FromContextOrError(ctx)
	if err != nil {
		t.Fatalf("no request context: %v", err)
	}
	conv := rc.Conversation()
	if conv == nil {
		t.Fatal("request context has no conversation")
	}
	return conv
}

func textMsg(content string) MessageTypeContent {
	return MessageTypeContent{
		Message: Message{Type: MessageTypeText},
		Role:    RoleTypeUser,
		Content: content,
	}
}

func contents(msgs []ConversationMessage) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if c, ok := m.(MessageTypeContent); ok {
			out = append(out, c.Content)
		}
	}
	return out
}

// TestConversation_IncrementalFlushAndResume verifies that (a) flush only writes
// the tail appended since the previous flush, and (b) resuming an id loads the
// full thread exactly once — the load-time high-water mark prevents the resumed
// history from being re-persisted on the next flush.
func TestConversation_IncrementalFlushAndResume(t *testing.T) {
	const id = "test-sync-incremental"

	conv := NewConversation(id)
	conv.Append(textMsg("one"), textMsg("two"))
	if err := conv.flush(); err != nil {
		t.Fatalf("first flush: %v", err)
	}
	if conv.flushed != 2 {
		t.Fatalf("flushed = %d, want 2", conv.flushed)
	}

	// Second flush writes only the new tail; the first two are not re-written.
	conv.Append(textMsg("three"))
	if err := conv.flush(); err != nil {
		t.Fatalf("second flush: %v", err)
	}
	if conv.flushed != 3 {
		t.Fatalf("flushed = %d, want 3", conv.flushed)
	}

	// A no-op flush (nothing pending) must not error or advance.
	if err := conv.flush(); err != nil {
		t.Fatalf("noop flush: %v", err)
	}

	// Resume: a fresh conversation loads the whole thread, once, in order.
	resumed := NewConversation(id)
	if err := resumed.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	got := contents(resumed.Messages())
	want := []string{"one", "two", "three"}
	if len(got) != len(want) {
		t.Fatalf("resumed contents = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resumed[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	// Loaded history is already persisted: flushed must equal the loaded count so
	// the next flush is a no-op, not a duplicate write.
	if resumed.flushed != len(want) {
		t.Fatalf("resumed.flushed = %d, want %d", resumed.flushed, len(want))
	}
	if err := resumed.flush(); err != nil {
		t.Fatalf("post-resume flush: %v", err)
	}

	// Loading again must still see exactly three (no duplication introduced).
	check := NewConversation(id)
	if err := check.load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if n := len(check.Messages()); n != 3 {
		t.Fatalf("reload count = %d, want 3", n)
	}
}

// TestConversation_FlushAtRequestCompletion verifies the wiring Start relies on:
// a conversation accumulates in memory and is persisted by the completion hook,
// with nothing written before it fires.
func TestConversation_FlushAtRequestCompletion(t *testing.T) {
	const id = "test-flush-on-complete"

	ctx, rc := Start(context.Background(), Options{ConversationID: id})
	conv := FromContextConversation(t, ctx)
	conv.Append(textMsg("during the request"))

	// Nothing is persisted until the request completes.
	before := NewConversation(id)
	if err := before.load(); err != nil {
		t.Fatalf("pre-completion load: %v", err)
	}
	if n := len(before.Messages()); n != 0 {
		t.Fatalf("persisted %d messages before completion, want 0", n)
	}

	rc.Done() // main flow finished; no child flows → completes immediately

	after := NewConversation(id)
	if err := after.load(); err != nil {
		t.Fatalf("post-completion load: %v", err)
	}
	if got := contents(after.Messages()); len(got) != 1 || got[0] != "during the request" {
		t.Fatalf("persisted contents = %v, want [during the request]", got)
	}
}

// TestConversation_ImageNotPersisted verifies image payloads never reach storage:
// the bytes are dropped and the message is stored as a text tool result carrying
// the metadata, while the in-memory message keeps its image intact.
func TestConversation_ImageNotPersisted(t *testing.T) {
	const id = "test-image-stripped"

	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	img := MessageToolCallResponse{
		Message:          Message{Type: MessageTypeToolResponse},
		ToolResponseType: ToolResponseTypeImage,
		ID:               "tool_call_1",
		ImageData:        imageBytes,
		ImageMimeType:    "image/png",
	}

	conv := NewConversation(id)
	conv.Append(img)
	if err := conv.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// The live message is untouched — the model still gets the real image.
	live, ok := conv.Messages()[0].(MessageToolCallResponse)
	if !ok {
		t.Fatalf("in-memory message type = %T", conv.Messages()[0])
	}
	if live.ToolResponseType != ToolResponseTypeImage || len(live.ImageData) != len(imageBytes) {
		t.Fatalf("in-memory message was mutated: type=%v bytes=%d",
			live.ToolResponseType, len(live.ImageData))
	}

	// The stored message carries metadata only.
	resumed := NewConversation(id)
	if err := resumed.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	msgs := resumed.Messages()
	if len(msgs) != 1 {
		t.Fatalf("resumed %d messages, want 1", len(msgs))
	}
	got, ok := msgs[0].(MessageToolCallResponse)
	if !ok {
		t.Fatalf("resumed message type = %T", msgs[0])
	}
	if len(got.ImageData) != 0 {
		t.Fatalf("image bytes were persisted (%d bytes)", len(got.ImageData))
	}
	// Downgraded to text so a resumed thread is not rebuilt as an empty image
	// block by the integrations.
	if got.ToolResponseType != ToolResponseTypeText {
		t.Fatalf("resumed ToolResponseType = %v, want text", got.ToolResponseType)
	}
	if got.ID != "tool_call_1" || got.ImageMimeType != "image/png" {
		t.Fatalf("metadata lost: id=%q mime=%q", got.ID, got.ImageMimeType)
	}
	if !strings.Contains(got.Text, "image/png") || !strings.Contains(got.Text, "8 bytes") {
		t.Fatalf("stored text %q lacks image metadata", got.Text)
	}
	content, _, outputType := got.GenerateContent()
	if outputType != ToolCallOutputTypeText || content != got.Text {
		t.Fatalf("resumed message renders as %v/%q, want text/%q", outputType, content, got.Text)
	}
}
