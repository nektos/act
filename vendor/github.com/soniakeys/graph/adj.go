// Copyright 2014 Sonia Keys
// License MIT: https://opensource.org/licenses/MIT

package graph

// adj.go contains methods on AdjacencyList and LabeledAdjacencyList.
//
// AdjacencyList methods are placed first and are alphabetized.
// LabeledAdjacencyList methods follow, also alphabetized.
// Only exported methods need be alphabetized; non-exported methods can
// be left near their use.

import (
	"sort"

	"github.com/soniakeys/bits"
)

// NI is a "node int"
//
// It is a node number or node ID.  NIs are used extensively as slice indexes.
// NIs typically account for a significant fraction of the memory footprint of
// a graph.
type NI int32

// AnyParallel identifies if a graph contains parallel arcs, multiple arcs
// that lead from a node to the same node.
//
// If the graph has parallel arcs, the results fr and to represent an example
// where there are parallel arcs from node `fr` to node `to`.
//
// If there are no parallel arcs, the method returns false -1 -1.
//
// Multiple loops on a node count as parallel arcs.
//
// See also alt.AnyParallelMap, which can perform better for some large
// or dense graphs.
func (g AdjacencyList) AnyParallel() (has bool, fr, to NI) {
	var t []NI
	for n, to := range g {
		if len(to) == 0 {
			continue
		}
		// different code in the labeled version, so no code gen.
		t = append(t[:0], to...)
		sort.Slice(t, func(i, j int) bool { return t[i] < t[j] })
		t0 := t[0]
		for _, to := range t[1:] {
			if to == t0 {
				return true, NI(n), t0
			}
			t0 = to
		}
	}
	return false, -1, -1
}

// Complement returns the arc-complement of a simple graph.
//
// The result will have an arc for every pair of distinct nodes where there
// is not an arc in g.  The complement is valid for both directed and
// undirected graphs.  If g is undirected, the complement will be undirected.
// The result will always be a simple graph, having no loops or parallel arcs.
func (g AdjacencyList) Complement() AdjacencyList {
	c := make(AdjacencyList, len(g))
	b := bits.New(len(g))
	for n, to := range g {
		b.ClearAll()
		for _, to := range to {
			b.SetBit(int(to), 1)
		}
		b.SetBit(n, 1)
		ct := make([]NI, len(g)-b.OnesCount())
		i := 0
		b.IterateZeros(func(to int) bool {
			ct[i] = NI(to)
			i++
			return true
		})
		c[n] = ct
	}
	return c
}

// IsUndirected returns true if g represents an undirected graph.
//
// Returns true when all non-loop arcs are paired in reciprocal pairs.
// Otherwise returns false and an example unpaired arc.
func (g AdjacencyList) IsUndirected() (u bool, from, to NI) {
	// similar code in dot/writeUndirected
	unpaired := make(AdjacencyList, len(g))
	for fr, to := range g {
	arc: // for each arc in g
		for _, to := range to {
			if to == NI(fr) {
				continue // loop
			}
			// search unpaired arcs
			ut := unpaired[to]
			for i, u := range ut {
				if u == NI(fr) { // found reciprocal
					last := len(ut) - 1
					ut[i] = ut[last]
					unpaired[to] = ut[:last]
					continue arc
				}
			}
			// reciprocal not found
			unpaired[fr] = append(unpaired[fr], to)
		}
	}
	for fr, to := range unpaired {
		if len(to) > 0 {
			return false, NI(fr), to[0]
		}
	}
	return true, -1, -1
}

// SortArcLists sorts the arc lists of each node of receiver g.
//
// Nodes are not relabeled and the graph remains equivalent.
func (g AdjacencyList) SortArcLists() {
	for _, to := range g {
		sort.Slice(to, func(i, j int) bool { return to[i] < to[j] })
	}
}

// ------- Labeled methods below -------

// ArcsAsEdges constructs an edge list with an edge for each arc, including
// reciprocals.
//
// This is a simple way to construct an edge list for algorithms that allow
// the duplication represented by the reciprocal arcs.  (e.g. Kruskal)
//
// See also LabeledUndirected.Edges for the edge list without this duplication.
func (g LabeledAdjacencyList) ArcsAsEdges() (el []LabeledEdge) {
	for fr, to := range g {
		for _, to := range to {
			el = append(el, LabeledEdge{Edge{NI(fr), to.To}, to.Label})
		}
	}
	return
}

// DistanceMatrix constructs a distance matrix corresponding to the arcs
// of graph g and weight function w.
//
// An arc from f to t with WeightFunc return w is represented by d[f][t] == w.
// In case of parallel arcs, the lowest weight is stored.  The distance from
// any node to itself d[n][n] is 0, unless the node has a loop with a negative
// weight.  If g has no arc from f to distinct t, +Inf is stored for d[f][t].
//
// The returned DistanceMatrix is suitable for DistanceMatrix.FloydWarshall.
func (g LabeledAdjacencyList) DistanceMatrix(w WeightFunc) (d DistanceMatrix) {
	d = newDM(len(g))
	for fr, to := range g {
		for _, to := range to {
			// < to pick min of parallel arcs (also nicely ignores NaN)
			if wt := w(to.Label); wt < d[fr][to.To] {
				d[fr][to.To] = wt
			}
		}
	}
	return
}

// HasArcLabel returns true if g has any arc from node `fr` to node `to`
// with label `l`.
//
// Also returned is the index within the slice of arcs from node `fr`.
// If no arc from `fr` to `to` with label `l` is present, HasArcLabel returns
// false, -1.
func (g LabeledAdjacencyList) HasArcLabel(fr, to NI, l LI) (bool, int) {
	t := Half{to, l}
	for x, h := range g[fr] {
		if h == t {
			return true, x
		}
	}
	return false, -1
}

// AnyParallel identifies if a graph contains parallel arcs, multiple arcs
// that lead from a node to the same node.
//
// If the graph has parallel arcs, the results fr and to represent an example
// where there are parallel arcs from node `fr` to node `to`.
//
// If there are no parallel arcs, the method returns -1 -1.
//
// Multiple loops on a node count as parallel arcs.
//
// See also alt.AnyParallelMap, which can perform better for some large
// or dense graphs.
func (g LabeledAdjacencyList) AnyParallel() (has bool, fr, to NI) {
	var t []NI
	for n, to := range g {
		if len(to) == 0 {
			continue
		}
		// slightly different code needed here compared to AdjacencyList
		t = t[:0]
		for _, to := range to {
			t = append(t, to.To)
		}
		sort.Slice(t, func(i, j int) bool { return t[i] < t[j] })
		t0 := t[0]
		for _, to := range t[1:] {
			if to == t0 {
				return true, NI(n), t0
			}
			t0 = to
		}
	}
	return false, -1, -1
}

// AnyParallelLabel identifies if a graph contains parallel arcs with the same
// label.
//
// If the graph has parallel arcs with the same label, the results fr and to
// represent an example where there are parallel arcs from node `fr`
// to node `to`.
//
// If there are no parallel arcs, the method returns false -1 Half{}.
//
// Multiple loops on a node count as parallel arcs.
func (g LabeledAdjacencyList) AnyParallelLabel() (has bool, fr NI, to Half) {
	var t []Half
	for n, to := range g {
		if len(to) == 0 {
			continue
		}
		// slightly different code needed here compared to AdjacencyList
		t = t[:0]
		for _, to := range to {
			t = append(t, to)
		}
		sort.Slice(t, func(i, j int) bool {
			return t[i].To < t[j].To ||
				t[i].To == t[j].To && t[i].Label < t[j].Label
		})
		t0 := t[0]
		for _, to := range t[1:] {
			if to == t0 {
				return true, NI(n), t0
			}
			t0 = to
		}
	}
	return false, -1, Half{}
}

// IsUndirected returns true if g represents an undirected graph.
//
// Returns true when all non-loop arcs are paired in reciprocal pairs with
// matching labels.  Otherwise returns false and an example unpaired arc.
//
// Note the requirement that reciprocal pairs have matching labels is
// an additional test not present in the otherwise equivalent unlabeled version
// of IsUndirected.
func (g LabeledAdjacencyList) IsUndirected() (u bool, from NI, to Half) {
	// similar code in LabeledAdjacencyList.Edges
	unpaired := make(LabeledAdjacencyList, len(g))
	for fr, to := range g {
	arc: // for each arc in g
		for _, to := range to {
			if to.To == NI(fr) {
				continue // loop
			}
			// search unpaired arcs
			ut := unpaired[to.To]
			for i, u := range ut {
				if u.To == NI(fr) && u.Label == to.Label { // found reciprocal
					last := len(ut) - 1
					ut[i] = ut[last]
					unpaired[to.To] = ut[:last]
					continue arc
				}
			}
			// reciprocal not found
			unpaired[fr] = append(unpaired[fr], to)
		}
	}
	for fr, to := range unpaired {
		if len(to) > 0 {
			return false, NI(fr), to[0]
		}
	}
	return true, -1, to
}

// ArcLabels constructs the multiset of LIs present in g.
func (g LabeledAdjacencyList) ArcLabels() map[LI]int {
	s := map[LI]int{}
	for _, to := range g {
		for _, to := range to {
			s[to.Label]++
		}
	}
	return s
}

// NegativeArc returns true if the receiver graph contains a negative arc.
func (g LabeledAdjacencyList) NegativeArc(w WeightFunc) bool {
	for _, nbs := range g {
		for _, nb := range nbs {
			if w(nb.Label) < 0 {
				return true
			}
		}
	}
	return false
}

// ParallelArcsLabel identifies all arcs from node `fr` to node `to` with label `l`.
//
// The returned slice contains an element for each arc from node `fr` to node `to`
// with label `l`.  The element value is the index within the slice of arcs from node
// `fr`.
//
// See also the method HasArcLabel, which stops after finding a single arc.
func (g LabeledAdjacencyList) ParallelArcsLabel(fr, to NI, l LI) (p []int) {
	t := Half{to, l}
	for x, h := range g[fr] {
		if h == t {
			p = append(p, x)
		}
	}
	return
}

// Unlabeled constructs the unlabeled graph corresponding to g.
func (g LabeledAdjacencyList) Unlabeled() AdjacencyList {
	a := make(AdjacencyList, len(g))
	for n, nbs := range g {
		to := make([]NI, len(nbs))
		for i, nb := range nbs {
			to[i] = nb.To
		}
		a[n] = to
	}
	return a
}

// WeightedArcsAsEdges constructs a WeightedEdgeList object from the receiver.
//
// Internally it calls g.ArcsAsEdges() to obtain the Edges member.
// See LabeledAdjacencyList.ArcsAsEdges().
func (g LabeledAdjacencyList) WeightedArcsAsEdges(w WeightFunc) *WeightedEdgeList {
	return &WeightedEdgeList{
		Order:      g.Order(),
		WeightFunc: w,
		Edges:      g.ArcsAsEdges(),
	}
}

// WeightedInDegree computes the weighted in-degree of each node in g
// for a given weight function w.
//
// The weighted in-degree of a node is the sum of weights of arcs going to
// the node.
//
// A weighted degree of a node is often termed the "strength" of a node.
//
// See note for undirected graphs at LabeledAdjacencyList.WeightedOutDegree.
func (g LabeledAdjacencyList) WeightedInDegree(w WeightFunc) []float64 {
	ind := make([]float64, len(g))
	for _, to := range g {
		for _, to := range to {
			ind[to.To] += w(to.Label)
		}
	}
	return ind
}

// WeightedOutDegree computes the weighted out-degree of the specified node
// for a given weight function w.
//
// The weighted out-degree of a node is the sum of weights of arcs going from
// the node.
//
// A weighted degree of a node is often termed the "strength" of a node.
//
// Note for undirected graphs, the WeightedOutDegree result for a node will
// equal the WeightedInDegree for the node.  You can use WeightedInDegree if
// you have need for the weighted degrees of all nodes or use WeightedOutDegree
// to compute the weighted degrees of individual nodes.  In either case loops
// are counted just once, unlike the (unweighted) UndirectedDegree methods.
func (g LabeledAdjacencyList) WeightedOutDegree(n NI, w WeightFunc) (d float64) {
	for _, to := range g[n] {
		d += w(to.Label)
	}
	return
}

// More about loops and strength:  I didn't see consensus on this especially
// in the case of undirected graphs.  Some sources said to add in-degree and
// out-degree, which would seemingly double both loops and non-loops.
// Some said to double loops.  Some said sum the edge weights and had no
// comment on loops.  R of course makes everything an option.  The meaning
// of "strength" where loops exist is unclear.  So while I could write an
// UndirectedWeighted degree function that doubles loops but not edges,
// I'm going to just leave this for now.
