// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

import (
	"errors"
	"fmt"

	"github.com/soniakeys/bits"
)

// dir_RO.go is code generated from dir_cg.go by directives in graph.go.
// Editing dir_cg.go is okay.  It is the code generation source.
// DO NOT EDIT dir_RO.go.
// The RO means read only and it is upper case RO to slow you down a bit
// in case you start to edit the file.

// Balanced returns true if for every node in g, in-degree equals out-degree.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) Balanced() bool {
	for n, in := range g.InDegree() {
		if in != len(g.LabeledAdjacencyList[n]) {
			return false
		}
	}
	return true
}

// Copy makes a deep copy of g.
// Copy also computes the arc size ma, the number of arcs.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) Copy() (c LabeledDirected, ma int) {
	l, s := g.LabeledAdjacencyList.Copy()
	return LabeledDirected{l}, s
}

// Cyclic determines if g contains a cycle, a non-empty path from a node
// back to itself.
//
// Cyclic returns true if g contains at least one cycle.  It also returns
// an example of an arc involved in a cycle.
// Cyclic returns false if g is acyclic.
//
// Also see Topological, which detects cycles.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) Cyclic() (cyclic bool, fr NI, to Half) {
	a := g.LabeledAdjacencyList
	fr, to.To = -1, -1
	temp := bits.New(len(a))
	perm := bits.New(len(a))
	var df func(int)
	df = func(n int) {
		switch {
		case temp.Bit(n) == 1:
			cyclic = true
			return
		case perm.Bit(n) == 1:
			return
		}
		temp.SetBit(n, 1)
		for _, nb := range a[n] {
			df(int(nb.To))
			if cyclic {
				if fr < 0 {
					fr, to = NI(n), nb
				}
				return
			}
		}
		temp.SetBit(n, 0)
		perm.SetBit(n, 1)
	}
	for n := range a {
		if perm.Bit(n) == 1 {
			continue
		}
		if df(n); cyclic { // short circuit as soon as a cycle is found
			break
		}
	}
	return
}

// DegreeCentralization returns out-degree centralization.
//
// Out-degree of a node is one measure of node centrality and is directly
// available from the adjacency list representation.  This allows degree
// centralization for the graph to be very efficiently computed.
//
// The value returned is from 0 to 1 inclusive for simple directed graphs of
// two or more nodes.  As a special case, 0 is returned for graphs of 0 or 1
// nodes.  The value returned can be > 1 for graphs with loops or parallel
// edges.
//
// In-degree centralization can be computed as DegreeCentralization of the
// transpose.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) DegreeCentralization() float64 {
	a := g.LabeledAdjacencyList
	if len(a) <= 1 {
		return 0
	}
	var max, sum int
	for _, to := range a {
		if len(to) > max {
			max = len(to)
		}
		sum += len(to)
	}
	l1 := len(a) - 1
	return float64(len(a)*max-sum) / float64(l1*l1)
}

// Dominators computes the immediate dominator for each node reachable from
// start.
//
// The slice returned as Dominators.Immediate will have the length of
// g.AdjacencyList.  Nodes without a path to end will have a value of -1.
//
// See also the method Doms.  Internally Dominators must construct the
// transpose of g and also compute a postordering of a spanning tree of the
// subgraph reachable from start.  If you happen to have either of these
// computed anyway, it can be more efficient to call Doms directly.
func (g LabeledDirected) Dominators(start NI) Dominators {
	a := g.LabeledAdjacencyList
	l := len(a)
	// ExampleDoms shows traditional depth-first postorder, but it works to
	// generate a reverse preorder.  Also breadth-first works instead of
	// depth-first and may allow Doms to run a little faster by presenting
	// a shallower tree.
	post := make([]NI, l)
	a.BreadthFirst(start, func(n NI) {
		l--
		post[l] = n
	})
	tr, _ := g.Transpose()
	return g.Doms(tr, post[l:])
}

// Doms computes either immediate dominators or postdominators.
//
// The slice returned as Dominators.Immediate will have the length of
// g.AdjacencyList.  Nodes without a path to end will have a value of -1.
//
// But see also the simpler methods Dominators and PostDominators.
//
// Doms requires argument tr to be the transpose graph of receiver g,
// and requres argument post to be a post ordering of receiver g.  More
// specifically a post ordering of a spanning tree of the subgraph reachable
// from some start node in g.  The start node will always be the last node in
// this postordering so it does not need to passed as a separate argument.
//
// Doms can be used to construct either dominators or postdominators.
// To construct dominators on a graph f, generate a postordering p on f
// and call f.Doms(f.Transpose(), p).  To construct postdominators, generate
// the transpose t first, then a postordering p on t (not f), and call
// t.Doms(f, p).
//
// Caution:  The argument tr is retained in the returned Dominators object
// and is used by the method Dominators.Frontier.  It is not deep-copied
// so it is invalid to call Doms, modify the tr graph, and then call Frontier.
func (g LabeledDirected) Doms(tr LabeledDirected, post []NI) Dominators {
	a := g.LabeledAdjacencyList
	dom := make([]NI, len(a))
	pi := make([]int, len(a))
	for i, n := range post {
		pi[n] = i
	}
	intersect := func(b1, b2 NI) NI {
		for b1 != b2 {
			for pi[b1] < pi[b2] {
				b1 = dom[b1]
			}
			for pi[b2] < pi[b1] {
				b2 = dom[b2]
			}
		}
		return b1
	}
	for n := range dom {
		dom[n] = -1
	}
	start := post[len(post)-1]
	dom[start] = start
	for changed := false; ; changed = false {
		for i := len(post) - 2; i >= 0; i-- {
			b := post[i]
			var im NI
			fr := tr.LabeledAdjacencyList[b]
			var j int
			var fp Half
			for j, fp = range fr {
				if dom[fp.To] >= 0 {
					im = fp.To
					break
				}
			}
			for _, p := range fr[j:] {
				if dom[p.To] >= 0 {
					im = intersect(im, p.To)
				}
			}
			if dom[b] != im {
				dom[b] = im
				changed = true
			}
		}
		if !changed {
			return Dominators{dom, tr}
		}
	}
}

// PostDominators computes the immediate postdominator for each node that can
// reach node end.
//
// The slice returned as Dominators.Immediate will have the length of
// g.AdjacencyList.  Nodes without a path to end will have a value of -1.
//
// See also the method Doms.  Internally Dominators must construct the
// transpose of g and also compute a postordering of a spanning tree of the
// subgraph of the transpose reachable from end.  If you happen to have either
// of these computed anyway, it can be more efficient to call Doms directly.
//
// See the method Doms anyway for the caution note.  PostDominators calls
// Doms internally, passing receiver g as Doms argument tr.  The caution means
// that it is invalid to call PostDominators, modify the graph g, then call
// Frontier.
func (g LabeledDirected) PostDominators(end NI) Dominators {
	tr, _ := g.Transpose()
	a := tr.LabeledAdjacencyList
	l := len(a)
	post := make([]NI, l)
	a.BreadthFirst(end, func(n NI) {
		l--
		post[l] = n
	})
	return tr.Doms(g, post[l:])
}

// called from Dominators.Frontier via interface
func (from LabeledDirected) domFrontiers(d Dominators) DominanceFrontiers {
	im := d.Immediate
	f := make(DominanceFrontiers, len(im))
	for i := range f {
		if im[i] >= 0 {
			f[i] = map[NI]struct{}{}
		}
	}
	for b, fr := range from.LabeledAdjacencyList {
		if len(fr) < 2 {
			continue
		}
		imb := im[b]
		for _, p := range fr {
			for runner := p.To; runner != imb; runner = im[runner] {
				f[runner][NI(b)] = struct{}{}
			}
		}
	}
	return f
}

// Eulerian scans a directed graph to determine if it is Eulerian.
//
// If the graph represents an Eulerian cycle, it returns -1, -1, nil.
//
// If the graph does not represent an Eulerian cycle but does represent an
// Eulerian path, it returns the start and end nodes of the path, and nil.
//
// Otherwise it returns an error indicating a reason the graph is non-Eulerian.
// Also in this case it returns a relevant node in either start or end.
//
// See also method EulerianStart, which short-circuits when it finds a start
// node whereas this method completely validates a graph as Eulerian.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) Eulerian() (start, end NI, err error) {
	ind := g.InDegree()
	start = -1
	end = -1
	for n, to := range g.LabeledAdjacencyList {
		switch {
		case len(to) > ind[n]:
			if start >= 0 {
				return NI(n), -1, errors.New("multiple start candidates")
			}
			if len(to) > ind[n]+1 {
				return NI(n), -1, errors.New("excessive out-degree")
			}
			start = NI(n)
		case ind[n] > len(to):
			if end >= 0 {
				return -1, NI(n), errors.New("multiple end candidates")
			}
			if ind[n] > len(to)+1 {
				return -1, NI(n), errors.New("excessive in-degree")
			}
			end = NI(n)
		}
	}
	return start, end, nil
}

// EulerianCycle finds an Eulerian cycle in a directed multigraph.
//
// * If g has no nodes, result is nil, nil.
//
// * If g is Eulerian, result is an Eulerian cycle with err = nil.
// The first element of the result represents only a start node.
// The remaining elements represent the half arcs of the cycle.
//
// * Otherwise, result is nil, with a non-nil error giving a reason the graph
// is not Eulerian.
//
// Internally, EulerianCycle copies the entire graph g.
// See EulerianCycleD for a more space efficient version.
//
// There are nearly equivalent labeled and unlabeled versions of this method.
// In the labeled version the first element of of the
func (g LabeledDirected) EulerianCycle() ([]Half, error) {
	c, m := g.Copy()
	return c.EulerianCycleD(m)
}

// EulerianCycleD finds an Eulerian cycle in a directed multigraph.
//
// EulerianCycleD is destructive on its receiver g.  See EulerianCycle for
// a non-destructive version.
//
// Argument ma must be the correct arc size, or number of arcs in g.
//
// * If g has no nodes, result is nil, nil.
//
// * If g is Eulerian, result is an Eulerian cycle with err = nil.
// The first element of the result represents only a start node.
// The remaining elements represent the half arcs of the cycle.
//
// * Otherwise, result is nil, with a non-nil error giving a reason the graph
// is not Eulerian.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) EulerianCycleD(ma int) ([]Half, error) {
	// algorithm adapted from "Sketch of Eulerian Circuit Algorithm" by
	// Carl Lee, accessed at http://www.ms.uky.edu/~lee/ma515fa10/euler.pdf.
	if g.Order() == 0 {
		return nil, nil
	}
	e := newLabEulerian(g.LabeledAdjacencyList, ma)
	e.p[0] = Half{0, -1}
	for e.s >= 0 {
		v := e.top() // v is node that starts cycle
		e.push()
		// if Eulerian, we'll always come back to starting node
		if e.top().To != v.To {
			return nil, errors.New("not Eulerian")
		}
		e.keep()
	}
	if !e.uv.AllZeros() {
		return nil, errors.New("not strongly connected")
	}
	return e.p, nil
}

// EulerianPath finds an Eulerian path in a directed multigraph.
//
// * If g has no nodes, result is nil, nil.
//
// * If g has an Eulerian path, result is an Eulerian path with err = nil.
// The first element of the result represents only a start node.
// The remaining elements represent the half arcs of the path.
//
// * Otherwise, result is nil, with a non-nil error giving a reason the graph
// is not Eulerian.
//
// Internally, EulerianPath copies the entire graph g.
// See EulerianPathD for a more space efficient version.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) EulerianPath() ([]Half, error) {
	c, m := g.Copy()
	start, err := c.EulerianStart()
	if err != nil {
		return nil, err
	}
	if start < 0 {
		start = 0
	}
	return c.EulerianPathD(m, start)
}

// EulerianPathD finds an Eulerian path in a directed multigraph.
//
// EulerianPathD is destructive on its receiver g.  See EulerianPath for
// a non-destructive version.
//
// Argument ma must be the correct arc size, or number of arcs in g.
// Argument start must be a valid start node for the path.
//
// * If g has no nodes, result is nil, nil.
//
// * If g has an Eulerian path starting at start, result is an Eulerian path
// with err = nil.
// The first element of the result represents only a start node.
// The remaining elements represent the half arcs of the path.
//
// * Otherwise, result is nil, with a non-nil error giving a reason the graph
// is not Eulerian.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) EulerianPathD(ma int, start NI) ([]Half, error) {
	if g.Order() == 0 {
		return nil, nil
	}
	e := newLabEulerian(g.LabeledAdjacencyList, ma)
	e.p[0] = Half{start, -1}
	// unlike EulerianCycle, the first path doesn't have to be a cycle.
	e.push()
	e.keep()
	for e.s >= 0 {
		start = e.top().To
		e.push()
		// paths after the first must be cycles though
		// (as long as there are nodes on the stack)
		if e.top().To != start {
			return nil, errors.New("no Eulerian path")
		}
		e.keep()
	}
	if !e.uv.AllZeros() {
		return nil, errors.New("no Eulerian path")
	}
	return e.p, nil
}

// EulerianStart finds a candidate start node for an Eulerian path.
//
// A candidate start node in the directed case has out-degree one greater then
// in-degree.  EulerianStart scans the graph returning immediately with the
// node (and err == nil) when it finds such a candidate.
//
// EulerianStart also returns immediately with an error if it finds the graph
// cannot contain an Eulerian path.  In this case it also returns a relevant
// node.
//
// If the scan completes without finding a candidate start node, the graph
// represents an Eulerian cycle.  In this case it returns -1, nil, and any
// node can be chosen as a start node for an eulerian path.
//
// See also method Eulerian, which completely validates a graph as Eulerian
// whereas this method short-curcuits when it finds a start node.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) EulerianStart() (start NI, err error) {
	ind := g.InDegree()
	end := -1
	for n, to := range g.LabeledAdjacencyList {
		switch {
		case len(to) > ind[n]:
			if len(to) == ind[n]+1 {
				return NI(n), nil // candidate start
			}
			return -1, errors.New("excessive out-degree")
		case ind[n] > len(to):
			if end >= 0 {
				return NI(n), errors.New("multiple end candidates")
			}
			if ind[n] > len(to)+1 {
				return NI(n), errors.New("excessive in-degree")
			}
			end = n
		}
	}
	return -1, nil // cycle
}

type labEulerian struct {
	g  LabeledAdjacencyList // working copy of graph, it gets consumed
	m  int                  // number of arcs in g, updated as g is consumed
	uv bits.Bits            // unvisited
	// low end of p is stack of unfinished nodes
	// high end is finished path
	p []Half // stack + path
	s int    // stack pointer
}

func newLabEulerian(g LabeledAdjacencyList, m int) *labEulerian {
	e := &labEulerian{
		g:  g,
		m:  m,
		uv: bits.New(len(g)),
		p:  make([]Half, m+1),
	}
	e.uv.SetAll()
	return e
}

// starting with the node on top of the stack, move nodes with no arcs.
func (e *labEulerian) keep() {
	for e.s >= 0 {
		n := e.top()
		if len(e.g[n.To]) > 0 {
			break
		}
		e.p[e.m] = n
		e.s--
		e.m--
	}
}

func (e *labEulerian) top() Half {
	return e.p[e.s]
}

// MaximalNonBranchingPaths finds all paths in a directed graph that are
// "maximal" and "non-branching".
//
// A non-branching path is one where path nodes other than the first and last
// have exactly one arc leading to the node and one arc leading from the node,
// thus there is no possibility to branch away to a different path.
//
// A maximal non-branching path cannot be extended to a longer non-branching
// path by including another node at either end.
//
// In the case of a cyclic non-branching path, the first and last nodes
// of the path will be the same node, indicating an isolated cycle.
//
// The method calls the emit argument for each path or isolated cycle in g,
// as long as emit returns true.  If emit returns false,
// MaximalNonBranchingPaths returns immediately.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) MaximalNonBranchingPaths(emit func([]Half) bool) {
	a := g.LabeledAdjacencyList
	ind := g.InDegree()
	uv := bits.New(g.Order())
	uv.SetAll()
	for v, vTo := range a {
		if !(ind[v] == 1 && len(vTo) == 1) {
			for _, w := range vTo {
				n := []Half{Half{NI(v), -1}, w}
				uv.SetBit(v, 0)
				uv.SetBit(int(w.To), 0)
				wTo := a[w.To]
				for ind[w.To] == 1 && len(wTo) == 1 {
					u := wTo[0]
					n = append(n, u)
					uv.SetBit(int(u.To), 0)
					w = u
					wTo = a[w.To]
				}
				if !emit(n) { // n is a path
					return
				}
			}
		}
	}
	// use uv.From rather than uv.Iterate.
	// Iterate doesn't work here because we're modifying uv
	for b := uv.OneFrom(0); b >= 0; b = uv.OneFrom(b + 1) {
		v := Half{NI(b), -1}
		n := []Half{v}
		for w := v; ; {
			w = a[w.To][0]
			uv.SetBit(int(w.To), 0)
			n = append(n, w)
			if w.To == v.To {
				break
			}
		}
		if !emit(n) { // n is an isolated cycle
			return
		}
	}
}

// InDegree computes the in-degree of each node in g
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) InDegree() []int {
	ind := make([]int, g.Order())
	for _, nbs := range g.LabeledAdjacencyList {
		for _, nb := range nbs {
			ind[nb.To]++
		}
	}
	return ind
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
func (s *LabeledDirectedSubgraph) AddNode(p NI) (b NI) {
	if int(p) < 0 || int(p) >= s.Super.Order() {
		panic(fmt.Sprint("AddNode: NI ", p, " not in supergraph"))
	}
	if b, ok := s.SubNI[p]; ok {
		return b
	}
	a := s.LabeledDirected.LabeledAdjacencyList
	b = NI(len(a))
	s.LabeledDirected.LabeledAdjacencyList = append(a, nil)
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
func (s *LabeledDirectedSubgraph) AddArc(fr NI, to Half) error {
	// verify supergraph NIs first, but without adding subgraph nodes just yet.
	if int(fr) < 0 || int(fr) >= s.Super.Order() {
		panic(fmt.Sprint("AddArc: NI ", fr, " not in supergraph"))
	}
	if int(to.To) < 0 || int(to.To) >= s.Super.Order() {
		panic(fmt.Sprint("AddArc: NI ", to.To, " not in supergraph"))
	}
	// count existing matching arcs in subgraph
	n := 0
	a := s.LabeledDirected.LabeledAdjacencyList
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
	for _, t := range (*s.Super).LabeledAdjacencyList[fr] {
		if t == to {
			if n > 0 {
				n-- // match existing arc
				continue
			}
			// no more existing arcs need to be matched.  nodes can finally
			// be added as needed and then the arc can be added.
			bf := s.AddNode(fr)
			to.To = s.AddNode(to.To)
			s.LabeledDirected.LabeledAdjacencyList[bf] =
				append(s.LabeledDirected.LabeledAdjacencyList[bf], to)
			return nil // success
		}
	}
	return errors.New("arc not available in supergraph")
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
func (g *LabeledDirected) InduceList(l []NI) *LabeledDirectedSubgraph {
	sub, sup := mapList(l)
	return &LabeledDirectedSubgraph{
		Super:   g,
		SubNI:   sub,
		SuperNI: sup,
		LabeledDirected: LabeledDirected{
			g.LabeledAdjacencyList.induceArcs(sub, sup),
		}}
}

// InduceBits constructs a node-induced subgraph.
//
// The subgraph is induced on receiver graph g.  Argument t must be a bitmap
// representing NIs in receiver graph g.  Receiver g becomes the supergraph
// of the induced subgraph.  NIs in t that are not in g will panic.
//
// Returned is the constructed Subgraph object containing the induced subgraph
// and the mappings to the supergraph.
func (g *LabeledDirected) InduceBits(t bits.Bits) *LabeledDirectedSubgraph {
	sub, sup := mapBits(t)
	return &LabeledDirectedSubgraph{
		Super:   g,
		SubNI:   sub,
		SuperNI: sup,
		LabeledDirected: LabeledDirected{
			g.LabeledAdjacencyList.induceArcs(sub, sup),
		}}
}

// IsTree identifies trees in directed graphs.
//
// Return value isTree is true if the subgraph reachable from root is a tree.
// Further, return value allTree is true if the entire graph g is reachable
// from root.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) IsTree(root NI) (isTree, allTree bool) {
	a := g.LabeledAdjacencyList
	v := bits.New(len(a))
	v.SetAll()
	var df func(NI) bool
	df = func(n NI) bool {
		if v.Bit(int(n)) == 0 {
			return false
		}
		v.SetBit(int(n), 0)
		for _, to := range a[n] {
			if !df(to.To) {
				return false
			}
		}
		return true
	}
	isTree = df(root)
	return isTree, isTree && v.AllZeros()
}

// PageRank computes a significance score for each node of a graph.
//
// The algorithm is credited to Google founders Brin and Lawrence.
//
// Argument d is a damping factor.  Reportedly a value of .85 works well.
// Argument n is a number of iterations.  Reportedly values of 20 to 50
// work well.
//
// Returned is the PageRank score for each node of g.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) PageRank(d float64, n int) []float64 {
	// Following "PageRank Explained" by Ian Rogers, accessed at
	// http://www.cs.princeton.edu/~chazelle/courses/BIB/pagerank.htm
	a := g.LabeledAdjacencyList
	p0 := make([]float64, len(a))
	p1 := make([]float64, len(a))
	for i := range p0 {
		p0[i] = 1
	}
	d1 := 1 - d
	for ; n > 0; n-- {
		for i := range p1 {
			p1[i] = d1
		}
		for fr, to := range a {
			f := d / float64(len(to))
			for _, to := range to {
				p1[to.To] += p0[fr] * f
			}
		}
		p0, p1 = p1, p0
	}
	return p0
}

// StronglyConnectedComponents identifies strongly connected components in
// a directed graph.
//
// The method calls the emit function for each component identified.  The
// argument to emit is the node list of a component.  The emit function must
// return true for the method to continue identifying components.  If emit
// returns false, the method returns immediately.
//
// Note well:  The backing slice for the node list passed to emit is reused
// across emit calls.  If you need to retain the node list you must copy it.
//
// The components emitted represent a partition of the nodes in g.
// So for example, if the first component emitted has the same length as g
// then it will be the only component and it means the entire graph g is
// strongly connected.
//
// See also Condensation which returns a condensation graph in addition
// to the strongly connected components.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// The algorithm here is by David Pearce.  See also alt.SCCPathBased and
// alt.SCCTarjan.
func (g LabeledDirected) StronglyConnectedComponents(emit func([]NI) bool) {
	// See Algorithm 3 PEA FIND SCC2(V,E) in "An Improved Algorithm for
	// Finding the Strongly Connected Components of a Directed Graph"
	// by David J. Pearce.
	a := g.LabeledAdjacencyList
	rindex := make([]int, len(a))
	var S, scc []NI
	index := 1
	c := len(a) - 1
	var visit func(NI) bool
	visit = func(v NI) bool {
		root := true
		rindex[v] = index
		index++
		for _, w := range a[v] {
			if rindex[w.To] == 0 {
				if !visit(w.To) {
					return false
				}
			}
			if rindex[w.To] < rindex[v] {
				rindex[v] = rindex[w.To]
				root = false
			}
		}
		if !root {
			S = append(S, v)
			return true
		}
		scc = scc[:0]
		index--
		for last := len(S) - 1; last >= 0; last-- {
			w := S[last]
			if rindex[v] > rindex[w] {
				break
			}
			S = S[:last]
			rindex[w] = c
			scc = append(scc, w)
			index--
		}
		rindex[v] = c
		c--
		return emit(append(scc, v))
	}
	for v := range a {
		if rindex[v] == 0 && !visit(NI(v)) {
			break
		}
	}
}

// Condensation returns strongly connected components and their
// condensation graph.
//
// A condensation represents a directed acyclic graph.
// Components are ordered in a reverse topological ordering.
//
// See also StronglyConnectedComponents, which returns the components only.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) Condensation() (scc [][]NI, cd AdjacencyList) {
	a := g.LabeledAdjacencyList
	b := make([]NI, len(a)) // backing slice for scc
	g.StronglyConnectedComponents(func(c []NI) bool {
		n := copy(b, c)
		scc = append(scc, b[:n])
		b = b[n:]
		return true
	})
	cd = make(AdjacencyList, len(scc)) // return value
	cond := make([]NI, len(a))         // mapping from g node to cd node
	for cn, c := range scc {
		for _, n := range c {
			cond[n] = NI(cn) // map g node to cd node
		}
		var tos []NI           // list of 'to' nodes
		m := bits.New(len(cd)) // tos map
		m.SetBit(cn, 1)
		for _, n := range c {
			for _, to := range a[n] {
				if ct := cond[to.To]; m.Bit(int(ct)) == 0 {
					m.SetBit(int(ct), 1)
					tos = append(tos, ct)
				}
			}
		}
		cd[cn] = tos
	}
	return
}

// Topological computes a topological ordering of a directed acyclic graph.
//
// For an acyclic graph, return value ordering is a permutation of node numbers
// in topologically sorted order and cycle will be nil.  If the graph is found
// to be cyclic, ordering will be nil and cycle will be the path of a found
// cycle.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) Topological() (ordering, cycle []NI) {
	i := -1
	return g.dfTopo(func() NI {
		i++
		if i < g.Order() {
			return NI(i)
		}
		return -1
	})
}

func (g LabeledDirected) dfTopo(f func() NI) (ordering, cycle []NI) {
	a := g.LabeledAdjacencyList
	ordering = make([]NI, len(a))
	i := len(ordering)
	temp := bits.New(len(a))
	perm := bits.New(len(a))
	var cycleFound bool
	var cycleStart NI
	var df func(NI)
	df = func(n NI) {
		switch {
		case temp.Bit(int(n)) == 1:
			cycleFound = true
			cycleStart = n
			return
		case perm.Bit(int(n)) == 1:
			return
		}
		temp.SetBit(int(n), 1)
		for _, nb := range a[n] {
			df(nb.To)
			if cycleFound {
				if cycleStart >= 0 {
					// a little hack: orderng won't be needed so repurpose the
					// slice as cycle.  this is read out in reverse order
					// as the recursion unwinds.
					x := len(ordering) - 1 - len(cycle)
					ordering[x] = n
					cycle = ordering[x:]
					if n == cycleStart {
						cycleStart = -1
					}
				}
				return
			}
		}
		temp.SetBit(int(n), 0)
		perm.SetBit(int(n), 1)
		i--
		ordering[i] = n
	}
	for {
		n := f()
		if n < 0 {
			return ordering[i:], nil
		}
		if perm.Bit(int(n)) == 1 {
			continue
		}
		df(n)
		if cycleFound {
			return nil, cycle
		}
	}
}

// TopologicalKahn computes a topological ordering of a directed acyclic graph.
//
// For an acyclic graph, return value ordering is a permutation of node numbers
// in topologically sorted order and cycle will be nil.  If the graph is found
// to be cyclic, ordering will be nil and cycle will be the path of a found
// cycle.
//
// This function is based on the algorithm by Arthur Kahn and requires the
// transpose of g be passed as the argument.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) TopologicalKahn(tr Directed) (ordering, cycle []NI) {
	// code follows Wikipedia pseudocode.
	var L, S []NI
	// rem for "remaining edges," this function makes a local copy of the
	// in-degrees and consumes that instead of consuming an input.
	rem := make([]int, g.Order())
	for n, fr := range tr.AdjacencyList {
		if len(fr) == 0 {
			// accumulate "set of all nodes with no incoming edges"
			S = append(S, NI(n))
		} else {
			// initialize rem from in-degree
			rem[n] = len(fr)
		}
	}
	for len(S) > 0 {
		last := len(S) - 1 // "remove a node n from S"
		n := S[last]
		S = S[:last]
		L = append(L, n) // "add n to tail of L"
		for _, m := range g.LabeledAdjacencyList[n] {
			// WP pseudo code reads "for each node m..." but it means for each
			// node m *remaining in the graph.*  We consume rem rather than
			// the graph, so "remaining in the graph" for us means rem[m] > 0.
			if rem[m.To] > 0 {
				rem[m.To]--         // "remove edge from the graph"
				if rem[m.To] == 0 { // if "m has no other incoming edges"
					S = append(S, m.To) // "insert m into S"
				}
			}
		}
	}
	// "If graph has edges," for us means a value in rem is > 0.
	for c, in := range rem {
		if in > 0 {
			// recover cyclic nodes
			for _, nb := range g.LabeledAdjacencyList[c] {
				if rem[nb.To] > 0 {
					cycle = append(cycle, NI(c))
					break
				}
			}
		}
	}
	if len(cycle) > 0 {
		return nil, cycle
	}
	return L, nil
}

// TopologicalSubgraph computes a topological ordering of a subgraph of a
// directed acyclic graph.
//
// The subgraph considered is that reachable from the specified node list.
//
// For an acyclic subgraph, return value ordering is a permutation of reachable
// node numbers in topologically sorted order and cycle will be nil.  If the
// subgraph is found to be cyclic, ordering will be nil and cycle will be
// the path of a found cycle.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g LabeledDirected) TopologicalSubgraph(nodes []NI) (ordering, cycle []NI) {
	i := -1
	return g.dfTopo(func() NI {
		i++
		if i < len(nodes) {
			return nodes[i]
		}
		return -1
	})
}

// TransitiveClosure returns the transitive closure of directed graph g.
//
// The algorithm is Warren's, which works most naturally with an adjacency
// matrix representation.  The returned transitive closure is left in this
// adjacency matrix representation.  For a graph g of order n, matrix tc
// is returned as a length n slice of length n bits.Bits values, where
// tc[from].Bit(to) == 1 represents an arc of the transitive closure.
func (g LabeledDirected) TransitiveClosure() []bits.Bits {
	// construct adjacency matrix
	a := g.LabeledAdjacencyList
	t := make([]bits.Bits, len(a))
	for n := range t {
		tn := bits.New(len(a))
		for _, to := range a[n] {
			tn.SetBit(int(to.To), 1)
		}
		t[n] = tn
	}
	// above diagonal
	for i := 1; i < len(a); i++ {
		ti := t[i]
		for k := 0; k < i; k++ {
			if ti.Bit(k) == 1 {
				ti.Or(ti, t[k])
			}
		}
	}
	// below diagonal
	for i, ti := range t[:len(a)-1] {
		for k := i + 1; k < len(a); k++ {
			if ti.Bit(k) == 1 {
				ti.Or(ti, t[k])
			}
		}
	}
	return t
}
