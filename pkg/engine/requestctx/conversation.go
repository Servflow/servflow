package requestctx

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Servflow/servflow/pkg/storage"
)

// conversationStoragePrefix namespaces persisted conversation messages in the
// log store. Kept identical to the historic per-session prefix so threads
// written before centralisation remain loadable.
const conversationStoragePrefix = "agent_conversation_"

// Conversation is the run-scoped message thread owned by a RequestContext.
// Every agent action in a plan reads and appends to the SAME Conversation, so
// the thread is the shared, mid-run-accessible record of everything the run's
// agents have said and done. A Conversation always has an identifier and is
// created once per request by Start (resumed from the log store when the id was
// supplied, otherwise fresh); sub-workflows share it by pointer with the parent.
//
// Appends are in-memory only: the agent hot path never blocks on storage. The
// thread is written to the log store once, at request completion (Flush).
//
// All methods are safe for concurrent use: the per-conversation mutex is what
// stops parallel agent actions in the same request from clobbering each other —
// a batch of messages is appended atomically, never interleaved mid-batch.
type Conversation struct {
	mu       sync.Mutex
	id       string
	messages []ConversationMessage
	// flushed is the number of leading messages already written to the log store:
	// messages[flushed:] is what a Flush still has to persist. Set by load to the
	// size of the resumed history so that history is never written back.
	flushed int
}

// NewConversation builds an empty conversation with the given identifier.
// Appends go to memory; the thread is persisted when Flush is called (Start
// arranges this at completion for the request that owns the thread). Production
// code obtains its conversation from the RequestContext; this constructor is
// exported for hosts that inject a thread and for tests.
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

// Append adds messages to the in-memory thread. It is the single method through
// which agent actions contribute to the shared conversation, and it never
// touches storage — persistence happens once at request completion. The whole
// batch is appended under the lock, so concurrent agents cannot interleave
// within it. A nil/empty call is a no-op.
func (c *Conversation) Append(msgs ...ConversationMessage) {
	if c == nil || len(msgs) == 0 {
		return
	}
	c.mu.Lock()
	c.messages = append(c.messages, msgs...)
	c.mu.Unlock()
}

// flush writes the messages appended since the last flush to the log store and
// advances the high-water mark, so a resumed thread only ever persists what the
// current request added.
func (c *Conversation) flush() error {
	c.mu.Lock()
	if c.id == "" || c.flushed >= len(c.messages) {
		c.mu.Unlock()
		return nil
	}
	pending := c.messages[c.flushed:]
	batch := make([]storage.Serializable, len(pending))
	for i := range pending {
		batch[i] = pending[i]
	}
	high := len(c.messages)
	id := c.id
	c.mu.Unlock()

	if err := storage.WriteToLog(conversationStoragePrefix+id, batch); err != nil {
		return err
	}
	c.mu.Lock()
	c.flushed = high
	c.mu.Unlock()
	return nil
}

// Flush writes the messages added during this request to the log store. Called
// once, at request completion, by the request that owns the conversation.
func (c *Conversation) Flush() error {
	if c == nil {
		return nil
	}
	return c.flush()
}

// load prepends the stored thread for this conversation id, so a resumed run
// starts with the existing history in front of anything it appends. It is called
// by resolveConversation at construction — before any agent can append — so the
// loaded messages are necessarily the leading ones. Unknown message types are
// skipped (requestctx cannot depend on pkg/logging to warn — that would cycle).
func (c *Conversation) load() error {
	entries, err := storage.GetLogEntriesByPrefix(conversationStoragePrefix+c.id, decodeConversationMessage)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range entries {
		if msg, ok := e.(ConversationMessage); ok {
			c.messages = append(c.messages, msg)
		}
	}
	// Resumed messages are already in the store; the sync must only write
	// messages appended after this point, never re-persist the loaded history.
	c.flushed = len(c.messages)
	return nil
}

// decodeConversationMessage deserialises a stored message back into its concrete
// type. Returns (nil, nil) for unrecognised types so a single bad/legacy entry
// does not abort the whole load.
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
		return nil, nil
	}
}

// resolveConversation builds the Conversation for a request from its Options and
// reports whether this request OWNS it (must run the background sync). A child
// request with no explicit ConversationID joins its parent's thread by pointer
// and does NOT own it. Otherwise a fresh thread is created — always with an
// identifier (generated when none was supplied). A supplied id is resumed from
// the store best-effort: a load failure yields an empty thread rather than
// failing request start.
func resolveConversation(opts Options) (conv *Conversation, owned bool) {
	if opts.ConversationID == "" && opts.Parent != nil {
		if pc := opts.Parent.Conversation(); pc != nil {
			return pc, false
		}
	}
	id := opts.ConversationID
	if id == "" {
		id = fmt.Sprintf("conversation_%d", time.Now().UnixNano())
	}
	conv = NewConversation(id)
	if opts.ConversationID != "" {
		_ = conv.load()
	}
	return conv, true
}

// Conversation returns the request's conversation thread. It is never nil for a
// request opened via Start; the zero RequestContext (bare NewRequestContext /
// tests) returns nil, which the Conversation methods tolerate.
func (rc *RequestContext) Conversation() *Conversation {
	rc.Lock()
	defer rc.Unlock()
	return rc.conversation
}

// setConversation installs the thread for this request. Used by Start.
func (rc *RequestContext) setConversation(c *Conversation) {
	rc.Lock()
	defer rc.Unlock()
	rc.conversation = c
}
