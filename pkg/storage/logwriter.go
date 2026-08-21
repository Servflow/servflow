package storage

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// logQueueSize is how many appends may be waiting on the writer before an
// append blocks. Blocking at the limit is the intended behaviour: it bounds the
// memory the queue can hold and pushes back on a producer outrunning the disk.
// With batched commits the queue only fills if badger itself has stalled.
const logQueueSize = 1024

// logBatchSize caps how many queued appends one commit takes, so a producer
// that never pauses cannot grow a batch without bound.
const logBatchSize = 256

// logWrite is one queued append, or — when done is non-nil — a barrier: the
// writer closes done once everything queued ahead of it is committed, which is
// what lets a caller wait for its own appends to become readable.
type logWrite struct {
	prefix string
	value  Serializable
	done   chan error
}

// logWriter serialises every append to the log store onto one goroutine.
//
// A single writer is what makes the log's ordering guarantees cheap. Keys are
// assigned from one counter by one goroutine, so no two entries can collide or
// land out of order however many requests are appending at once, and whatever
// piled up while the last commit was in flight is written as a single batch.
type logWriter struct {
	queue chan logWrite
	seq   uint64 // writer-owned: only run() touches it, so it needs no lock
}

var (
	writer     *logWriter
	writerOnce sync.Once
)

// logQueue returns the process's log writer, starting it on first use.
func logQueue() *logWriter {
	writerOnce.Do(func() {
		writer = &logWriter{queue: make(chan logWrite, logQueueSize)}
		go writer.run()
	})
	return writer
}

// writerLogger is where the writer reports failures. A queued append is written
// long after the call that made it returned, so this is the only place those
// errors can surface.
var writerLogger atomic.Pointer[zap.Logger]

// SetLogger installs the logger the background log writer reports failures to.
// Until a host calls this, failures are discarded.
func SetLogger(l *zap.Logger) {
	writerLogger.Store(l)
}

func writerLog() *zap.Logger {
	if l := writerLogger.Load(); l != nil {
		return l
	}
	return zap.NewNop()
}

// AppendToLog queues entries to be written under prefix and returns without
// waiting for the disk.
//
// Entries appear in the log in the order the appends completed, and become
// readable once the writer has committed them — SyncLog waits for exactly that.
// Nothing is reported back here: a failure to serialise or write happens after
// this call has returned, and goes to the logger installed with SetLogger.
func AppendToLog(prefix string, values ...Serializable) {
	if len(values) == 0 {
		return
	}
	if prefix == "" {
		writerLog().Error("dropping log entries appended with no prefix",
			zap.Int("entries", len(values)))
		return
	}
	q := logQueue()
	for _, value := range values {
		q.queue <- logWrite{prefix: prefix, value: value}
	}
}

// SyncLog waits for every append queued before the call to be committed, and
// reports whether they were.
//
// It is for callers that must be certain a reader will see what they wrote —
// one turn of a conversation handing over to the next turn on the same thread.
// Ordinary appends never wait.
func SyncLog() error {
	done := make(chan error, 1)
	logQueue().queue <- logWrite{done: done}
	return <-done
}

// run drains the queue for the life of the process, committing each batch it
// collects. It takes whatever is already queued behind the entry that woke it,
// so a burst of appends costs one commit rather than one per message.
func (w *logWriter) run() {
	for first := range w.queue {
		batch := append(make([]logWrite, 0, logBatchSize), first)
	drain:
		for len(batch) < logBatchSize {
			select {
			case next := <-w.queue:
				batch = append(batch, next)
			default:
				break drain
			}
		}
		w.commit(batch)
	}
}

// commit writes a batch's entries, then releases the barriers it carries. The
// release happens after the write, which is the guarantee SyncLog sells.
func (w *logWriter) commit(batch []logWrite) {
	err := w.write(batch)
	if err != nil {
		writerLog().Error("failed to write queued log entries",
			zap.Int("batch", len(batch)), zap.Error(err))
	}
	for _, entry := range batch {
		if entry.done == nil {
			continue
		}
		if err != nil {
			entry.done <- err
		}
		close(entry.done)
	}
}

// write commits a batch's entries in one badger batch. A batch of nothing but
// barriers writes nothing — a sync on an idle queue must not open the store.
func (w *logWriter) write(batch []logWrite) error {
	pending := make([]logWrite, 0, len(batch))
	for _, entry := range batch {
		if entry.done == nil {
			pending = append(pending, entry)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	_, err := withRetryOnClose(func(db *badger.DB) (struct{}, error) {
		wb := db.NewWriteBatch()
		defer wb.Cancel()
		for _, entry := range pending {
			value, err := entry.value.Serialize()
			if err != nil {
				return struct{}{}, fmt.Errorf("serialise entry for %q: %w", entry.prefix, err)
			}
			if err := wb.Set(w.nextKey(entry.prefix), value); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, wb.Flush()
	})
	return err
}

// nextKey builds the storage key for one entry: the log's prefix, the write
// time, and a counter.
//
// Entries are read back by lexicographic range scan, so a key has to sort the
// way the log was written. The timestamp holds that order across restarts, and
// the counter separates entries written within the same nanosecond — which a
// timestamp alone does not: two appends in one tick produced the same key, and
// the second silently replaced the first. Both parts are fixed width so string
// order and numeric order agree.
func (w *logWriter) nextKey(prefix string) []byte {
	w.seq++
	return []byte(fmt.Sprintf("%s:%s:%019d-%010d",
		servflowPrefix, strings.Trim(prefix, ":"), time.Now().UnixNano(), w.seq))
}
