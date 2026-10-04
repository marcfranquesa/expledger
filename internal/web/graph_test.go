package web

import (
	"reflect"
	"testing"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

func TestLineage(t *testing.T) {
	records := []experiment.Record{
		{ID: "merge", BasedOn: []string{"left", "right", "right"}},
		{ID: "right", BasedOn: []string{"root"}},
		{ID: "left", BasedOn: []string{"root", "absent"}},
		{ID: "root"}, {ID: "alone"},
		{ID: "cycle-a", BasedOn: []string{"cycle-b"}},
		{ID: "cycle-b", BasedOn: []string{"cycle-a"}},
		{ID: "self", BasedOn: []string{"self"}},
		{ID: "after-cycle", BasedOn: []string{"cycle-a"}},
	}
	build := func(records []experiment.Record) graphView {
		views := make([]experimentView, len(records))
		for i, r := range records {
			views[i] = experimentView{ID: r.ID, Title: r.ID}
		}
		return lineage(records, views)
	}
	g := build(records)
	ranks := map[string]int{}
	byIndex := map[int]string{}
	missing := 0
	for rank, row := range g.Rows {
		for _, n := range row {
			if _, ok := ranks[n.ID]; ok {
				t.Fatalf("duplicate node %s", n.ID)
			}
			ranks[n.ID] = rank
			byIndex[n.Index] = n.ID
			if n.Missing {
				missing++
				if n.ID != "absent" {
					t.Fatal(n)
				}
			}
		}
	}
	if len(ranks) != 10 || missing != 1 {
		t.Fatalf("nodes: %v, missing: %d", ranks, missing)
	}
	for _, pair := range [][2]string{{"root", "left"}, {"root", "right"}, {"left", "merge"}, {"right", "merge"}, {"absent", "left"}, {"cycle-a", "after-cycle"}} {
		if ranks[pair[0]] >= ranks[pair[1]] {
			t.Fatalf("parent not before child: %v", pair)
		}
	}
	if ranks["cycle-a"] != ranks["cycle-b"] {
		t.Fatal("cycle split")
	}
	edges := map[[2]string]bool{}
	for _, e := range g.Edges {
		pair := [2]string{byIndex[e.From], byIndex[e.To]}
		if edges[pair] {
			t.Fatal("duplicate edge")
		}
		edges[pair] = true
	}
	if len(edges) != 9 || !edges[[2]string{"self", "self"}] || !edges[[2]string{"cycle-b", "cycle-a"}] {
		t.Fatalf("edges: %v", edges)
	}
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	if !reflect.DeepEqual(g, build(records)) {
		t.Fatal("layout depends on input order")
	}
}

func TestLineageEmptyAndSingle(t *testing.T) {
	if g := lineage(nil, nil); len(g.Rows) != 0 || len(g.Edges) != 0 {
		t.Fatal(g)
	}
	g := lineage([]experiment.Record{{ID: "only"}}, []experimentView{{ID: "only"}})
	if len(g.Rows) != 1 || len(g.Rows[0]) != 1 || len(g.Edges) != 0 {
		t.Fatal(g)
	}
}

func TestLineageLongestParentPath(t *testing.T) {
	records := []experiment.Record{{ID: "root"}, {ID: "mid", BasedOn: []string{"root"}}, {ID: "leaf", BasedOn: []string{"root", "mid", "root"}}}
	views := []experimentView{{ID: "root"}, {ID: "mid"}, {ID: "leaf", Title: "Leaf title", RemoteURL: "https://example.com/leaf"}}
	g := lineage(records, views)
	if len(g.Rows) != 3 || len(g.Rows[2]) != 1 {
		t.Fatalf("rows: %+v", g.Rows)
	}
	leaf := g.Rows[2][0]
	if leaf.ID != "leaf" || leaf.Title != "Leaf title" || leaf.RemoteURL != "https://example.com/leaf" || !reflect.DeepEqual(leaf.Parents, []string{"mid", "root"}) || len(g.Edges) != 3 {
		t.Fatalf("leaf: %+v, edges: %v", leaf, g.Edges)
	}
}

func TestTerminalDescendantsIncludeSinkCycles(t *testing.T) {
	records := []experiment.Record{
		{ID: "root"},
		{ID: "cycle-a", BasedOn: []string{"root", "cycle-b"}},
		{ID: "cycle-b", BasedOn: []string{"cycle-a"}},
		{ID: "leaf", BasedOn: []string{"cycle-a"}},
		{ID: "sink-a", BasedOn: []string{"root", "sink-b"}},
		{ID: "sink-b", BasedOn: []string{"sink-a"}},
		{ID: "self", BasedOn: []string{"self"}},
		{ID: "alone"},
		{ID: "orphan", BasedOn: []string{"missing"}},
	}
	views := make([]experimentView, len(records))
	for i, record := range records {
		views[i].ID = record.ID
	}
	want := map[string]bool{"leaf": true, "sink-a": true, "sink-b": true, "self": true, "alone": true, "orphan": true}
	for _, row := range lineage(records, views).Rows {
		for _, node := range row {
			if node.Terminal != want[node.ID] {
				t.Errorf("%s: terminal = %v, want %v", node.ID, node.Terminal, want[node.ID])
			}
		}
	}
}
