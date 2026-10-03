package web

import (
	"sort"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

type graphNode struct {
	experimentView
	Index   int
	Missing bool
	Parents []string
}

type graphEdge struct{ From, To int }
type graphView struct {
	Columns [][]graphNode
	Edges   []graphEdge
}

// Condense strongly connected components before assigning dependency columns.
// Cycles stay together; columns express lineage, never time.
func lineage(records []experiment.Record, views []experimentView) graphView {
	nodes := map[string]graphNode{}
	for i, r := range records {
		nodes[r.ID] = graphNode{experimentView: views[i]}
	}
	for _, r := range records {
		for _, p := range r.BasedOn {
			if _, ok := nodes[p]; !ok {
				nodes[p] = graphNode{experimentView: experimentView{ID: p, Title: "Missing experiment"}, Missing: true}
			}
		}
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	indexes := map[string]int{}
	for i, id := range ids {
		indexes[id] = i
		n := nodes[id]
		n.Index = i
		nodes[id] = n
	}
	children := make([][]int, len(ids))
	var graph graphView
	for _, r := range records {
		parents := append([]string(nil), r.BasedOn...)
		sort.Strings(parents)
		unique := []string{}
		for _, p := range parents {
			if len(unique) > 0 && unique[len(unique)-1] == p {
				continue
			}
			unique = append(unique, p)
			children[indexes[p]] = append(children[indexes[p]], indexes[r.ID])
		}
		n := nodes[r.ID]
		n.Parents = unique
		nodes[r.ID] = n
	}
	for from := range children {
		sort.Ints(children[from])
		for _, to := range children[from] {
			graph.Edges = append(graph.Edges, graphEdge{from, to})
		}
	}
	next := 0
	stack := []int{}
	number := make([]int, len(ids))
	low := make([]int, len(ids))
	active := make([]bool, len(ids))
	component := make([]int, len(ids))
	count := 0
	var visit func(int)
	visit = func(v int) {
		next++
		number[v] = next
		low[v] = next
		stack = append(stack, v)
		active[v] = true
		for _, w := range children[v] {
			if number[w] == 0 {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if active[w] {
				low[v] = min(low[v], number[w])
			}
		}
		if low[v] == number[v] {
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				active[w] = false
				component[w] = count
				if w == v {
					break
				}
			}
			count++
		}
	}
	for v := range ids {
		if number[v] == 0 {
			visit(v)
		}
	}
	// Tarjan emits child components first, so reverse order is topological.
	ranks := make([]int, count)
	members := make([][]int, count)
	for v, c := range component {
		members[c] = append(members[c], v)
	}
	for c := count - 1; c >= 0; c-- {
		for _, v := range members[c] {
			for _, w := range children[v] {
				if component[w] != c {
					ranks[component[w]] = max(ranks[component[w]], ranks[c]+1)
				}
			}
		}
	}
	for v, id := range ids {
		rank := ranks[component[v]]
		for len(graph.Columns) <= rank {
			graph.Columns = append(graph.Columns, nil)
		}
		graph.Columns[rank] = append(graph.Columns[rank], nodes[id])
	}
	return graph
}
