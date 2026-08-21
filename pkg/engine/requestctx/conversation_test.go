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

// TestConversation_WritesThroughAndResumes verifies the thread's whole
// persistence contract: what is appended is written as it is said, a resume
// loads it once and in order, and a resumed thread that speaks again does not
// write its loaded history back.
func TestConversation_WritesThroughAndResumes(t *testing.T) {
	const id = "test-write-through"

	conv := NewConversation(id)
	conv.Append(textMsg("one"), textMsg("two"))
	if err := conv.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	conv.Append(textMsg("three"))
	if err := conv.Sync(); err != nil {
		t.Fatalf("second sync: %v", err)
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

	// The resumed thread speaks: only the new message is added to the store.
	resumed.Append(textMsg("four"))
	if err := resumed.Sync(); err != nil {
		t.Fatalf("post-resume sync: %v", err)
	}
	check := NewConversation(id)
	if err := check.load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := contents(check.Messages()); len(got) != 4 {
		t.Fatalf("reload contents = %v, want four messages with no duplicated history", got)
	}
}

// TestConversation_ReadableBeforeTheRequestCompletes is what a chat handing one
// turn to the next depends on: a message is durable once it has been said and
// synced, not at the end of the request that said it.
func TestConversation_ReadableBeforeTheRequestCompletes(t *testing.T) {
	const id = "test-readable-mid-request"

	ctx, rc := Start(context.Background(), Options{ConversationID: id})
	conv := FromContextConversation(t, ctx)
	conv.Append(textMsg("during the request"))
	if err := conv.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	during := NewConversation(id)
	if err := during.load(); err != nil {
		t.Fatalf("mid-request load: %v", err)
	}
	if got := contents(during.Messages()); len(got) != 1 || got[0] != "during the request" {
		t.Fatalf("mid-request contents = %v, want [during the request]", got)
	}

	// Completing the request adds nothing: the thread was already written.
	rc.Done()

	after := NewConversation(id)
	if err := after.load(); err != nil {
		t.Fatalf("post-completion load: %v", err)
	}
	if got := contents(after.Messages()); len(got) != 1 {
		t.Fatalf("post-completion contents = %v, want the one message, written once", got)
	}
}

// TestConversation_SubRunKeepsItsOwnThread verifies the isolation a workflow
// tool relies on: a child request speaks into its own thread, named under its
// caller's, and the caller's thread is untouched by it.
func TestConversation_SubRunKeepsItsOwnThread(t *testing.T) {
	const id = "test-parent-thread"

	parentCtx, parent := Start(context.Background(), Options{ConversationID: id})
	FromContextConversation(t, parentCtx).Append(textMsg("caller asked"))

	_, child := Start(parentCtx, Options{ID: "child", Parent: parent})
	childConv := child.Conversation()
	if childConv == nil {
		t.Fatal("child request has no conversation")
	}
	if childConv == parent.Conversation() {
		t.Fatal("child request shares its caller's thread")
	}
	if !strings.HasPrefix(childConv.ID(), id+"/") {
		t.Fatalf("child thread id = %q, want it named under %q", childConv.ID(), id)
	}
	if !strings.HasPrefix(child.ID(), parent.ID()+"/") {
		t.Fatalf("child request id = %q, want it named under %q", child.ID(), parent.ID())
	}

	childConv.Append(textMsg("sub-run worked"))
	if err := childConv.Sync(); err != nil {
		t.Fatalf("child sync: %v", err)
	}
	child.Done()
	parent.Done()

	// The caller's thread holds only what the caller said. What the sub-run said
	// reaches the caller as the result of the call, not as a share of its thread.
	after := NewConversation(id)
	if err := after.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := contents(after.Messages()); len(got) != 1 || got[0] != "caller asked" {
		t.Fatalf("caller's thread = %v, want [caller asked]", got)
	}

	// The child's own thread is in the store, under the caller's id.
	sub := NewConversation(childConv.ID())
	if err := sub.load(); err != nil {
		t.Fatalf("sub load: %v", err)
	}
	if got := contents(sub.Messages()); len(got) != 1 || got[0] != "sub-run worked" {
		t.Fatalf("sub-run thread = %v, want [sub-run worked]", got)
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
	if err := conv.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
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

// TestBindConversation_PutsTheRequestOnAnotherThread verifies what an entry
// handler binding to a sender-named thread relies on: the bound thread's stored
// history is loaded, and what the request goes on to say lands on that thread
// rather than the one Start created.
func TestBindConversation_PutsTheRequestOnAnotherThread(t *testing.T) {
	const bound = "test-bind-target"

	// A thread with a past, as a previous request would have left it.
	previous := NewConversation(bound)
	previous.Append(textMsg("said before"))
	if err := previous.Sync(); err != nil {
		t.Fatalf("seeding the thread: %v", err)
	}

	// A request that starts with no id of its own, as a delivery carrying no
	// conversation header does.
	ctx, rc := Start(context.Background(), Options{})
	started := FromContextConversation(t, ctx)

	if err := rc.BindConversation(bound); err != nil {
		t.Fatalf("bind: %v", err)
	}

	conv := rc.Conversation()
	if conv.ID() != bound {
		t.Fatalf("conversation id = %q, want %q", conv.ID(), bound)
	}
	if got := contents(conv.Messages()); len(got) != 1 || got[0] != "said before" {
		t.Fatalf("bound thread contents = %v, want [said before]", got)
	}

	conv.Append(textMsg("said now"))
	if err := conv.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	rc.Done()

	after := NewConversation(bound)
	if err := after.load(); err != nil {
		t.Fatalf("post-completion load: %v", err)
	}
	if got := contents(after.Messages()); len(got) != 2 || got[1] != "said now" {
		t.Fatalf("persisted contents = %v, want [said before said now]", got)
	}

	// The thread the request started on was never appended to, so it left
	// nothing behind.
	orphan := NewConversation(started.ID())
	if err := orphan.load(); err != nil {
		t.Fatalf("orphan load: %v", err)
	}
	if n := len(orphan.Messages()); n != 0 {
		t.Fatalf("the dropped thread persisted %d messages, want 0", n)
	}
}

// TestBindConversation_RefusesAfterAnAppend verifies the guard that keeps
// binding from discarding messages: they live only in memory until completion.
func TestBindConversation_RefusesAfterAnAppend(t *testing.T) {
	ctx, rc := Start(context.Background(), Options{})
	FromContextConversation(t, ctx).Append(textMsg("already said"))

	if err := rc.BindConversation("test-bind-too-late"); err == nil {
		t.Fatal("bind after an append succeeded, want an error")
	}
	if err := rc.BindConversation(""); err == nil {
		t.Fatal("bind to an empty id succeeded, want an error")
	}
}
