// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common

import (
	"fmt"
	"strings"
	"sync"
)

// Graph keeps track of a directed acyclic graph.
type Graph struct {
	nodes map[string]map[string]struct{}
	mu    sync.Mutex
}

// AddEdge adds a new edge to this graph. Returns an error if the new edge would
// introduce a cycle in the graph.
func (g *Graph) AddEdge(from string, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if from == to {
		return fmt.Errorf("cycle detected: %s -> %s", from, to)
	}

	if path := g.path(to, from); len(path) > 0 {
		return fmt.Errorf("cycle detected: %s -> %s", strings.Join(path, " -> "), to)
	}

	if g.nodes == nil {
		g.nodes = make(map[string]map[string]struct{})
	}

	edges := g.nodes[from]
	if edges == nil {
		edges = make(map[string]struct{})
		g.nodes[from] = edges
	}

	edges[to] = struct{}{}
	return nil
}

// RemoveEdge removes an edge from this graph.
func (g *Graph) RemoveEdge(from string, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.nodes[from], to)
	if len(g.nodes[from]) == 0 {
		delete(g.nodes, from)
	}
}

func (g *Graph) path(from string, to string) []string {
	var path []string
	for child := range g.nodes[from] {
		if child == to {
			return []string{from, to}
		}
		if childPath := g.path(child, to); path == nil || len(childPath) < len(path) {
			path = childPath
		}
	}

	if path != nil {
		result := make([]string, 0, len(path)+1)
		result = append(result, from)
		result = append(result, path...)
		return result
	}
	return nil
}
