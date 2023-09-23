package graphx

import (
	"fmt"

	"github.com/dominikbraun/graph"
	"golang.org/x/exp/maps"
)

func DFS[K comparable, T any](g graph.Graph[K, T], start K, visit func(K, []K) bool) error {
	adjacencyMap, err := g.AdjacencyMap()
	if err != nil {
		return fmt.Errorf("could not get adjacency map: %w", err)
	}

	if _, ok := adjacencyMap[start]; !ok {
		return fmt.Errorf("could not find start vertex with hash %v", start)
	}

	dfs(g, nil, adjacencyMap, start, visit)

	return nil
}

func dfs[K comparable, T any](g graph.Graph[K, T], ancestors []K, m map[K]map[K]graph.Edge[K], n K, visit func(K, []K) bool) bool {
	// Stop traversing the graph if the visit function returns true.
	if stop := visit(n, ancestors); stop {
		return stop
	}
	na := make([]K, 0, len(ancestors))
	na = append(na, ancestors...)
	na = append(na, n)

	children := maps.Keys(m[n])

	for _, c := range children {
		if stop := dfs(g, na, m, c, visit); stop {
			return stop
		}
	}

	return false
}
