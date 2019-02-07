// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

import (
	"errors"
	"fmt"

	"github.com/soniakeys/bits"
)

// undir_RO.go is code generated from undir_cg.go by directives in graph.go.
// Editing undir_cg.go is okay.  It is the code generation source.
// DO NOT EDIT undir_RO.go.
// The RO means read only and it is upper case RO to slow you down a bit
// in case you start to edit the file.
//-------------------

// Bipartite constructs an object indexing the bipartite structure of a graph.
//
// In a bipartite component, nodes can be partitioned into two sets, or
// "colors," such that every edge in the component goes from one set to the
// other.
//
// If the graph is bipartite, the method constructs and returns a new
// Bipartite object as b and returns ok = true.
//
// If the component is not bipartite, a representative odd cycle as oc and
// returns ok = false.
//
// In the case of a graph with mulitiple connected components, this method
// provides no control over the color orientation by component.  See
// Undirected.BipartiteComponent if this control is needed.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) Bipartite() (b *Bipartite, oc []NI, ok bool) {
	c1 := bits.New(g.Order())
	c2 := bits.New(g.Order())
	r, _, _ := g.ConnectedComponentReps()
	// accumulate n2 number of zero bits in c2 as number of one bits in n1
	var n, n2 int
	for _, r := range r {
		ok, n, _, oc = g.BipartiteComponent(r, c1, c2)
		if !ok {
			return
		}
		n2 += n
	}
	return &Bipartite{g, c2, n2}, nil, true
}

// BipartiteComponent analyzes the bipartite structure of a connected component
// of an undirected graph.
//
// In a bipartite component, nodes can be partitioned into two sets, or
// "colors," such that every edge in the component goes from one set to the
// other.
//
// Argument n can be any representative node of the component to be analyzed.
// Arguments c1 and c2 must be separate bits.Bits objects constructed to be
// of length of the number of nodes of g.  These bitmaps are used in the
// component traversal and the bits of the component must be zero when the
// method is called.
//
// If the component is bipartite, BipartiteComponent populates bitmaps
// c1 and c2 with the two-coloring of the component, always assigning the set
// with representative node n to bitmap c1.  It returns b = true,
// and also returns the number of bits set in c1 and c2 as n1 and n2
// respectively.
//
// If the component is not bipartite, BipartiteComponent returns b = false
// and a representative odd cycle as oc.
//
// See also method Bipartite.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) BipartiteComponent(n NI, c1, c2 bits.Bits) (b bool, n1, n2 int, oc []NI) {
	a := g.AdjacencyList
	b = true
	var open bool
	var df func(n NI, c1, c2 *bits.Bits, n1, n2 *int)
	df = func(n NI, c1, c2 *bits.Bits, n1, n2 *int) {
		c1.SetBit(int(n), 1)
		*n1++
		for _, nb := range a[n] {
			if c1.Bit(int(nb)) == 1 {
				b = false
				oc = []NI{nb, n}
				open = true
				return
			}
			if c2.Bit(int(nb)) == 1 {
				continue
			}
			df(nb, c2, c1, n2, n1)
			if b {
				continue
			}
			switch {
			case !open:
			case n == oc[0]:
				open = false
			default:
				oc = append(oc, n)
			}
			return
		}
	}
	df(n, &c1, &c2, &n1, &n2)
	if b {
		return b, n1, n2, nil
	}
	return b, 0, 0, oc
}

// BronKerbosch1 finds maximal cliques in an undirected graph.
//
// The graph must not contain parallel edges or loops.
//
// See https://en.wikipedia.org/wiki/Clique_(graph_theory) and
// https://en.wikipedia.org/wiki/Bron%E2%80%93Kerbosch_algorithm for background.
//
// This method implements the BronKerbosch1 algorithm of WP; that is,
// the original algorithm without improvements.
//
// The method calls the emit argument for each maximal clique in g, as long
// as emit returns true.  If emit returns false, BronKerbosch1 returns
// immediately.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also more sophisticated variants BronKerbosch2 and BronKerbosch3.
func (g Undirected) BronKerbosch1(emit func(bits.Bits) bool) {
	a := g.AdjacencyList
	var f func(R, P, X bits.Bits) bool
	f = func(R, P, X bits.Bits) bool {
		switch {
		case !P.AllZeros():
			r2 := bits.New(len(a))
			p2 := bits.New(len(a))
			x2 := bits.New(len(a))
			pf := func(n int) bool {
				r2.Set(R)
				r2.SetBit(n, 1)
				p2.ClearAll()
				x2.ClearAll()
				for _, to := range a[n] {
					if P.Bit(int(to)) == 1 {
						p2.SetBit(int(to), 1)
					}
					if X.Bit(int(to)) == 1 {
						x2.SetBit(int(to), 1)
					}
				}
				if !f(r2, p2, x2) {
					return false
				}
				P.SetBit(n, 0)
				X.SetBit(n, 1)
				return true
			}
			if !P.IterateOnes(pf) {
				return false
			}
		case X.AllZeros():
			return emit(R)
		}
		return true
	}
	var R, P, X bits.Bits
	R = bits.New(len(a))
	P = bits.New(len(a))
	X = bits.New(len(a))
	P.SetAll()
	f(R, P, X)
}

// BKPivotMaxDegree is a strategy for BronKerbosch methods.
//
// To use it, take the method value (see golang.org/ref/spec#Method_values)
// and pass it as the argument to BronKerbosch2 or 3.
//
// The strategy is to pick the node from P or X with the maximum degree
// (number of edges) in g.  Note this is a shortcut from evaluating degrees
// in P.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) BKPivotMaxDegree(P, X bits.Bits) (p NI) {
	// choose pivot u as highest degree node from P or X
	a := g.AdjacencyList
	maxDeg := -1
	P.IterateOnes(func(n int) bool { // scan P
		if d := len(a[n]); d > maxDeg {
			p = NI(n)
			maxDeg = d
		}
		return true
	})
	X.IterateOnes(func(n int) bool { // scan X
		if d := len(a[n]); d > maxDeg {
			p = NI(n)
			maxDeg = d
		}
		return true
	})
	return
}

// BKPivotMinP is a strategy for BronKerbosch methods.
//
// To use it, take the method value (see golang.org/ref/spec#Method_values)
// and pass it as the argument to BronKerbosch2 or 3.
//
// The strategy is to simply pick the first node in P.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) BKPivotMinP(P, X bits.Bits) NI {
	return NI(P.OneFrom(0))
}

// BronKerbosch2 finds maximal cliques in an undirected graph.
//
// The graph must not contain parallel edges or loops.
//
// See https://en.wikipedia.org/wiki/Clique_(graph_theory) and
// https://en.wikipedia.org/wiki/Bron%E2%80%93Kerbosch_algorithm for background.
//
// This method implements the BronKerbosch2 algorithm of WP; that is,
// the original algorithm plus pivoting.
//
// The argument is a pivot function that must return a node of P or X.
// P is guaranteed to contain at least one node.  X is not.
// For example see BKPivotMaxDegree.
//
// The method calls the emit argument for each maximal clique in g, as long
// as emit returns true.  If emit returns false, BronKerbosch1 returns
// immediately.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also simpler variant BronKerbosch1 and more sophisticated variant
// BronKerbosch3.
func (g Undirected) BronKerbosch2(pivot func(P, X bits.Bits) NI, emit func(bits.Bits) bool) {
	a := g.AdjacencyList
	var f func(R, P, X bits.Bits) bool
	f = func(R, P, X bits.Bits) bool {
		switch {
		case !P.AllZeros():
			r2 := bits.New(len(a))
			p2 := bits.New(len(a))
			x2 := bits.New(len(a))
			pnu := bits.New(len(a))
			// compute P \ N(u).  next 5 lines are only difference from BK1
			pnu.Set(P)
			for _, to := range a[pivot(P, X)] {
				pnu.SetBit(int(to), 0)
			}
			// remaining code like BK1
			pf := func(n int) bool {
				r2.Set(R)
				r2.SetBit(n, 1)
				p2.ClearAll()
				x2.ClearAll()
				for _, to := range a[n] {
					if P.Bit(int(to)) == 1 {
						p2.SetBit(int(to), 1)
					}
					if X.Bit(int(to)) == 1 {
						x2.SetBit(int(to), 1)
					}
				}
				if !f(r2, p2, x2) {
					return false
				}
				P.SetBit(n, 0)
				X.SetBit(n, 1)
				return true
			}
			if !pnu.IterateOnes(pf) {
				return false
			}
		case X.AllZeros():
			return emit(R)
		}
		return true
	}
	R := bits.New(len(a))
	P := bits.New(len(a))
	X := bits.New(len(a))
	P.SetAll()
	f(R, P, X)
}

// BronKerbosch3 finds maximal cliques in an undirected graph.
//
// The graph must not contain parallel edges or loops.
//
// See https://en.wikipedia.org/wiki/Clique_(graph_theory) and
// https://en.wikipedia.org/wiki/Bron%E2%80%93Kerbosch_algorithm for background.
//
// This method implements the BronKerbosch3 algorithm of WP; that is,
// the original algorithm with pivoting and degeneracy ordering.
//
// The argument is a pivot function that must return a node of P or X.
// P is guaranteed to contain at least one node.  X is not.
// For example see BKPivotMaxDegree.
//
// The method calls the emit argument for each maximal clique in g, as long
// as emit returns true.  If emit returns false, BronKerbosch1 returns
// immediately.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also simpler variants BronKerbosch1 and BronKerbosch2.
func (g Undirected) BronKerbosch3(pivot func(P, X bits.Bits) NI, emit func(bits.Bits) bool) {
	a := g.AdjacencyList
	var f func(R, P, X bits.Bits) bool
	f = func(R, P, X bits.Bits) bool {
		switch {
		case !P.AllZeros():
			r2 := bits.New(len(a))
			p2 := bits.New(len(a))
			x2 := bits.New(len(a))
			pnu := bits.New(len(a))
			// compute P \ N(u).  next lines are only difference from BK1
			pnu.Set(P)
			for _, to := range a[pivot(P, X)] {
				pnu.SetBit(int(to), 0)
			}
			// remaining code like BK2
			pf := func(n int) bool {
				r2.Set(R)
				r2.SetBit(n, 1)
				p2.ClearAll()
				x2.ClearAll()
				for _, to := range a[n] {
					if P.Bit(int(to)) == 1 {
						p2.SetBit(int(to), 1)
					}
					if X.Bit(int(to)) == 1 {
						x2.SetBit(int(to), 1)
					}
				}
				if !f(r2, p2, x2) {
					return false
				}
				P.SetBit(n, 0)
				X.SetBit(n, 1)
				return true
			}
			if !pnu.IterateOnes(pf) {
				return false
			}
		case X.AllZeros():
			return emit(R)
		}
		return true
	}
	R := bits.New(len(a))
	P := bits.New(len(a))
	X := bits.New(len(a))
	P.SetAll()
	// code above same as BK2
	// code below new to BK3
	ord, _ := g.DegeneracyOrdering()
	p2 := bits.New(len(a))
	x2 := bits.New(len(a))
	for _, n := range ord {
		R.SetBit(int(n), 1)
		p2.ClearAll()
		x2.ClearAll()
		for _, to := range a[n] {
			if P.Bit(int(to)) == 1 {
				p2.SetBit(int(to), 1)
			}
			if X.Bit(int(to)) == 1 {
				x2.SetBit(int(to), 1)
			}
		}
		if !f(R, p2, x2) {
			return
		}
		R.SetBit(int(n), 0)
		P.SetBit(int(n), 0)
		X.SetBit(int(n), 1)
	}
}

// ConnectedComponentBits returns a function that iterates over connected
// components of g, returning a member bitmap for each.
//
// Each call of the returned function returns the order, arc size,
// and bits of a connected component.  The underlying bits allocation is
// the same for each call and is overwritten on subsequent calls.  Use or
// save the bits before calling the function again.  The function returns
// zeros after returning all connected components.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also ConnectedComponentInts, ConnectedComponentReps, and
// ConnectedComponentReps.
func (g Undirected) ConnectedComponentBits() func() (order, arcSize int, bits bits.Bits) {
	a := g.AdjacencyList
	vg := bits.New(len(a)) // nodes visited in graph
	vc := bits.New(len(a)) // nodes visited in current component
	var order, arcSize int
	var df func(NI)
	df = func(n NI) {
		vg.SetBit(int(n), 1)
		vc.SetBit(int(n), 1)
		order++
		arcSize += len(a[n])
		for _, nb := range a[n] {
			if vg.Bit(int(nb)) == 0 {
				df(nb)
			}
		}
		return
	}
	var n int
	return func() (o, ma int, b bits.Bits) {
		for ; n < len(a); n++ {
			if vg.Bit(n) == 0 {
				vc.ClearAll()
				order, arcSize = 0, 0
				df(NI(n))
				return order, arcSize, vc
			}
		}
		return // return zeros signalling no more components
	}
}

// ConnectedComponenInts returns a list of component numbers (ints) for each
// node of graph g.
//
// The method assigns numbers to components 1-based, 1 through the number of
// components.  Return value ci contains the component number for each node.
// Return value nc is the number of components.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also ConnectedComponentBits, ConnectedComponentLists, and
// ConnectedComponentReps.
func (g Undirected) ConnectedComponentInts() (ci []int, nc int) {
	a := g.AdjacencyList
	ci = make([]int, len(a))
	var df func(NI)
	df = func(nd NI) {
		ci[nd] = nc
		for _, to := range a[nd] {
			if ci[to] == 0 {
				df(to)
			}
		}
		return
	}
	for nd := range a {
		if ci[nd] == 0 {
			nc++
			df(NI(nd))
		}
	}
	return
}

// ConnectedComponentLists returns a function that iterates over connected
// components of g, returning the member list of each.
//
// Each call of the returned function returns a node list of a connected
// component and the arc size of the component.  The returned function returns
// nil, 0 after returning all connected components.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also ConnectedComponentBits, ConnectedComponentInts, and
// ConnectedComponentReps.
func (g Undirected) ConnectedComponentLists() func() (nodes []NI, arcSize int) {
	a := g.AdjacencyList
	vg := bits.New(len(a)) // nodes visited in graph
	var l []NI             // accumulated node list of current component
	var ma int             // accumulated arc size of current component
	var df func(NI)
	df = func(n NI) {
		vg.SetBit(int(n), 1)
		l = append(l, n)
		ma += len(a[n])
		for _, nb := range a[n] {
			if vg.Bit(int(nb)) == 0 {
				df(nb)
			}
		}
		return
	}
	var n int
	return func() ([]NI, int) {
		for ; n < len(a); n++ {
			if vg.Bit(n) == 0 {
				l, ma = nil, 0
				df(NI(n))
				return l, ma
			}
		}
		return nil, 0
	}
}

// ConnectedComponentReps returns a representative node from each connected
// component of g.
//
// Returned is a slice with a single representative node from each connected
// component and also parallel slices with the orders and arc sizes
// in the corresponding components.
//
// This is fairly minimal information describing connected components.
// From a representative node, other nodes in the component can be reached
// by depth first traversal for example.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also ConnectedComponentBits and ConnectedComponentLists which can
// collect component members in a single traversal, and IsConnected which
// is an even simpler boolean test.
func (g Undirected) ConnectedComponentReps() (reps []NI, orders, arcSizes []int) {
	a := g.AdjacencyList
	c := bits.New(len(a))
	var o, ma int
	var df func(NI)
	df = func(n NI) {
		c.SetBit(int(n), 1)
		o++
		ma += len(a[n])
		for _, nb := range a[n] {
			if c.Bit(int(nb)) == 0 {
				df(nb)
			}
		}
		return
	}
	for n := range a {
		if c.Bit(n) == 0 {
			o, ma = 0, 0
			df(NI(n))
			reps = append(reps, NI(n))
			orders = append(orders, o)
			arcSizes = append(arcSizes, ma)
		}
	}
	return
}

// Copy makes a deep copy of g.
// Copy also computes the arc size ma, the number of arcs.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) Copy() (c Undirected, ma int) {
	l, s := g.AdjacencyList.Copy()
	return Undirected{l}, s
}

// Degeneracy is a measure of dense subgraphs within a graph.
//
// See Wikipedia https://en.wikipedia.org/wiki/Degeneracy_(graph_theory)
//
// See also method DegeneracyOrdering which returns a degeneracy node
// ordering and k-core breaks.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) Degeneracy() (k int) {
	a := g.AdjacencyList
	// WP algorithm, attributed to Matula and Beck.
	L := bits.New(len(a))
	d := make([]int, len(a))
	var D [][]NI
	for v, nb := range a {
		dv := len(nb)
		d[v] = dv
		for len(D) <= dv {
			D = append(D, nil)
		}
		D[dv] = append(D[dv], NI(v))
	}
	for range a {
		// find a non-empty D
		i := 0
		for len(D[i]) == 0 {
			i++
		}
		// k is max(i, k)
		if i > k {
			k = i
		}
		// select from D[i]
		Di := D[i]
		last := len(Di) - 1
		v := Di[last]
		// Add v to ordering, remove from Di
		L.SetBit(int(v), 1)
		D[i] = Di[:last]
		// move neighbors
		for _, nb := range a[v] {
			if L.Bit(int(nb)) == 1 {
				continue
			}
			dn := d[nb]  // old number of neighbors of nb
			Ddn := D[dn] // nb is in this list
			// remove it from the list
			for wx, w := range Ddn {
				if w == nb {
					last := len(Ddn) - 1
					Ddn[wx], Ddn[last] = Ddn[last], Ddn[wx]
					D[dn] = Ddn[:last]
				}
			}
			dn-- // new number of neighbors
			d[nb] = dn
			// re--add it to it's new list
			D[dn] = append(D[dn], nb)
		}
	}
	return
}

// DegeneracyOrdering computes degeneracy node ordering and k-core breaks.
//
// See Wikipedia https://en.wikipedia.org/wiki/Degeneracy_(graph_theory)
//
// In return value ordering, nodes are ordered by their "coreness" as
// defined at https://en.wikipedia.org/wiki/Degeneracy_(graph_theory)#k-Cores.
//
// Return value kbreaks indexes ordering by coreness number.  len(kbreaks)
// will be one more than the graph degeneracy as returned by the Degeneracy
// method.  If degeneracy is d, d = len(kbreaks) - 1, kbreaks[d] is the last
// value in kbreaks and ordering[:kbreaks[d]] contains nodes of the d-cores
// of the graph.  kbreaks[0] is always the number of nodes in g as all nodes
// are in in a 0-core.
//
// Note that definitions of "k-core" differ on whether a k-core must be a
// single connected component.  This method does not resolve individual
// connected components.
//
// See also method Degeneracy which returns just the degeneracy number.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) DegeneracyOrdering() (ordering []NI, kbreaks []int) {
	a := g.AdjacencyList
	// WP algorithm
	k := 0
	ordering = make([]NI, len(a))
	kbreaks = []int{len(a)}
	L := bits.New(len(a))
	d := make([]int, len(a))
	var D [][]NI
	for v, nb := range a {
		dv := len(nb)
		d[v] = dv
		for len(D) <= dv {
			D = append(D, nil)
		}
		D[dv] = append(D[dv], NI(v))
	}
	for ox := len(a) - 1; ox >= 0; ox-- {
		// find a non-empty D
		i := 0
		for len(D[i]) == 0 {
			i++
		}
		// k is max(i, k)
		if i > k {
			for len(kbreaks) <= i {
				kbreaks = append(kbreaks, ox+1)
			}
			k = i
		}
		// select from D[i]
		Di := D[i]
		last := len(Di) - 1
		v := Di[last]
		// Add v to ordering, remove from Di
		ordering[ox] = v
		L.SetBit(int(v), 1)
		D[i] = Di[:last]
		// move neighbors
		for _, nb := range a[v] {
			if L.Bit(int(nb)) == 1 {
				continue
			}
			dn := d[nb]  // old number of neighbors of nb
			Ddn := D[dn] // nb is in this list
			// remove it from the list
			for wx, w := range Ddn {
				if w == nb {
					last := len(Ddn) - 1
					Ddn[wx], Ddn[last] = Ddn[last], Ddn[wx]
					D[dn] = Ddn[:last]
				}
			}
			dn-- // new number of neighbors
			d[nb] = dn
			// re--add it to it's new list
			D[dn] = append(D[dn], nb)
		}
	}
	//for i, j := 0, k; i < j; i, j = i+1, j-1 {
	//	kbreaks[i], kbreaks[j] = kbreaks[j], kbreaks[i]
	//}
	return
}

// Degree for undirected graphs, returns the degree of a node.
//
// The degree of a node in an undirected graph is the number of incident
// edges, where loops count twice.
//
// If g is known to be loop-free, the result is simply equivalent to len(g[n]).
// See handshaking lemma example at AdjacencyList.ArcSize.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) Degree(n NI) int {
	to := g.AdjacencyList[n]
	d := len(to) // just "out" degree,
	for _, to := range to {
		if to == n {
			d++ // except loops count twice
		}
	}
	return d
}

// DegreeCentralization returns the degree centralization metric of a graph.
//
// Degree of a node is one measure of node centrality and is directly
// available from the adjacency list representation.  This allows degree
// centralization for the graph to be very efficiently computed.
//
// The value returned is from 0 to 1 inclusive for simple graphs of three or
// more nodes.  As a special case, 0 is returned for graphs of two or fewer
// nodes.  The value returned can be > 1 for graphs with loops or parallel
// edges.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) DegreeCentralization() float64 {
	a := g.AdjacencyList
	if len(a) <= 2 {
		return 0
	}
	var max, sum int
	for _, to := range a {
		if len(to) > max {
			max = len(to)
		}
		sum += len(to)
	}
	return float64(len(a)*max-sum) / float64((len(a)-1)*(len(a)-2))
}

// Density returns density for a simple graph.
//
// See also Density function.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) Density() float64 {
	return Density(g.Order(), g.Size())
}

// Eulerian scans an undirected graph to determine if it is Eulerian.
//
// If the graph represents an Eulerian cycle, it returns -1, -1, nil.
//
// If the graph does not represent an Eulerian cycle but does represent an
// Eulerian path, it returns the two end nodes of the path, and nil.
//
// Otherwise it returns an error.
//
// See also method EulerianStart, which short-circuits as soon as it finds
// a node that must be a start or end node of an Eulerian path.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) Eulerian() (end1, end2 NI, err error) {
	end1 = -1
	end2 = -1
	for n := range g.AdjacencyList {
		switch {
		case g.Degree(NI(n))%2 == 0:
		case end1 < 0:
			end1 = NI(n)
		case end2 < 0:
			end2 = NI(n)
		default:
			err = errors.New("non-Eulerian")
			return
		}
	}
	return
}

// EulerianCycle finds an Eulerian cycle in an undirected multigraph.
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
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) EulerianCycle() ([]NI, error) {
	c, _ := g.Copy()
	return c.EulerianCycleD(c.Size())
}

// EulerianCycleD finds an Eulerian cycle in an undirected multigraph.
//
// EulerianCycleD is destructive on its receiver g.  See EulerianCycle for
// a non-destructive version.
//
// Parameter m must be the size of the undirected graph -- the
// number of edges.  Use Undirected.Size if the size is unknown.
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
func (g Undirected) EulerianCycleD(m int) ([]NI, error) {
	if g.Order() == 0 {
		return nil, nil
	}
	e := newEulerian(g.AdjacencyList, m)
	e.p[0] = 0
	for e.s >= 0 {
		v := e.top()
		if err := e.pushUndir(); err != nil {
			return nil, err
		}
		if e.top() != v {
			return nil, errors.New("not Eulerian")
		}
		e.keep()
	}
	if !e.uv.AllZeros() {
		return nil, errors.New("not strongly connected")
	}
	return e.p, nil
}

// EulerianPath finds an Eulerian path in an undirected multigraph.
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
func (g Undirected) EulerianPath() ([]NI, error) {
	c, _ := g.Copy()
	start := c.EulerianStart()
	if start < 0 {
		start = 0
	}
	return c.EulerianPathD(c.Size(), start)
}

// EulerianPathD finds an Eulerian path in a undirected multigraph.
//
// EulerianPathD is destructive on its receiver g.  See EulerianPath for
// a non-destructive version.
//
// Argument m must be the correct size, or number of edges in g.
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
func (g Undirected) EulerianPathD(m int, start NI) ([]NI, error) {
	if g.Order() == 0 {
		return nil, nil
	}
	e := newEulerian(g.AdjacencyList, m)
	e.p[0] = start
	// unlike EulerianCycle, the first path doesn't have to be a cycle.
	if err := e.pushUndir(); err != nil {
		return nil, err
	}
	e.keep()
	for e.s >= 0 {
		start = e.top()
		e.push()
		// paths after the first must be cycles though
		// (as long as there are nodes on the stack)
		if e.top() != start {
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
// A graph representing an Eulerian path can have two nodes with odd degree.
// If it does, these must be the end nodes of the path.  EulerianEnd scans
// for a node with an odd degree, returning immediately with the first one
// it finds.
//
// If the scan completes without finding a node with odd degree the method
// returns -1.
//
// See also method Eulerian, which completely validates a graph as representing
// an Eulerian path.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) EulerianStart() NI {
	for n := range g.AdjacencyList {
		if g.Degree(NI(n))%2 != 0 {
			return NI(n)
		}
	}
	return -1
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
func (s *UndirectedSubgraph) AddNode(p NI) (b NI) {
	if int(p) < 0 || int(p) >= s.Super.Order() {
		panic(fmt.Sprint("AddNode: NI ", p, " not in supergraph"))
	}
	if b, ok := s.SubNI[p]; ok {
		return b
	}
	a := s.Undirected.AdjacencyList
	b = NI(len(a))
	s.Undirected.AdjacencyList = append(a, nil)
	s.SuperNI = append(s.SuperNI, p)
	s.SubNI[p] = b
	return
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
func (g *Undirected) InduceList(l []NI) *UndirectedSubgraph {
	sub, sup := mapList(l)
	return &UndirectedSubgraph{
		Super:   g,
		SubNI:   sub,
		SuperNI: sup,
		Undirected: Undirected{
			g.AdjacencyList.induceArcs(sub, sup),
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
func (g *Undirected) InduceBits(t bits.Bits) *UndirectedSubgraph {
	sub, sup := mapBits(t)
	return &UndirectedSubgraph{
		Super:   g,
		SubNI:   sub,
		SuperNI: sup,
		Undirected: Undirected{
			g.AdjacencyList.induceArcs(sub, sup),
		}}
}

// IsConnected tests if an undirected graph is a single connected component.
//
// There are equivalent labeled and unlabeled versions of this method.
//
// See also ConnectedComponentReps for a method returning more information.
func (g Undirected) IsConnected() bool {
	a := g.AdjacencyList
	if len(a) == 0 {
		return true
	}
	b := bits.New(len(a))
	var df func(NI)
	df = func(n NI) {
		b.SetBit(int(n), 1)
		for _, to := range a[n] {
			if b.Bit(int(to)) == 0 {
				df(to)
			}
		}
	}
	df(0)
	return b.AllOnes()
}

// IsTree identifies trees in undirected graphs.
//
// Return value isTree is true if the connected component reachable from root
// is a tree.  Further, return value allTree is true if the entire graph g is
// connected.
//
// There are equivalent labeled and unlabeled versions of this method.
func (g Undirected) IsTree(root NI) (isTree, allTree bool) {
	a := g.AdjacencyList
	v := bits.New(len(a))
	v.SetAll()
	var df func(NI, NI) bool
	df = func(fr, n NI) bool {
		if v.Bit(int(n)) == 0 {
			return false
		}
		v.SetBit(int(n), 0)
		for _, to := range a[n] {
			if to != fr && !df(n, to) {
				return false
			}
		}
		return true
	}
	v.SetBit(int(root), 0)
	for _, to := range a[root] {
		if !df(root, to) {
			return false, false
		}
	}
	return true, v.AllZeros()
}

// Size returns the number of edges in g.
//
// See also ArcSize and AnyLoop.
func (g Undirected) Size() int {
	m2 := 0
	for fr, to := range g.AdjacencyList {
		m2 += len(to)
		for _, to := range to {
			if to == NI(fr) {
				m2++
			}
		}
	}
	return m2 / 2
}

// Density returns edge density of a bipartite graph.
//
// Edge density is number of edges over maximum possible number of edges.
// Maximum possible number of edges in a bipartite graph is number of
// nodes of one color times number of nodes of the other color.
func (g Bipartite) Density() float64 {
	a := g.Undirected.AdjacencyList
	s := 0
	g.Color.IterateOnes(func(n int) bool {
		s += len(a[n])
		return true
	})
	return float64(s) / float64(g.N0*(len(a)-g.N0))
}

// PermuteBiadjacency permutes a bipartite graph in place so that a prefix
// of the adjacency list encodes a biadjacency matrix.
//
// The permutation applied is returned.  This would be helpful in referencing
// any externally stored node information.
//
// The biadjacency matrix is encoded as the prefix AdjacencyList[:g.N0].
// Note though that this slice does not represent a valid complete
// AdjacencyList.  BoundsOk would return false, for example.
//
// In adjacency list terms, the result of the permutation is that nodes of
// the prefix only have arcs to the suffix and nodes of the suffix only have
// arcs to the prefix.
func (g Bipartite) PermuteBiadjacency() []int {
	p := make([]int, g.Order())
	i := 0
	g.Color.IterateZeros(func(n int) bool {
		p[n] = i
		i++
		return true
	})
	g.Color.IterateOnes(func(n int) bool {
		p[n] = i
		i++
		return true
	})
	g.Permute(p)
	g.Color.ClearAll()
	for i := g.N0; i < g.Order(); i++ {
		g.Color.SetBit(i, 1)
	}
	return p
}
