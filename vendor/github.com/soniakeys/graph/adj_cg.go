// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

// adj_RO.go is code generated from adj_cg.go by directives in graph.go.
// Editing adj_cg.go is okay.
// DO NOT EDIT adj_RO.go.  The RO is for Read Only.

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/soniakeys/bits"
)

// ArcDensity returns density for an simple directed graph.
//
// See also ArcDensity function.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) ArcDensity() float64 {
	return ArcDensity(len(g), g.ArcSize())
}

// ArcSize returns the number of arcs in g.
//
// Note that for an undirected graph without loops, the number of undirected
// edges -- the traditional meaning of graph size -- will be ArcSize()/2.
// On the other hand, if g is an undirected graph that has or may have loops,
// g.ArcSize()/2 is not a meaningful quantity.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) ArcSize() int {
	m := 0
	for _, to := range g {
		m += len(to)
	}
	return m
}

// BoundsOk validates that all arcs in g stay within the slice bounds of g.
//
// BoundsOk returns true when no arcs point outside the bounds of g.
// Otherwise it returns false and an example arc that points outside of g.
//
// Most methods of this package assume the BoundsOk condition and may
// panic when they encounter an arc pointing outside of the graph.  This
// function can be used to validate a graph when the BoundsOk condition
// is unknown.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) BoundsOk() (ok bool, fr NI, to Half) {
	for fr, to := range g {
		for _, to := range to {
			if to.To < 0 || to.To >= NI(len(g)) {
				return false, NI(fr), to
			}
		}
	}
	return true, -1, to
}

// BreadthFirst traverses a directed or undirected graph in breadth
// first order.
//
// Traversal starts at node start and visits the nodes reachable from
// start.  The function visit is called for each node visited.  Nodes
// not reachable from start are not visited.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also alt.BreadthFirst, a variant with more options, and
// alt.BreadthFirst2, a direction optimizing variant.
func (g LabeledAdjacencyList) BreadthFirst(start NI, visit func(NI)) {
	v := bits.New(len(g))
	v.SetBit(int(start), 1)
	visit(start)
	var next []NI
	for frontier := []NI{start}; len(frontier) > 0; {
		for _, n := range frontier {
			for _, nb := range g[n] {
				if v.Bit(int(nb.To)) == 0 {
					v.SetBit(int(nb.To), 1)
					visit(nb.To)
					next = append(next, nb.To)
				}
			}
		}
		frontier, next = next, frontier[:0]
	}
}

// Copy makes a deep copy of g.
// Copy also computes the arc size ma, the number of arcs.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) Copy() (c LabeledAdjacencyList, ma int) {
	c = make(LabeledAdjacencyList, len(g))
	for n, to := range g {
		c[n] = append([]Half{}, to...)
		ma += len(to)
	}
	return
}

// DepthFirst traverses a directed or undirected graph in depth
// first order.
//
// Traversal starts at node start and visits the nodes reachable from
// start.  The function visit is called for each node visited.  Nodes
// not reachable from start are not visited.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also alt.DepthFirst, a variant with more options.
func (g LabeledAdjacencyList) DepthFirst(start NI, visit func(NI)) {
	v := bits.New(len(g))
	var f func(NI)
	f = func(n NI) {
		visit(n)
		v.SetBit(int(n), 1)
		for _, to := range g[n] {
			if v.Bit(int(to.To)) == 0 {
				f(to.To)
			}
		}
	}
	f(start)
}

// HasArc returns true if g has any arc from node `fr` to node `to`.
//
// Also returned is the index within the slice of arcs from node `fr`.
// If no arc from `fr` to `to` is present, HasArc returns false, -1.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also the method ParallelArcs, which finds all parallel arcs from
// `fr` to `to`.
func (g LabeledAdjacencyList) HasArc(fr, to NI) (bool, int) {
	for x, h := range g[fr] {
		if h.To == to {
			return true, x
		}
	}
	return false, -1
}

// AnyLoop identifies if a graph contains a loop, an arc that leads from a
// a node back to the same node.
//
// If g contains a loop, the method returns true and an example of a node
// with a loop.  If there are no loops in g, the method returns false, -1.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) AnyLoop() (bool, NI) {
	for fr, to := range g {
		for _, to := range to {
			if NI(fr) == to.To {
				return true, to.To
			}
		}
	}
	return false, -1
}

// AddNode maps a node in a supergraph to a subgraph node.
//
// Argument p must be an NI in supergraph s.Super.  AddNode panics if
// p is not a valid node index of s.Super.
//
// AddNode is idempotent in that it does not add a new node to the subgraph if
// a subgraph node already exists mapped to supergraph node p.
//
// The mapped subgraph NI is returned.
func (s *LabeledSubgraph) AddNode(p NI) (b NI) {
	if int(p) < 0 || int(p) >= s.Super.Order() {
		panic(fmt.Sprint("AddNode: NI ", p, " not in supergraph"))
	}
	if b, ok := s.SubNI[p]; ok {
		return b
	}
	a := s.LabeledAdjacencyList
	b = NI(len(a))
	s.LabeledAdjacencyList = append(a, nil)
	s.SuperNI = append(s.SuperNI, p)
	s.SubNI[p] = b
	return
}

// AddArc adds an arc to a subgraph.
//
// Arguments fr, to must be NIs in supergraph s.Super.  As with AddNode,
// AddArc panics if fr and to are not valid node indexes of s.Super.
//
// The arc specfied by fr, to must exist in s.Super.  Further, the number of
// parallel arcs in the subgraph cannot exceed the number of corresponding
// parallel arcs in the supergraph.  That is, each arc already added to the
// subgraph counts against the arcs available in the supergraph.  If a matching
// arc is not available, AddArc returns an error.
//
// If a matching arc is available, subgraph nodes are added as needed, the
// subgraph arc is added, and the method returns nil.
func (s *LabeledSubgraph) AddArc(fr NI, to Half) error {
	// verify supergraph NIs first, but without adding subgraph nodes just yet.
	if int(fr) < 0 || int(fr) >= s.Super.Order() {
		panic(fmt.Sprint("AddArc: NI ", fr, " not in supergraph"))
	}
	if int(to.To) < 0 || int(to.To) >= s.Super.Order() {
		panic(fmt.Sprint("AddArc: NI ", to.To, " not in supergraph"))
	}
	// count existing matching arcs in subgraph
	n := 0
	a := s.LabeledAdjacencyList
	if bf, ok := s.SubNI[fr]; ok {
		if bt, ok := s.SubNI[to.To]; ok {
			// both NIs already exist in subgraph, need to count arcs
			bTo := to
			bTo.To = bt
			for _, t := range a[bf] {
				if t == bTo {
					n++
				}
			}
		}
	}
	// verify matching arcs are available in supergraph
	for _, t := range (*s.Super)[fr] {
		if t == to {
			if n > 0 {
				n-- // match existing arc
				continue
			}
			// no more existing arcs need to be matched.  nodes can finally
			// be added as needed and then the arc can be added.
			bf := s.AddNode(fr)
			to.To = s.AddNode(to.To)
			s.LabeledAdjacencyList[bf] = append(s.LabeledAdjacencyList[bf], to)
			return nil // success
		}
	}
	return errors.New("arc not available in supergraph")
}

func (super LabeledAdjacencyList) induceArcs(sub map[NI]NI, sup []NI) LabeledAdjacencyList {
	s := make(LabeledAdjacencyList, len(sup))
	for b, p := range sup {
		var a []Half
		for _, to := range super[p] {
			if bt, ok := sub[to.To]; ok {
				to.To = bt
				a = append(a, to)
			}
		}
		s[b] = a
	}
	return s
}

// InduceList constructs a node-induced subgraph.
//
// The subgraph is induced on receiver graph g.  Argument l must be a list of
// NIs in receiver graph g.  Receiver g becomes the supergraph of the induced
// subgraph.
//
// Duplicate NIs are allowed in list l.  The duplicates are effectively removed
// and only a single corresponding node is created in the subgraph.  Subgraph
// NIs are mapped in the order of list l, execpt for ignoring duplicates.
// NIs in l that are not in g will panic.
//
// Returned is the constructed Subgraph object containing the induced subgraph
// and the mappings to the supergraph.
func (g *LabeledAdjacencyList) InduceList(l []NI) *LabeledSubgraph {
	sub, sup := mapList(l)
	return &LabeledSubgraph{
		Super:   g,
		SubNI:   sub,
		SuperNI: sup,

		LabeledAdjacencyList: g.induceArcs(sub, sup)}
}

// InduceBits constructs a node-induced subgraph.
//
// The subgraph is induced on receiver graph g.  Argument t must be a bitmap
// representing NIs in receiver graph g.  Receiver g becomes the supergraph
// of the induced subgraph.  NIs in t that are not in g will panic.
//
// Returned is the constructed Subgraph object containing the induced subgraph
// and the mappings to the supergraph.
func (g *LabeledAdjacencyList) InduceBits(t bits.Bits) *LabeledSubgraph {
	sub, sup := mapBits(t)
	return &LabeledSubgraph{
		Super:   g,
		SubNI:   sub,
		SuperNI: sup,

		LabeledAdjacencyList: g.induceArcs(sub, sup)}
}

// IsSimple checks for loops and parallel arcs.
//
// A graph is "simple" if it has no loops or parallel arcs.
//
// IsSimple returns true, -1 for simple graphs.  If a loop or parallel arc is
// found, simple returns false and a node that represents a counterexample
// to the graph being simple.
//
// See also separate methods AnyLoop and AnyParallel.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) IsSimple() (ok bool, n NI) {
	if lp, n := g.AnyLoop(); lp {
		return false, n
	}
	if pa, n, _ := g.AnyParallel(); pa {
		return false, n
	}
	return true, -1
}

// IsolatedNodes returns a bitmap of isolated nodes in receiver graph g.
//
// An isolated node is one with no arcs going to or from it.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) IsolatedNodes() (i bits.Bits) {
	i = bits.New(len(g))
	i.SetAll()
	for fr, to := range g {
		if len(to) > 0 {
			i.SetBit(fr, 0)
			for _, to := range to {
				i.SetBit(int(to.To), 0)
			}
		}
	}
	return
}

// Order is the number of nodes in receiver g.
//
// It is simply a wrapper method for the Go builtin len().
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) Order() int {
	// Why a wrapper for len()?  Mostly for Directed and Undirected.
	// u.Order() is a little nicer than len(u.LabeledAdjacencyList).
	return len(g)
}

// ParallelArcs identifies all arcs from node `fr` to node `to`.
//
// The returned slice contains an element for each arc from node `fr` to node `to`.
// The element value is the index within the slice of arcs from node `fr`.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also the method HasArc, which stops after finding a single arc.
func (g LabeledAdjacencyList) ParallelArcs(fr, to NI) (p []int) {
	for x, h := range g[fr] {
		if h.To == to {
			p = append(p, x)
		}
	}
	return
}

// Permute permutes the node labeling of receiver g.
//
// Argument p must be a permutation of the node numbers of the graph,
// 0 through len(g)-1.  A permutation returned by rand.Perm(len(g)) for
// example is acceptable.
//
// The graph is permuted in place.  The graph keeps the same underlying
// memory but values of the graph representation are permuted to produce
// an isomorphic graph.  The node previously labeled 0 becomes p[0] and so on.
// See example (or the code) for clarification.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) Permute(p []int) {
	old := append(LabeledAdjacencyList{}, g...) // shallow copy
	for fr, arcs := range old {
		for i, to := range arcs {
			arcs[i].To = NI(p[to.To])
		}
		g[p[fr]] = arcs
	}
}

// ShuffleArcLists shuffles the arc lists of each node of receiver g.
//
// For example a node with arcs leading to nodes 3 and 7 might have an
// arc list of either [3 7] or [7 3] after calling this method.  The
// connectivity of the graph is not changed.  The resulting graph stays
// equivalent but a traversal will encounter arcs in a different
// order.
//
// If Rand r is nil, the rand package default shared source is used.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledAdjacencyList) ShuffleArcLists(r *rand.Rand) {
	ri := rand.Intn
	if r != nil {
		ri = r.Intn
	}
	// Knuth-Fisher-Yates
	for _, to := range g {
		for i := len(to); i > 1; {
			j := ri(i)
			i--
			to[i], to[j] = to[j], to[i]
		}
	}
}
