package apiconfig

import (
	"fmt"
	"sort"
	"strings"
)

// InvalidReferenceError is a step reference that does not resolve to an existing
// step (bad prefix, or names a step that does not exist).
type InvalidReferenceError struct {
	From   string // canonical id of the source, or an entry label like "http entry"
	To     string // the raw reference that failed
	Reason string
}

func (e *InvalidReferenceError) Error() string {
	return fmt.Sprintf("invalid reference from %s to %q: %s", e.From, e.To, e.Reason)
}

// CycleError is a cycle in the step graph. Path lists the steps forming the
// loop, e.g. action.a -> action.b -> action.a.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle detected: %s", strings.Join(e.Path, " -> "))
}

// UnreachableStepError is a step not reachable from any entry root. It is
// reported as a warning, not a fatal error.
type UnreachableStepError struct {
	ID string
}

func (e *UnreachableStepError) Error() string {
	return fmt.Sprintf("step %s is unreachable from any entry", e.ID)
}

// collectGraphErrors builds the directed step graph and checks it for invalid
// references, cycles, and unreachable steps. extraRoots are additional entry
// points (e.g. a trigger's start step) on top of the http/mcp entries.
//
// Severity: invalid references and cycles reachable from an entry are fatal;
// unreachable steps and cycles confined to an unreachable subgraph are warnings
// (they cannot execute, so they do not block the config).
func (a *APIConfig) collectGraphErrors(ve *ValidationErrors, extraRoots []string) {
	// node set: canonical (prefixed) id -> kind
	nodes := make(map[string]StepKind)
	for id := range a.Actions {
		nodes[ActionConfigPrefix+id] = StepKindAction
	}
	for id := range a.Conditionals {
		nodes[ConditionalConfigPrefix+id] = StepKindConditional
	}
	for id := range a.Responses {
		nodes[ResponsesConfigPrefix+id] = StepKindResponse
	}

	// resolve a reference to a canonical node id. Records an InvalidReferenceError
	// and returns ok=false when the reference is malformed or dangling. Terminal
	// (empty) references return ok=false with no error.
	resolve := func(from, ref string) (string, bool) {
		kind, bare, terminal, err := ParseStepRef(ref)
		if terminal {
			return "", false
		}
		if err != nil {
			ve.Add(&InvalidReferenceError{From: from, To: ref, Reason: err.Error()})
			return "", false
		}
		canonical := CanonicalStepID(ref)
		if _, ok := nodes[canonical]; !ok {
			ve.Add(&InvalidReferenceError{From: from, To: ref, Reason: fmt.Sprintf("no %s named %q", kind, bare)})
			return "", false
		}
		return canonical, true
	}

	// flow edges (next/fail/onTrue/onFalse) used for cycle detection + reachability
	adj := make(map[string][]string)
	var roots []string

	addRoot := func(from, ref string) {
		if c, ok := resolve(from, ref); ok {
			roots = append(roots, c)
		}
	}
	addEdge := func(from, ref string) {
		if c, ok := resolve(from, ref); ok {
			adj[from] = append(adj[from], c)
		}
	}

	// entry roots
	if a.HttpConfig.Next != "" {
		addRoot("http entry", a.HttpConfig.Next)
	}
	if a.IsMCPConfig() && a.McpTool.Start != "" {
		addRoot("mcp entry", a.McpTool.Start)
	}
	for _, r := range extraRoots {
		if r != "" {
			addRoot("trigger entry", r)
		}
	}

	// action edges + dispatch (dispatch is reference-checked and treated as an
	// independent root, but excluded from main-flow cycle detection)
	for id, act := range a.Actions {
		from := ActionConfigPrefix + id
		addEdge(from, act.Next)
		addEdge(from, act.Fail)
		for _, d := range act.Dispatch {
			addRoot(from+" dispatch", d)
		}
	}
	// conditional edges
	for id, cond := range a.Conditionals {
		from := ConditionalConfigPrefix + id
		addEdge(from, cond.OnTrue)
		addEdge(from, cond.OnFalse)
	}

	// deterministic adjacency + roots for stable traversal and error messages
	for k := range adj {
		sort.Strings(adj[k])
	}
	sort.Strings(roots)

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	reported := make(map[string]bool)

	// iterative tri-color DFS from a single source. fatal selects whether a cycle
	// found here is an error or a warning.
	dfs := func(start string, fatal bool) {
		if color[start] != white {
			return
		}
		type frame struct {
			node string
			idx  int
		}
		stack := []frame{{start, 0}}
		path := []string{start}
		color[start] = gray
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.idx < len(adj[top.node]) {
				nb := adj[top.node][top.idx]
				top.idx++
				switch color[nb] {
				case white:
					color[nb] = gray
					stack = append(stack, frame{nb, 0})
					path = append(path, nb)
				case gray:
					// back-edge -> cycle; path[ci:] + nb is the loop
					ci := indexOf(path, nb)
					if ci >= 0 {
						cyc := append(append([]string{}, path[ci:]...), nb)
						key := cycleKey(cyc)
						if !reported[key] {
							reported[key] = true
							if fatal {
								ve.Add(&CycleError{Path: cyc})
							} else {
								ve.AddWarning(&CycleError{Path: cyc})
							}
						}
					}
				}
			} else {
				color[top.node] = black
				stack = stack[:len(stack)-1]
				path = path[:len(path)-1]
			}
		}
	}

	// root-seeded pass: fatal cycles + marks reachable nodes
	for _, r := range roots {
		dfs(r, true)
	}

	// orphans: anything still white is unreachable from any entry (warning)
	var orphans []string
	for id := range nodes {
		if color[id] == white {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	for _, id := range orphans {
		ve.AddWarning(&UnreachableStepError{ID: id})
	}
	// continuation pass over unreachable nodes: cycles here are warnings
	for _, id := range orphans {
		dfs(id, false)
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// cycleKey normalizes a cycle (which starts and ends at the same node) to a
// rotation-independent key so the same loop is only reported once.
func cycleKey(cyc []string) string {
	nodes := cyc
	if len(nodes) > 1 && nodes[0] == nodes[len(nodes)-1] {
		nodes = nodes[:len(nodes)-1]
	}
	if len(nodes) == 0 {
		return ""
	}
	minI := 0
	for i, n := range nodes {
		if n < nodes[minI] {
			minI = i
		}
	}
	rot := append(append([]string{}, nodes[minI:]...), nodes[:minI]...)
	return strings.Join(rot, ">")
}
