// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

// undir.go has methods specific to undirected graphs, Undirected and
// LabeledUndirected.

import (
	"fmt"

	"github.com/soniakeys/bits"
)

// AddEdge adds an edge to a graph.
//
// It can be useful for constructing undirected graphs.
//
// When n1 and n2 are distinct, it adds the arc n1->n2 and the reciprocal
// n2->n1.  When n1 and n2 are the same, it adds a single arc loop.
//
// The pointer receiver allows the method to expand the graph as needed
// to include the values n1 and n2.  If n1 or n2 happen to be greater than
// len(*p) the method does not panic, but simply expands the graph.
//
// If you know or can compute the final graph order however, consider
// preallocating to avoid any overhead of expanding the graph.
// See second example, "More".
func (p *Undirected) AddEdge(n1, n2 NI) {
	// Similar code in LabeledAdjacencyList.AddEdge.

	// determine max of the two end points
	max := n1
	if n2 > max {
		max = n2
	}
	// expand graph if needed, to include both
	g := p.AdjacencyList
	if int(max) >= len(g) {
		p.AdjacencyList = make(AdjacencyList, max+1)
		copy(p.AdjacencyList, g)
		g = p.AdjacencyList
	}
	// create one half-arc,
	g[n1] = append(g[n1], n2)
	// and except for loops, create the reciprocal
	if n1 != n2 {
		g[n2] = append(g[n2], n1)
	}
}

// RemoveEdge removes a single edge between nodes n1 and n2.
//
// It removes reciprocal arcs in the case of distinct n1 and n2 or removes
// a single arc loop in the case of n1 == n2.
//
// Returns true if the specified edge is found and successfully removed,
// false if the edge does not exist.
func (g Undirected) RemoveEdge(n1, n2 NI) (ok bool) {
	ok, x1, x2 := g.HasEdge(n1, n2)
	if !ok {
		return
	}
	a := g.AdjacencyList
	to := a[n1]
	last := len(to) - 1
	to[x1] = to[last]
	a[n1] = to[:last]
	if n1 == n2 {
		return
	}
	to = a[n2]
	last = len(to) - 1
	to[x2] = to[last]
	a[n2] = to[:last]
	return
}

// ArcDensity returns density for a simple directed graph.
//
// Parameter n is order, or number of nodes of a simple directed graph.
// Parameter a is the arc size, or number of directed arcs.
//
// Returned density is the fraction `a` over the total possible number of arcs
// or a / (n * (n-1)).
//
// See also Density for density of a simple undirected graph.
//
// See also the corresponding methods AdjacencyList.ArcDensity and
// LabeledAdjacencyList.ArcDensity.
func ArcDensity(n, a int) float64 {
	return float64(a) / (float64(n) * float64(n-1))
}

// Density returns density for a simple undirected graph.
//
// Parameter n is order, or number of nodes of a simple undirected graph.
// Parameter m is the size, or number of undirected edges.
//
// Returned density is the fraction m over the total possible number of edges
// or m / ((n * (n-1))/2).
//
// See also ArcDensity for simple directed graphs.
//
// See also the corresponding methods AdjacencyList.Density and
// LabeledAdjacencyList.Density.
func Density(n, m int) float64 {
	return float64(m) * 2 / (float64(n) * float64(n-1))
}

// An EdgeVisitor is an argument to some traversal methods.
//
// Traversal methods call the visitor function for each edge visited.
// Argument e is the edge being visited.
type EdgeVisitor func(e Edge)

// Edges iterates over the edges of an undirected graph.
//
// Edge visitor v is called for each edge of the graph.  That is, it is called
// once for each reciprocal arc pair and once for each loop.
//
// See also LabeledUndirected.Edges for a labeled version.
// See also Undirected.SimpleEdges for a version that emits only the simple
// subgraph.
func (g Undirected) Edges(v EdgeVisitor) {
	a := g.AdjacencyList
	unpaired := make(AdjacencyList, len(a))
	for fr, to := range a {
	arc: // for each arc in a
		for _, to := range to {
			if to == NI(fr) {
				v(Edge{NI(fr), to}) // output loop
				continue
			}
			// search unpaired arcs
			ut := unpaired[to]
			for i, u := range ut {
				if u == NI(fr) { // found reciprocal
					v(Edge{u, to}) // output edge
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
	// undefined behavior is that unpaired arcs are silently ignored.
}

// FromList builds a forest with a tree spanning each connected component.
//
// For each component a root is chosen and spanning is done with the method
// Undirected.SpanTree, and so is breadth-first.  Returned is a FromList with
// all spanned trees, a list of roots chosen, and a bool indicating if the
// receiver graph g was found to be a simple graph connected as a forest.
// Any cycles, loops, or parallel edges in any component will cause
// simpleForest to be false, but FromList f will still be populated with
// a valid and complete spanning forest.
func (g Undirected) FromList() (f FromList, roots []NI, simpleForest bool) {
	p := make([]PathEnd, g.Order())
	for i := range p {
		p[i].From = -1
	}
	f.Paths = p
	simpleForest = true
	ts := 0
	for n := range g.AdjacencyList {
		if p[n].From >= 0 {
			continue
		}
		roots = append(roots, NI(n))
		ns, st := g.SpanTree(NI(n), &f)
		if !st {
			simpleForest = false
		}
		ts += ns
		if ts == len(p) {
			break
		}
	}
	return
}

// HasEdge returns true if g has any edge between nodes n1 and n2.
//
// Also returned are indexes x1 and x2 such that g[n1][x1] == n2
// and g[n2][x2] == n1.  If no edge between n1 and n2 is present HasArc
// returns `has` == false.
//
// See also HasArc.  If you are interested only in the boolean result and
// g is a well formed (passes IsUndirected) then HasArc is an adequate test.
func (g Undirected) HasEdge(n1, n2 NI) (has bool, x1, x2 int) {
	if has, x1 = g.HasArc(n1, n2); !has {
		return has, x1, x1
	}
	has, x2 = g.HasArc(n2, n1)
	return
}

// SimpleEdges iterates over the edges of the simple subgraph of an undirected
// graph.
//
// Edge visitor v is called for each pair of distinct nodes that is connected
// with an edge.  That is, loops are ignored and parallel edges are reduced to
// a single edge.
//
// See also Undirected.Edges for a version that emits all edges.
func (g Undirected) SimpleEdges(v EdgeVisitor) {
	for fr, to := range g.AdjacencyList {
		e := bits.New(len(g.AdjacencyList))
		for _, to := range to {
			if to > NI(fr) && e.Bit(int(to)) == 0 {
				e.SetBit(int(to), 1)
				v(Edge{NI(fr), to})
			}
		}
	}
	// undefined behavior is that unpaired arcs may or may not be emitted.
}

// SpanTree builds a tree spanning a connected component.
//
// The component is spanned by breadth-first search from the given root.
// The resulting spanning tree in stored a FromList.
//
// If FromList.Paths is not the same length as g, it is allocated and
// initialized. This allows a zero value FromList to be passed as f.
// If FromList.Paths is the same length as g, it is used as is and is not
// reinitialized. This allows multiple trees to be spanned in the same
// FromList with successive calls.
//
// For nodes spanned, the Path member of the returned FromList is populated
// with both From and Len values.  The MaxLen member will be updated but
// not Leaves.
//
// Returned is the number of nodes spanned, which will be the number of nodes
// in the component, and a bool indicating if the component was found to be a
// simply connected unrooted tree in the receiver graph g.  Any cycles, loops,
// or parallel edges in the component will cause simpleTree to be false, but
// FromList f will still be populated with a valid and complete spanning tree.
func (g Undirected) SpanTree(root NI, f *FromList) (nSpanned int, simpleTree bool) {
	a := g.AdjacencyList
	p := f.Paths
	if len(p) != len(a) {
		p = make([]PathEnd, len(a))
		for i := range p {
			p[i].From = -1
		}
		f.Paths = p
	}
	simpleTree = true
	p[root] = PathEnd{From: -1, Len: 1}
	type arc struct {
		from NI
		half NI
	}
	var next []arc
	frontier := []arc{{-1, root}}
	for len(frontier) > 0 {
		for _, fa := range frontier { // fa frontier arc
			nSpanned++
			l := p[fa.half].Len + 1
			for _, to := range a[fa.half] {
				if to == fa.from {
					continue
				}
				if p[to].Len > 0 {
					simpleTree = false
					continue
				}
				p[to] = PathEnd{From: fa.half, Len: l}
				if l > f.MaxLen {
					f.MaxLen = l
				}
				next = append(next, arc{fa.half, to})
			}
		}
		frontier, next = next, frontier[:0]
	}
	return
}

// TarjanBiconnectedComponents decomposes a graph into maximal biconnected
// components, components for which if any node were removed the component
// would remain connected.
//
// The receiver g must be a simple graph.  The method calls the emit argument
// for each component identified, as long as emit returns true.  If emit
// returns false, TarjanBiconnectedComponents returns immediately.
//
// See also the eqivalent labeled TarjanBiconnectedComponents.
func (g Undirected) TarjanBiconnectedComponents(emit func([]Edge) bool) {
	// Implemented closely to pseudocode in "Depth-first search and linear
	// graph algorithms", Robert Tarjan, SIAM J. Comput. Vol. 1, No. 2,
	// June 1972.
	//
	// Note Tarjan's "adjacency structure" is graph.AdjacencyList,
	// His "adjacency list" is an element of a graph.AdjacencyList, also
	// termed a "to-list", "neighbor list", or "child list."
	a := g.AdjacencyList
	number := make([]int, len(a))
	lowpt := make([]int, len(a))
	var stack []Edge
	var i int
	var biconnect func(NI, NI) bool
	biconnect = func(v, u NI) bool {
		i++
		number[v] = i
		lowpt[v] = i
		for _, w := range a[v] {
			if number[w] == 0 {
				stack = append(stack, Edge{v, w})
				if !biconnect(w, v) {
					return false
				}
				if lowpt[w] < lowpt[v] {
					lowpt[v] = lowpt[w]
				}
				if lowpt[w] >= number[v] {
					var bcc []Edge
					top := len(stack) - 1
					for number[stack[top].N1] >= number[w] {
						bcc = append(bcc, stack[top])
						stack = stack[:top]
						top--
					}
					bcc = append(bcc, stack[top])
					stack = stack[:top]
					top--
					if !emit(bcc) {
						return false
					}
				}
			} else if number[w] < number[v] && w != u {
				stack = append(stack, Edge{v, w})
				if number[w] < lowpt[v] {
					lowpt[v] = number[w]
				}
			}
		}
		return true
	}
	for w := range a {
		if number[w] == 0 && !biconnect(NI(w), -1) {
			return
		}
	}
}

func (g Undirected) BlockCut(block func([]Edge) bool, cut func(NI) bool, isolated func(NI) bool) {
	a := g.AdjacencyList
	number := make([]int, len(a))
	lowpt := make([]int, len(a))
	var stack []Edge
	var i, rc int
	var biconnect func(NI, NI) bool
	biconnect = func(v, u NI) bool {
		i++
		number[v] = i
		lowpt[v] = i
		for _, w := range a[v] {
			if number[w] == 0 {
				if u < 0 {
					rc++
				}
				stack = append(stack, Edge{v, w})
				if !biconnect(w, v) {
					return false
				}
				if lowpt[w] < lowpt[v] {
					lowpt[v] = lowpt[w]
				}
				if lowpt[w] >= number[v] {
					if u >= 0 && !cut(v) {
						return false
					}
					var bcc []Edge
					top := len(stack) - 1
					for number[stack[top].N1] >= number[w] {
						bcc = append(bcc, stack[top])
						stack = stack[:top]
						top--
					}
					bcc = append(bcc, stack[top])
					stack = stack[:top]
					top--
					if !block(bcc) {
						return false
					}
				}
			} else if number[w] < number[v] && w != u {
				stack = append(stack, Edge{v, w})
				if number[w] < lowpt[v] {
					lowpt[v] = number[w]
				}
			}
		}
		if u < 0 && rc > 1 {
			return cut(v)
		}
		return true
	}
	for w := range a {
		if number[w] > 0 {
			continue
		}
		if len(a[w]) == 0 {
			if !isolated(NI(w)) {
				return
			}
			continue
		}
		rc = 0
		if !biconnect(NI(w), -1) {
			return
		}
	}
}

// AddEdge adds an edge to a labeled graph.
//
// It can be useful for constructing undirected graphs.
//
// When n1 and n2 are distinct, it adds the arc n1->n2 and the reciprocal
// n2->n1.  When n1 and n2 are the same, it adds a single arc loop.
//
// If the edge already exists in *p, a parallel edge is added.
//
// The pointer receiver allows the method to expand the graph as needed
// to include the values n1 and n2.  If n1 or n2 happen to be greater than
// len(*p) the method does not panic, but simply expands the graph.
func (p *LabeledUndirected) AddEdge(e Edge, l LI) {
	// Similar code in AdjacencyList.AddEdge.

	// determine max of the two end points
	max := e.N1
	if e.N2 > max {
		max = e.N2
	}
	// expand graph if needed, to include both
	g := p.LabeledAdjacencyList
	if max >= NI(len(g)) {
		p.LabeledAdjacencyList = make(LabeledAdjacencyList, max+1)
		copy(p.LabeledAdjacencyList, g)
		g = p.LabeledAdjacencyList
	}
	// create one half-arc,
	g[e.N1] = append(g[e.N1], Half{To: e.N2, Label: l})
	// and except for loops, create the reciprocal
	if e.N1 != e.N2 {
		g[e.N2] = append(g[e.N2], Half{To: e.N1, Label: l})
	}
}

// A LabeledEdgeVisitor is an argument to some traversal methods.
//
// Traversal methods call the visitor function for each edge visited.
// Argument e is the edge being visited.
type LabeledEdgeVisitor func(e LabeledEdge)

// Edges iterates over the edges of a labeled undirected graph.
//
// Edge visitor v is called for each edge of the graph.  That is, it is called
// once for each reciprocal arc pair and once for each loop.
//
// See also Undirected.Edges for an unlabeled version.
// See also the more simplistic LabeledAdjacencyList.ArcsAsEdges.
func (g LabeledUndirected) Edges(v LabeledEdgeVisitor) {
	// similar code in LabeledAdjacencyList.InUndirected
	a := g.LabeledAdjacencyList
	unpaired := make(LabeledAdjacencyList, len(a))
	for fr, to := range a {
	arc: // for each arc in a
		for _, to := range to {
			if to.To == NI(fr) {
				v(LabeledEdge{Edge{NI(fr), to.To}, to.Label}) // output loop
				continue
			}
			// search unpaired arcs
			ut := unpaired[to.To]
			for i, u := range ut {
				if u.To == NI(fr) && u.Label == to.Label { // found reciprocal
					v(LabeledEdge{Edge{NI(fr), to.To}, to.Label}) // output edge
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
}

// FromList builds a forest with a tree spanning each connected component in g.
//
// A root is chosen and spanning is done with the LabeledUndirected.SpanTree
// method, and so is breadth-first.  Returned is a FromList with all spanned
// trees, labels corresponding to arcs in f,
// a list of roots chosen, and a bool indicating if the receiver graph g was
// found to be a simple graph connected as a forest.  Any cycles, loops, or
// parallel edges in any component will cause simpleForest to be false, but
// FromList f will still be populated with a valid and complete spanning forest.

// FromList builds a forest with a tree spanning each connected component.
//
// For each component a root is chosen and spanning is done with the method
// Undirected.SpanTree, and so is breadth-first.  Returned is a FromList with
// all spanned trees, labels corresponding to arcs in f, a list of roots
// chosen, and a bool indicating if the receiver graph g was found to be a
// simple graph connected as a forest.  Any cycles, loops, or parallel edges
// in any component will cause simpleForest to be false, but FromList f will
// still be populated with a valid and complete spanning forest.
func (g LabeledUndirected) FromList() (f FromList, labels []LI, roots []NI, simpleForest bool) {
	p := make([]PathEnd, g.Order())
	for i := range p {
		p[i].From = -1
	}
	f.Paths = p
	labels = make([]LI, len(p))
	simpleForest = true
	ts := 0
	for n := range g.LabeledAdjacencyList {
		if p[n].From >= 0 {
			continue
		}
		roots = append(roots, NI(n))
		ns, st := g.SpanTree(NI(n), &f, labels)
		if !st {
			simpleForest = false
		}
		ts += ns
		if ts == len(p) {
			break
		}
	}
	return
}

// SpanTree builds a tree spanning a connected component.
//
// The component is spanned by breadth-first search from the given root.
// The resulting spanning tree in stored a FromList, and arc labels optionally
// stored in a slice.
//
// If FromList.Paths is not the same length as g, it is allocated and
// initialized.  This allows a zero value FromList to be passed as f.
// If FromList.Paths is the same length as g, it is used as is and is not
// reinitialized.  This allows multiple trees to be spanned in the same
// FromList with successive calls.
//
// For nodes spanned, the Path member of returned FromList f is populated
// populated with both From and Len values.  The MaxLen member will be
// updated but not Leaves.
//
// The labels slice will be populated only if it is same length as g.
// Nil can be passed for example if labels are not needed.
//
// Returned is the number of nodes spanned, which will be the number of nodes
// in the component, and a bool indicating if the component was found to be a
// simply connected unrooted tree in the receiver graph g.  Any cycles, loops,
// or parallel edges in the component will cause simpleTree to be false, but
// FromList f will still be populated with a valid and complete spanning tree.
func (g LabeledUndirected) SpanTree(root NI, f *FromList, labels []LI) (nSpanned int, simple bool) {
	a := g.LabeledAdjacencyList
	p := f.Paths
	if len(p) != len(a) {
		p = make([]PathEnd, len(a))
		for i := range p {
			p[i].From = -1
		}
		f.Paths = p
	}
	simple = true
	p[root].Len = 1
	type arc struct {
		from NI
		half Half
	}
	var next []arc
	frontier := []arc{{-1, Half{root, -1}}}
	for len(frontier) > 0 {
		for _, fa := range frontier { // fa frontier arc
			nSpanned++
			l := p[fa.half.To].Len + 1
			for _, to := range a[fa.half.To] {
				if to.To == fa.from && to.Label == fa.half.Label {
					continue
				}
				if p[to.To].Len > 0 {
					simple = false
					continue
				}
				p[to.To] = PathEnd{From: fa.half.To, Len: l}
				if len(labels) == len(p) {
					labels[to.To] = to.Label
				}
				if l > f.MaxLen {
					f.MaxLen = l
				}
				next = append(next, arc{fa.half.To, to})
			}
		}
		frontier, next = next, frontier[:0]
	}
	return
}

// HasEdge returns true if g has any edge between nodes n1 and n2.
//
// Also returned are indexes x1 and x2 such that g[n1][x1] == Half{n2, l}
// and g[n2][x2] == {n1, l} for some label l.  If no edge between n1 and n2
// exists, HasArc returns `has` == false.
//
// See also HasArc.  If you are only interested in the boolean result then
// HasArc is an adequate test.
func (g LabeledUndirected) HasEdge(n1, n2 NI) (has bool, x1, x2 int) {
	if has, x1 = g.HasArc(n1, n2); !has {
		return has, x1, x1
	}
	has, x2 = g.HasArcLabel(n2, n1, g.LabeledAdjacencyList[n1][x1].Label)
	return
}

// HasEdgeLabel returns true if g has any edge between nodes n1 and n2 with
// label l.
//
// Also returned are indexes x1 and x2 such that g[n1][x1] == Half{n2, l}
// and g[n2][x2] == Half{n1, l}.  If no edge between n1 and n2 with label l
// is present HasArc returns `has` == false.
func (g LabeledUndirected) HasEdgeLabel(n1, n2 NI, l LI) (has bool, x1, x2 int) {
	if has, x1 = g.HasArcLabel(n1, n2, l); !has {
		return has, x1, x1
	}
	has, x2 = g.HasArcLabel(n2, n1, l)
	return
}

// RemoveEdge removes a single edge between nodes n1 and n2.
//
// It removes reciprocal arcs in the case of distinct n1 and n2 or removes
// a single arc loop in the case of n1 == n2.
//
// If the specified edge is found and successfully removed, RemoveEdge returns
// true and the label of the edge removed.  If no edge exists between n1 and n2,
// RemoveEdge returns false, 0.
func (g LabeledUndirected) RemoveEdge(n1, n2 NI) (ok bool, label LI) {
	ok, x1, x2 := g.HasEdge(n1, n2)
	if !ok {
		return
	}
	a := g.LabeledAdjacencyList
	to := a[n1]
	label = to[x1].Label // return value
	last := len(to) - 1
	to[x1] = to[last]
	a[n1] = to[:last]
	if n1 == n2 {
		return
	}
	to = a[n2]
	last = len(to) - 1
	to[x2] = to[last]
	a[n2] = to[:last]
	return
}

// RemoveEdgeLabel removes a single edge between nodes n1 and n2 with label l.
//
// It removes reciprocal arcs in the case of distinct n1 and n2 or removes
// a single arc loop in the case of n1 == n2.
//
// Returns true if the specified edge is found and successfully removed,
// false if the edge does not exist.
func (g LabeledUndirected) RemoveEdgeLabel(n1, n2 NI, l LI) (ok bool) {
	ok, x1, x2 := g.HasEdgeLabel(n1, n2, l)
	if !ok {
		return
	}
	a := g.LabeledAdjacencyList
	to := a[n1]
	last := len(to) - 1
	to[x1] = to[last]
	a[n1] = to[:last]
	if n1 == n2 {
		return
	}
	to = a[n2]
	last = len(to) - 1
	to[x2] = to[last]
	a[n2] = to[:last]
	return
}

// TarjanBiconnectedComponents decomposes a graph into maximal biconnected
// components, components for which if any node were removed the component
// would remain connected.
//
// The receiver g must be a simple graph.  The method calls the emit argument
// for each component identified, as long as emit returns true.  If emit
// returns false, TarjanBiconnectedComponents returns immediately.
//
// See also the eqivalent unlabeled TarjanBiconnectedComponents.
func (g LabeledUndirected) TarjanBiconnectedComponents(emit func([]LabeledEdge) bool) {
	// Code nearly identical to unlabled version.
	number := make([]int, g.Order())
	lowpt := make([]int, g.Order())
	var stack []LabeledEdge
	var i int
	var biconnect func(NI, NI) bool
	biconnect = func(v, u NI) bool {
		i++
		number[v] = i
		lowpt[v] = i
		for _, w := range g.LabeledAdjacencyList[v] {
			if number[w.To] == 0 {
				stack = append(stack, LabeledEdge{Edge{v, w.To}, w.Label})
				if !biconnect(w.To, v) {
					return false
				}
				if lowpt[w.To] < lowpt[v] {
					lowpt[v] = lowpt[w.To]
				}
				if lowpt[w.To] >= number[v] {
					var bcc []LabeledEdge
					top := len(stack) - 1
					for number[stack[top].N1] >= number[w.To] {
						bcc = append(bcc, stack[top])
						stack = stack[:top]
						top--
					}
					bcc = append(bcc, stack[top])
					stack = stack[:top]
					top--
					if !emit(bcc) {
						return false
					}
				}
			} else if number[w.To] < number[v] && w.To != u {
				stack = append(stack, LabeledEdge{Edge{v, w.To}, w.Label})
				if number[w.To] < lowpt[v] {
					lowpt[v] = number[w.To]
				}
			}
		}
		return true
	}
	for w := range g.LabeledAdjacencyList {
		if number[w] == 0 && !biconnect(NI(w), -1) {
			return
		}
	}
}

func (e *eulerian) pushUndir() error {
	for u := e.top(); ; {
		e.uv.SetBit(int(u), 0)
		arcs := e.g[u]
		if len(arcs) == 0 {
			return nil
		}
		w := arcs[0]
		e.s++
		e.p[e.s] = w
		e.g[u] = arcs[1:] // consume arc
		// difference from directed counterpart in dir.go:
		// as long as it's not a loop, consume reciprocal arc as well
		if w != u {
			a2 := e.g[w]
			for x, rx := range a2 {
				if rx == u { // here it is
					last := len(a2) - 1
					a2[x] = a2[last]   // someone else gets the seat
					e.g[w] = a2[:last] // and it's gone.
					goto l
				}
			}
			return fmt.Errorf("graph not undirected.  %d -> %d reciprocal not found", u, w)
		}
	l:
		u = w
	}
}

func (e *labEulerian) pushUndir() error {
	for u := e.top(); ; {
		e.uv.SetBit(int(u.To), 0)
		arcs := e.g[u.To]
		if len(arcs) == 0 {
			return nil
		}
		w := arcs[0]
		e.s++
		e.p[e.s] = w
		e.g[u.To] = arcs[1:] // consume arc
		// difference from directed counterpart in dir.go:
		// as long as it's not a loop, consume reciprocal arc as well
		if w.To != u.To {
			a2 := e.g[w.To]
			for x, rx := range a2 {
				if rx.To == u.To && rx.Label == w.Label { // here it is
					last := len(a2) - 1
					a2[x] = a2[last]      // someone else can have the seat
					e.g[w.To] = a2[:last] // and it's gone.
					goto l
				}
			}
			return fmt.Errorf("graph not undirected.  %d -> %v reciprocal not found", u.To, w)
		}
	l:
		u = w
	}
}
