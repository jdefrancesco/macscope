package process

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Node uses ps lstart as a second-resolution process identity, not a timestamp
// with an invented timezone. Path is the final comm field, preserving spaces.
type Node struct {
	PID       int    `json:"pid"`
	PPID      int    `json:"ppid"`
	StartedAt string `json:"started_at"`
	Path      string `json:"path"`
	Name      string `json:"name"`
}

type Tree struct {
	Source     string    `json:"source"`
	Status     string    `json:"status"`
	CapturedAt time.Time `json:"captured_at"`
	Target     Node      `json:"target"`
	Ancestors  []Node    `json:"ancestors"`
	Children   []Node    `json:"children"`
	Warnings   []string  `json:"warnings,omitempty"`
}

func SnapshotTree(ctx context.Context, pid int, runner CommandRunner) (Tree, error) {
	result, err := runner.Run(ctx, "/bin/ps", "-axww", "-o", "pid=", "-o", "ppid=", "-o", "lstart=", "-o", "comm=")
	if err != nil {
		return Tree{}, fmt.Errorf("process ancestry: %w: %s", err, strings.TrimSpace(result.Stderr))
	}
	nodes, err := ParseSnapshot(result.Stdout)
	if err != nil {
		return Tree{}, err
	}
	tree, err := BuildTree(nodes, pid)
	tree.Source = "/bin/ps -axww -o pid= -o ppid= -o lstart= -o comm="
	tree.CapturedAt = time.Now().UTC()
	if detail := strings.TrimSpace(result.Stderr); detail != "" {
		tree.Warnings = append(tree.Warnings, detail)
		tree.Status = "partial"
	}
	return tree, err
}

func Identity(ctx context.Context, pid int, runner CommandRunner) (Node, error) {
	if pid <= 0 {
		return Node{}, fmt.Errorf("invalid PID: %d", pid)
	}
	result, err := runner.Run(ctx, "/bin/ps", "-ww", "-p", strconv.Itoa(pid), "-o", "pid=", "-o", "ppid=", "-o", "lstart=", "-o", "comm=")
	if err != nil {
		return Node{}, fmt.Errorf("process identity: %w: %s", err, strings.TrimSpace(result.Stderr))
	}
	nodes, err := ParseSnapshot(result.Stdout)
	if err != nil {
		return Node{}, err
	}
	if len(nodes) != 1 || nodes[0].PID != pid {
		return Node{}, fmt.Errorf("process identity unavailable for PID %d; the process may have exited", pid)
	}
	return nodes[0], nil
}

func ParseSnapshot(data string) ([]Node, error) {
	nodes := make([]Node, 0)
	seen := make(map[int]bool)
	for _, line := range strings.Split(data, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields, path := splitPrefix(line, 7)
		if len(fields) != 7 || path == "" {
			return nil, fmt.Errorf("unexpected ps ancestry row: %q", line)
		}
		pid, pidErr := strconv.Atoi(fields[0])
		ppid, err := strconv.Atoi(fields[1])
		if pidErr != nil || pid < 0 || err != nil || ppid < 0 || seen[pid] {
			return nil, fmt.Errorf("invalid or duplicate PID in ps ancestry row: %q", line)
		}
		started := strings.Join(fields[2:7], " ")
		if _, err := time.Parse("Mon Jan 2 15:04:05 2006", started); err != nil {
			return nil, fmt.Errorf("invalid ps start time %q", started)
		}
		nodes = append(nodes, Node{PID: pid, PPID: ppid, StartedAt: started, Path: path, Name: filepath.Base(path)})
		seen[pid] = true
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("ps returned no process identity rows")
	}
	return nodes, nil
}

func splitPrefix(line string, count int) ([]string, string) {
	fields := make([]string, 0, count)
	for len(fields) < count {
		line = strings.TrimLeft(line, " \t")
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			return fields, ""
		}
		fields = append(fields, line[:i])
		line = line[i:]
	}
	return fields, strings.TrimLeft(line, " \t")
}

func BuildTree(nodes []Node, pid int) (Tree, error) {
	byPID := make(map[int]Node, len(nodes))
	for _, node := range nodes {
		byPID[node.PID] = node
	}
	target, ok := byPID[pid]
	if !ok {
		return Tree{}, fmt.Errorf("PID %d absent from process snapshot; it may have exited", pid)
	}
	tree := Tree{Status: "ok", Target: target, Ancestors: []Node{}, Children: []Node{}}
	seen := map[int]bool{pid: true}
	parent := target.PPID
	for parent != 0 {
		if seen[parent] || len(tree.Ancestors) >= 64 {
			tree.Warnings = append(tree.Warnings, "ancestry cycle or depth limit reached")
			break
		}
		node, ok := byPID[parent]
		if !ok {
			tree.Warnings = append(tree.Warnings, fmt.Sprintf("parent PID %d was not visible in this snapshot", parent))
			break
		}
		seen[parent] = true
		tree.Ancestors = append(tree.Ancestors, node)
		parent = node.PPID
	}
	for _, node := range nodes {
		if node.PPID == pid && node.PID != pid {
			tree.Children = append(tree.Children, node)
		}
	}
	sort.Slice(tree.Children, func(i, j int) bool { return tree.Children[i].PID < tree.Children[j].PID })
	if len(tree.Warnings) > 0 {
		tree.Status = "partial"
	}
	return tree, nil
}
