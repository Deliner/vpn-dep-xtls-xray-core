package dag

import (
	"fmt"
	"slices"
	"strings"
)

type Graph map[string]map[string]bool

func (g Graph) Add(from, to string) {
	if g[from] == nil {
		g[from] = map[string]bool{}
	}
	if to != "" {
		g[from][to] = true
		if g[to] == nil {
			g[to] = map[string]bool{}
		}
	}
}

type walk struct {
	graph  Graph
	done   map[string]bool
	active map[string]int
	stack  []string
	order  []string
}

func Keys[V any](values map[string]V) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}

func (g Graph) Order() ([]string, error) {
	w := walk{graph: g, done: map[string]bool{}, active: map[string]int{}}
	for _, node := range Keys(g) {
		if err := w.visit(node); err != nil {
			return nil, err
		}
	}
	return w.order, nil
}

func (w *walk) visit(node string) error {
	if index, ok := w.active[node]; ok {
		cycle := append(slices.Clone(w.stack[index:]), node)
		return fmt.Errorf("dependency cycle: %s", strings.Join(cycle, " -> "))
	}
	if w.done[node] {
		return nil
	}
	w.active[node] = len(w.stack)
	w.stack = append(w.stack, node)
	for _, next := range Keys(w.graph[node]) {
		if err := w.visit(next); err != nil {
			return err
		}
	}
	delete(w.active, node)
	w.stack = w.stack[:len(w.stack)-1]
	w.done[node] = true
	w.order = append(w.order, node)
	return nil
}
