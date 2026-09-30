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

// Graph keeps track of a directed acyclic graph. Edges are reference-counted: an
// edge added N times remains in the graph until it has been removed N times.
type Graph struct {
	nodes map[string]map[string]int
	mu    sync.Mutex
}

// AddEdge adds a new edge to this graph (or adds a reference to it, if it is
// already present). Returns an error if the new edge would introduce a cycle in
// the graph.
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
		g.nodes = make(map[string]map[string]int)
	}

	edges := g.nodes[from]
	if edges == nil {
		edges = make(map[string]int)
		g.nodes[from] = edges
	}

	edges[to]++
	return nil
}

// RemoveEdge removes a reference to an edge from this graph, and removes the
// edge once no reference to it remains.
func (g *Graph) RemoveEdge(from string, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	edges := g.nodes[from]
	if edges[to] > 1 {
		edges[to]--
		return
	}

	delete(edges, to)
	if len(edges) == 0 {
		delete(g.nodes, from)
	}
}

func (g *Graph) path(from string, to string) []string {
	var path []string
	for child := range g.nodes[from] {
		if child == to {
			return []string{from, to}
		}
		// Keep the shortest path found so far; children with no path to `to` must not discard it.
		if childPath := g.path(child, to); childPath != nil && (path == nil || len(childPath) < len(path)) {
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
