package requestctx

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Servflow/servflow/pkg/storage"
)

// conversationStoragePrefix namespaces persisted conversation messages in the
// log store; the conversation id follows it to key one thread.
const conversationStoragePrefix = "agent_conversation_"

// conversationHistoryWindow is how much of a thread a resumed request starts
// with: its most recent messages, oldest first. It is a window rather than the
// whole thread because a thread is fed to the model on every turn — a chat
// that has run for months would otherwise put its entire past into each
// prompt — and because loading a log that only grows is unbounded work.
const conversationHistoryWindow = 200

// Conversation is the message thread of one request. Every agent action in a
// plan reads and appends to the SAME Conversation, so the thread is the
// mid-run-accessible record of everything that request's agents have said and
// done. It always has an identifier and is created once per request by Start —
// resumed from the log store when the id was supplied, otherwise fresh.
//
// A thread belongs to one request and is never handed to another: a
// sub-workflow gets its own, named under its caller's (see
// childConversationID), and returns a result rather than a share of the
// caller's thread. Two requests resuming the same id each work from their own
// copy and their appends merge in the log, which is append-only.
//
// Appends are written through to the log store in the background: the agent hot
// path neither blocks on the disk nor waits for the end of the request to
// become durable. Sync waits for what has been appended so far to be readable.
//
// All methods are safe for concurrent use: the per-conversation mutex is what
// stops parallel agent actions in the same request from clobbering each other —
// a batch of messages is appended atomically, never interleaved mid-batch, and
// it reaches the log in the order it reaches the thread.
type Conversation struct {
	mu       sync.Mutex
	id       string
	messages []ConversationMessage
	// appendedHere records whether this request has contributed to the thread, as
	// opposed to merely loading it. It is what BindConversation refuses on: those
	// messages are already in the store under this id.
	appendedHere bool
}

// NewConversation builds an empty conversation with the given identifier.
// Appends are written through to the log store under that identifier.
// Production code obtains its conversation from the RequestContext; this
// constructor is exported for hosts that inject a thread and for tests.
func NewConversation(id string) *Conversation {
	return &Conversation{
		id:       id,
		messages: make([]ConversationMessage, 0),
	}
}

// ID returns the conversation identifier.
func (c *Conversation) ID() string {
	return c.id
}

// Messages returns a snapshot copy of the thread so far. Callers (e.g. an agent
// session seeding its working context) may retain and mutate the slice without
// affecting the Conversation.
func (c *Conversation) Messages() []ConversationMessage {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ConversationMessage, len(c.messages))
	copy(out, c.messages)
	return out
}

// Append adds messages to the thread and queues them for the log store. It is
// the single method through which agent actions contribute to the conversation.
// It does not wait for the disk — what it queues is written by the log writer,
// which reports its own failures.
//
// The whole batch is appended under the lock, so concurrent agents cannot
// interleave within it, and the queueing happens under the same lock so the
// thread's order and the log's order are the same order. A nil/empty call is a
// no-op.
func (c *Conversation) Append(msgs ...ConversationMessage) {
	if c == nil || len(msgs) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, msgs...)
	c.appendedHere = true
	if c.id == "" {
		return
	}
	entries := make([]storage.Serializable, len(msgs))
	for i := range msgs {
		entries[i] = msgs[i]
	}
	storage.AppendToLog(conversationStoragePrefix+c.id, entries...)
}

// appended reports whether this request has contributed to the thread rather
// than only loading it. It is what makes rebinding to another thread safe to
// refuse: what has been appended is already in the store under the current id,
// and moving the request now would file the rest of the turn elsewhere.
func (c *Conversation) appended() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.appendedHere
}

// Sync waits for everything appended to this thread so far to be readable, and
// reports whether it is.
//
// Appends are written in the background, so a caller that is about to let
// something else read the thread — the next turn of a chat this one has been
// holding a lock against — waits here first. Nothing else needs to: a thread is
// durable shortly after it is spoken, with or without this.
func (c *Conversation) Sync() error {
	if c == nil {
		return nil
	}
	return storage.SyncLog()
}

// load prepends the stored thread for this conversation id — its most recent
// conversationHistoryWindow messages — so a resumed run starts with the
// existing history in front of anything it appends. It is called before any
// agent can append, so the loaded messages are necessarily the leading ones.
func (c *Conversation) load() error {
	entries, err := storage.GetLogEntriesByPrefix(
		conversationStoragePrefix+c.id, conversationHistoryWindow, decodeConversationMessage)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range entries {
		msg, ok := e.(ConversationMessage)
		if !ok {
			return fmt.Errorf("conversation %s: stored entry is a %T, not a message", c.id, e)
		}
		c.messages = append(c.messages, msg)
	}
	// Loaded messages come from the store and go back into the thread directly,
	// never through Append — re-queueing them would write the history a second
	// time on every resume.
	return nil
}

// decodeConversationMessage deserialises a stored message back into its concrete
// type. A type it does not recognise is a corrupt entry and fails the read:
// resuming a thread with a message missing from the middle of it would hand the
// model a conversation that never happened. resolveConversation treats a failed
// load as an empty thread, so the request still starts.
func decodeConversationMessage(data []byte) (any, error) {
	var header Message
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case MessageTypeText:
		var m MessageTypeContent
		return m, json.Unmarshal(data, &m)
	case MessageTypeToolResponse:
		var m MessageToolCallResponse
		return m, json.Unmarshal(data, &m)
	case MessageTypeToolCall:
		var m MessageToolCall
		return m, json.Unmarshal(data, &m)
	default:
		return nil, fmt.Errorf("unknown conversation message type %q", header.Type)
	}
}

// resolveConversation builds the Conversation for a request from its Options.
// Every request gets its own thread, always with an identifier:
//
//   - a supplied id resumes that thread from the store, best-effort — a load
//     failure yields an empty thread rather than failing request start;
//   - a child request gets a fresh thread named under its caller's, so a
//     sub-workflow speaks into its own record and hands back only its result;
//   - anything else gets a fresh thread under a generated id.
func resolveConversation(opts Options) *Conversation {
	if opts.ConversationID != "" {
		conv := NewConversation(opts.ConversationID)
		_ = conv.load()
		return conv
	}
	if opts.Parent != nil {
		if parent := opts.Parent.Conversation(); parent != nil {
			return NewConversation(childConversationID(parent.ID()))
		}
	}
	return NewConversation(fmt.Sprintf("conversation_%d", time.Now().UnixNano()))
}

// childSuffix numbers the threads created under a parent. It is a counter
// rather than a timestamp because a plan can start two sub-workflows in
// parallel, and two threads created in the same nanosecond must not share a
// name.
var childSuffix atomic.Uint64

// childConversationID names a child request's thread under its parent's.
//
// The lineage is in the id so a sub-workflow's thread is never loose in the
// store: everything said under a chat, however deep the workflow nesting, is
// reachable from that chat's id by prefix. The separator is deliberately not
// the ":" the log store builds keys with, so loading a parent cannot pick up
// its children's messages.
func childConversationID(parentID string) string {
	return fmt.Sprintf("%s/sub_%d", parentID, childSuffix.Add(1))
}

// Conversation returns the request's conversation thread. It is never nil for a
// request opened via Start; the zero RequestContext (bare NewRequestContext /
// tests) returns nil, which the Conversation methods tolerate.
func (rc *RequestContext) Conversation() *Conversation {
	rc.Lock()
	defer rc.Unlock()
	return rc.conversation
}

// setConversation installs the thread for this request. Used by Start and
// BindConversation.
func (rc *RequestContext) setConversation(c *Conversation) {
	rc.Lock()
	defer rc.Unlock()
	rc.conversation = c
}

// BindConversation puts this request on the thread named by id, loaded from the
// store.
//
// It exists for senders that carry their own notion of a thread and no way to
// state it as a request header: a Telegram chat is a conversation, but a
// delivery from Telegram says so only in its body, and only once the delivery
// has been authenticated. So an entry handler binds after it knows what it is
// talking to, and the thread the request started on — created by Start, empty,
// never spoken into — is dropped without a trace.
//
// It must be called before anything appends to the current thread: what has
// been appended is already in the store under the thread's own id, so binding
// now would leave one turn split across two threads. That is an error, not a
// silent misfiling.
func (rc *RequestContext) BindConversation(id string) error {
	if id == "" {
		return errors.New("conversation id cannot be empty")
	}
	current := rc.Conversation()
	if current.appended() {
		return fmt.Errorf("cannot bind conversation %q: this request has already appended to %q",
			id, current.ID())
	}
	if current != nil && current.ID() == id {
		return nil
	}

	conv := NewConversation(id)
	if err := conv.load(); err != nil {
		return fmt.Errorf("loading conversation %q: %w", id, err)
	}
	rc.setConversation(conv)
	return nil
}
