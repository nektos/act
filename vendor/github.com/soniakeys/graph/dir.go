// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

import (
	"math"
)

// dir.go has methods specific to directed graphs, types Directed and
// LabeledDirected, also Dominators.
//
// Methods on Directed are first, with exported methods alphabetized.
// Dominators type and methods are at the end.
//----------------------------

// Cycles emits all elementary cycles in a directed graph.
//
// The algorithm here is Johnson's.  See also the equivalent but generally
// slower alt.TarjanCycles.
func (g Directed) Cycles(emit func([]NI) bool) {
	// Johnsons "Finding all the elementary circuits of a directed graph",
	// SIAM J. Comput. Vol. 4, No. 1, March 1975.
	a := g.AdjacencyList
	k := make(AdjacencyList, len(a))
	B := make([]map[NI]bool, len(a))
	blocked := make([]bool, len(a))
	for i := range a {
		blocked[i] = true
		B[i] = map[NI]bool{}
	}
	var s NI
	var stack []NI
	var unblock func(NI)
	unblock = func(u NI) {
		blocked[u] = false
		for w := range B[u] {
			delete(B[u], w)
			if blocked[w] {
				unblock(w)
			}
		}
	}
	var circuit func(NI) (bool, bool)
	circuit = func(v NI) (found, ok bool) {
		f := false
		stack = append(stack, v)
		blocked[v] = true
		for _, w := range k[v] {
			if w == s {
				if !emit(stack) {
					return
				}
				f = true
			} else if !blocked[w] {
				switch found, ok = circuit(w); {
				case !ok:
					return
				case found:
					f = true
				}
			}
		}
		if f {
			unblock(v)
		} else {
			for _, w := range k[v] {
				B[w][v] = true
			}
		}
		stack = stack[:len(stack)-1]
		return f, true
	}
	for s = 0; int(s) < len(a); s++ {
		// (so there's a little extra n^2 component introduced here that
		// comes from not making a proper subgraph but just removing arcs
		// and leaving isolated nodes.  Iterating over the isolated nodes
		// should be very fast though.  It seems like it would be a net win
		// over creating a subgraph.)
		// shallow subgraph
		for z := NI(0); z < s; z++ {
			k[z] = nil
		}
		for z := int(s); z < len(a); z++ {
			k[z] = a[z]
		}
		// find scc in k with s
		var scc []NI
		Directed{k}.StronglyConnectedComponents(func(c []NI) bool {
			for _, n := range c {
				if n == s { // this is it
					scc = c
					return false // stop scc search
				}
			}
			return true // keep looking
		})
		// clear k
		for n := range k {
			k[n] = nil
		}
		// map component
		for _, n := range scc {
			blocked[n] = false
		}
		// copy component to k
		for _, fr := range scc {
			var kt []NI
			for _, to := range a[fr] {
				if !blocked[to] {
					kt = append(kt, to)
				}
			}
			k[fr] = kt
		}
		if _, ok := circuit(s); !ok {
			return
		}
		// reblock component
		for _, n := range scc {
			blocked[n] = true
		}
	}
}

// DAGMaxLenPath finds a maximum length path in a directed acyclic graph.
//
// Argument ordering must be a topological ordering of g.
func (g Directed) DAGMaxLenPath(ordering []NI) (path []NI) {
	// dynamic programming. visit nodes in reverse order. for each, compute
	// longest path as one plus longest of 'to' nodes.
	// Visits each arc once.  O(m).
	//
	// Similar code in label.go
	var n NI
	mlp := make([][]NI, g.Order()) // index by node number
	for i := len(ordering) - 1; i >= 0; i-- {
		fr := ordering[i] // node number
		to := g.AdjacencyList[fr]
		if len(to) == 0 {
			continue
		}
		mt := to[0]
		for _, to := range to[1:] {
			if len(mlp[to]) > len(mlp[mt]) {
				mt = to
			}
		}
		p := append([]NI{mt}, mlp[mt]...)
		mlp[fr] = p
		if len(p) > len(path) {
			n = fr
			path = p
		}
	}
	return append([]NI{n}, path...)
}

// FromList creates a spanning forest of a graph.
//
// The method populates the From members in f.Paths and returns the FromList.
// Also returned is a bool, true if the receiver is found to be a simple graph
// representing a tree or forest.  Loops, or any case of multiple arcs going to
// a node will cause simpleForest to be false.
//
// The FromList return value f will always be a spanning forest of the entire
// graph.  The bool return value simpleForest tells if the receiver graph g
// was a simple forest to begin with.
//
// Other members of the FromList are left as zero values.
// Use FromList.RecalcLen and FromList.RecalcLeaves as needed.
func (g Directed) FromList() (f *FromList, simpleForest bool) {
	paths := make([]PathEnd, g.Order())
	for i := range paths {
		paths[i].From = -1
	}
	simpleForest = true
	for fr, to := range g.AdjacencyList {
		for _, to := range to {
			if int(to) == fr || paths[to].From >= 0 {
				simpleForest = false
			} else {
				paths[to].From = NI(fr)
			}
		}
	}
	return &FromList{Paths: paths}, simpleForest
}

// SpanTree builds a tree spanning nodes reachable from the given root.
//
// The component is spanned by breadth-first search from root.
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
// reachable from root, and a bool indicating if these nodes were found to be
// a simply connected tree in the receiver graph g.  Any cycles, loops,
// or parallel arcs in the component will cause simpleTree to be false, but
// FromList f will still be populated with a valid spanning tree.
func (g Directed) SpanTree(root NI, f *FromList) (nSpanned int, simpleTree bool) {
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
	p[root].Len = 1
	type arc struct {
		from, to NI
	}
	var next []arc
	frontier := []arc{{-1, root}}
	for len(frontier) > 0 {
		for _, fa := range frontier { // fa frontier arc
			nSpanned++
			l := p[fa.to].Len + 1
			for _, to := range a[fa.to] {
				if p[to].Len > 0 {
					simpleTree = false
					continue
				}
				p[to] = PathEnd{From: fa.to, Len: l}
				if l > f.MaxLen {
					f.MaxLen = l
				}
				next = append(next, arc{fa.to, to})
			}
		}
		frontier, next = next, frontier[:0]
	}
	return
}

// Undirected returns copy of g augmented as needed to make it undirected.
func (g Directed) Undirected() Undirected {
	c, _ := g.AdjacencyList.Copy()       // start with a copy
	rw := make(AdjacencyList, g.Order()) // "reciprocals wanted"
	for fr, to := range g.AdjacencyList {
	arc: // for each arc in g
		for _, to := range to {
			if to == NI(fr) {
				continue // loop
			}
			// search wanted arcs
			wf := rw[fr]
			for i, w := range wf {
				if w == to { // found, remove
					last := len(wf) - 1
					wf[i] = wf[last]
					rw[fr] = wf[:last]
					continue arc
				}
			}
			// arc not found, add to reciprocal to wanted list
			rw[to] = append(rw[to], NI(fr))
		}
	}
	// add missing reciprocals
	for fr, to := range rw {
		c[fr] = append(c[fr], to...)
	}
	return Undirected{c}
}

// Transpose constructs a new adjacency list with all arcs reversed.
//
// For every arc from->to of g, the result will have an arc to->from.
// Transpose also counts arcs as it traverses and returns ma the number of arcs
// in g (equal to the number of arcs in the result.)
func (g Directed) Transpose() (t Directed, ma int) {
	ta := make(AdjacencyList, g.Order())
	for n, nbs := range g.AdjacencyList {
		for _, nb := range nbs {
			ta[nb] = append(ta[nb], NI(n))
			ma++
		}
	}
	return Directed{ta}, ma
}

// Cycles emits all elementary cycles in a directed graph.
//
// The algorithm here is Johnson's.  See also the equivalent but generally
// slower alt.TarjanCycles.
func (g LabeledDirected) Cycles(emit func([]Half) bool) {
	a := g.LabeledAdjacencyList
	k := make(LabeledAdjacencyList, len(a))
	B := make([]map[NI]bool, len(a))
	blocked := make([]bool, len(a))
	for i := range a {
		blocked[i] = true
		B[i] = map[NI]bool{}
	}
	var s NI
	var stack []Half
	var unblock func(NI)
	unblock = func(u NI) {
		blocked[u] = false
		for w := range B[u] {
			delete(B[u], w)
			if blocked[w] {
				unblock(w)
			}
		}
	}
	var circuit func(NI) (bool, bool)
	circuit = func(v NI) (found, ok bool) {
		f := false
		blocked[v] = true
		for _, w := range k[v] {
			if w.To == s {
				if !emit(append(stack, w)) {
					return
				}
				f = true
			} else if !blocked[w.To] {
				stack = append(stack, w)
				switch found, ok = circuit(w.To); {
				case !ok:
					return
				case found:
					f = true
				}
				stack = stack[:len(stack)-1]
			}
		}
		if f {
			unblock(v)
		} else {
			for _, w := range k[v] {
				B[w.To][v] = true
			}
		}
		return f, true
	}
	for s = 0; int(s) < len(a); s++ {
		for z := NI(0); z < s; z++ {
			k[z] = nil
		}
		for z := int(s); z < len(a); z++ {
			k[z] = a[z]
		}
		var scc []NI
		LabeledDirected{k}.StronglyConnectedComponents(func(c []NI) bool {
			for _, n := range c {
				if n == s {
					scc = c
					return false
				}
			}
			return true
		})
		for n := range k {
			k[n] = nil
		}
		for _, n := range scc {
			blocked[n] = false
		}
		for _, fr := range scc {
			var kt []Half
			for _, to := range a[fr] {
				if !blocked[to.To] {
					kt = append(kt, to)
				}
			}
			k[fr] = kt
		}
		if _, ok := circuit(s); !ok {
			return
		}
		for _, n := range scc {
			blocked[n] = true
		}
	}
}

// DAGMaxLenPath finds a maximum length path in a directed acyclic graph.
//
// Length here means number of nodes or arcs, not a sum of arc weights.
//
// Argument ordering must be a topological ordering of g.
//
// Returned is a node beginning a maximum length path, and a path of arcs
// starting from that node.
func (g LabeledDirected) DAGMaxLenPath(ordering []NI) (n NI, path []Half) {
	// dynamic programming. visit nodes in reverse order. for each, compute
	// longest path as one plus longest of 'to' nodes.
	// Visits each arc once.  Time complexity O(m).
	//
	// Similar code in dir.go.
	mlp := make([][]Half, g.Order()) // index by node number
	for i := len(ordering) - 1; i >= 0; i-- {
		fr := ordering[i] // node number
		to := g.LabeledAdjacencyList[fr]
		if len(to) == 0 {
			continue
		}
		mt := to[0]
		for _, to := range to[1:] {
			if len(mlp[to.To]) > len(mlp[mt.To]) {
				mt = to
			}
		}
		p := append([]Half{mt}, mlp[mt.To]...)
		mlp[fr] = p
		if len(p) > len(path) {
			n = fr
			path = p
		}
	}
	return
}

// FromList creates a spanning forest of a graph.
//
// The method populates the From members in f.Paths and returns the FromList.
// Also returned is a list of labels corresponding to the from arcs, and a
// bool, true if the receiver is found to be a simple graph representing
// a tree or forest.  Loops, or any case of multiple arcs going to a node
// will cause simpleForest to be false.
//
// The FromList return value f will always be a spanning forest of the entire
// graph.  The bool return value simpleForest tells if the receiver graph g
// was a simple forest to begin with.
//
// Other members of the FromList are left as zero values.
// Use FromList.RecalcLen and FromList.RecalcLeaves as needed.
func (g LabeledDirected) FromList() (f *FromList, labels []LI, simpleForest bool) {
	labels = make([]LI, g.Order())
	paths := make([]PathEnd, g.Order())
	for i := range paths {
		paths[i].From = -1
	}
	simpleForest = true
	for fr, to := range g.LabeledAdjacencyList {
		for _, to := range to {
			if int(to.To) == fr || paths[to.To].From >= 0 {
				simpleForest = false
			} else {
				paths[to.To].From = NI(fr)
				labels[to.To] = to.Label
			}
		}
	}
	return &FromList{Paths: paths}, labels, simpleForest
}

// NegativeCycles emits all cycles with negative cycle distance.
//
// The emit function is called for each cycle found.  Emit must return true
// to continue cycle enumeration.  If emit returns false, NegativeCycles
// stops and returns immediately.
//
// The method mutates receiver g while it runs.  Access to g before
// NegativeCycles returns, such as during the emit callback, will find
// g altered.  G is completely restored when NegativeCycles returns however,
// even if terminated early with a false return from emit.
//
// If mutations on g are unacceptable, use g.Copy and run NegativeCycles on
// the copy.
//
// See also:
//
// * NegativeCycle, which finds a single example of a negative cycle if
// one exists.
//
// * HasNegativeCycle, which detects if a negative cycle exists.
//
// * BellmanFord, which also detects negative cycles.
//
// * Cycles, from which negative cycles can be filtered.
//
// * alt.NegativeCycles, which uses less memory but is generally slower.
func (g LabeledDirected) NegativeCycles(w WeightFunc, emit func([]Half) bool) {
	// Implementation of "Finding all the negative cycles in a directed graph"
	// by Takeo Yamada and Harunobu Kinoshita, Discrete Applied Mathematics
	// 118 (2002) 279–291.
	newNegCyc(g, w, emit).all_nc(LabeledPath{})
}

type negCyc struct {
	g      LabeledDirected
	w      WeightFunc
	emit   func([]Half) bool
	a      LabeledAdjacencyList
	tr     AdjacencyList
	d0, d1 []float64
	dc     []float64
	bt     [][]fromHalf
	btLast []int
}

func newNegCyc(g LabeledDirected, w WeightFunc, emit func([]Half) bool) *negCyc {
	nc := &negCyc{g: g, w: w, emit: emit}
	nc.a = g.LabeledAdjacencyList
	// transpose to make it easier to find from-arcs.
	lt, _ := g.UnlabeledTranspose()
	nc.tr = lt.AdjacencyList
	nc.d0 = make([]float64, len(nc.a))
	nc.d1 = make([]float64, len(nc.a))
	nc.dc = make([]float64, len(nc.a))
	nc.btLast = make([]int, len(nc.a))
	nc.bt = make([][]fromHalf, len(nc.a))
	for i := range nc.bt {
		nc.bt[i] = make([]fromHalf, len(nc.a))
	}
	return nc
}

func (nc *negCyc) all_nc(F LabeledPath) bool {
	var C []Half
	var R LabeledPath
	// Step 1
	if len(F.Path) != 0 {
		return nc.step2(F)
	}
	C = nc.g.NegativeCycle(nc.w)
	if len(C) == 0 {
		return true
	}
	// prep step 4 with no F:
	F.Start = C[len(C)-1].To
	R = LabeledPath{F.Start, C}
	return nc.step4(C, F, R)
}

func (nc *negCyc) step2(F LabeledPath) bool {
	fEnd := F.Path[len(F.Path)-1].To
	wF := F.Distance(nc.w)
	dL, πL := nc.zL(F, fEnd, wF)
	if !(dL < 0) {
		return true
	}
	if len(πL) > 0 {
		C := append(F.Path, πL...)
		R := LabeledPath{fEnd, πL}
		return nc.step4(C, F, R)
	}
	return nc.step3(F, fEnd, wF)
}

func (nc *negCyc) step3(F LabeledPath, fEnd NI, wF float64) bool {
	πΓ := nc.zΓ(F, wF)
	if len(πΓ) > 0 {
		// prep for step 4
		C := append(F.Path, πΓ...)
		R := LabeledPath{fEnd, πΓ}
		return nc.step4(C, F, R)
	}
	return nc.step5(F, fEnd)
}

func (nc *negCyc) step4(C []Half, F, R LabeledPath) (ok bool) {
	// C is a new cycle.
	// F is fixed path to be extended and is a prefix of C.
	// R is the remainder of C
	if ok = nc.emit(C); !ok {
		return
	}
	// for each arc in R, if not the first arc,
	// extend F by the arc of the previous iteration.
	// remove arc from g,
	// Then make the recursive call, then put the arc back in g.
	//
	// after loop, replace arcs from the two stacks.
	type frto struct {
		fr NI
		to []Half
	}
	var frStack [][]arc
	var toStack []frto
	var fr0 NI
	var to0 Half
	for i, h := range R.Path {
		if i > 0 {
			// extend F by arc {fr0 to0}, the arc of the previous iteration.
			// Remove arcs to to0.To and save on stack.
			// Remove arcs from arc0.fr and save on stack.
			F.Path = append(F.Path, to0)
			frStack = append(frStack, nc.cutTo(to0.To))
			toStack = append(toStack, frto{fr0, nc.a[fr0]})
			nc.a[fr0] = nil
		}
		toList := nc.a[R.Start]
		for j, to := range toList {
			if to == h {
				last := len(toList) - 1
				toList[j], toList[last] = toList[last], toList[j]
				nc.a[R.Start] = toList[:last]
				ok = nc.all_nc(F) // return value
				toList[last], toList[j] = toList[j], toList[last]
				nc.a[R.Start] = toList
				break
			}
		}
		if !ok {
			break
		}
		fr0 = R.Start
		to0 = h
		R.Start = h.To
	}
	for i := len(frStack) - 1; i >= 0; i-- {
		nc.a[toStack[i].fr] = toStack[i].to
		nc.restore(frStack[i])
	}
	return
}

func (nc *negCyc) step5(F LabeledPath, fEnd NI) (ok bool) {
	// Step 5 (uncertain case)
	//
	// For each arc from end of F, search each case of extending
	// F by that arc.
	//
	// before loop: save arcs from current path end,
	// replace them with room for a single arc.
	// extend F by room for one more arc,
	ok = true
	save := nc.a[fEnd]
	nc.a[fEnd] = []Half{{}}
	last := len(F.Path)
	F.Path = append(F.Path, Half{})
	for _, h := range save {
		// in each iteration, set the final arc in F, and the single
		// outgoing arc, and save and clear all inbound arcs to the
		// new end node.  make recursive call, then restore saved
		// inbound arcs for the node.
		F.Path[last] = h
		nc.a[fEnd][0] = h
		save := nc.cutTo(h.To)
		ok = nc.all_nc(F)
		nc.restore(save)
		if !ok {
			break
		}
	}
	// after loop, restore saved outgoing arcs in g.
	nc.a[fEnd] = save
	return
}

type arc struct {
	n NI  // node that had an arc cut from its toList
	x int // index of arc that was swapped to the end of the list
}

// modify a cutting all arcs to node n.  return list of cut arcs than
// can be processed in reverse order to restore changes to a
func (nc *negCyc) cutTo(n NI) (c []arc) {
	for _, fr := range nc.tr[n] {
		toList := nc.a[fr]
		for x := 0; x < len(toList); {
			to := toList[x]
			if to.To == n {
				c = append(c, arc{fr, x})
				last := len(toList) - 1
				toList[x], toList[last] = toList[last], toList[x]
				toList = toList[:last]
			} else {
				x++
			}
		}
		nc.a[fr] = toList
	}
	return
}

func (nc *negCyc) restore(c []arc) {
	for i := len(c) - 1; i >= 0; i-- {
		r := c[i]
		toList := nc.a[r.n]
		last := len(toList)
		toList = toList[:last+1]
		toList[r.x], toList[last] = toList[last], toList[r.x]
		nc.a[r.n] = toList
	}
}

func (nc *negCyc) zL(F LabeledPath, fEnd NI, wp float64) (float64, []Half) {
	π, c, d := nc.πj(len(nc.a)-len(F.Path), F.Start, fEnd)
	if c < 0 {
		return d + wp, π
	}
	j := len(nc.a) - len(F.Path) - 1
	// G1: cut arcs going to c
	saveFr := nc.cutTo(c)
	for k := 1; k <= j; k++ {
		nc.dc[k] = nc.dj(k, F.Start, c)
	}
	// G0: also cut arcs coming from c
	saveTo := nc.a[c]
	nc.a[c] = nil
	min := nc.dj(j, F.Start, fEnd)
	// G2: restore arcs going to c
	nc.restore(saveFr)
	for k := 1; k <= j; k++ {
		d1 := nc.dc[k] + nc.dj(j+1-k, c, fEnd)
		if d1 < min {
			min = d1
		}
	}
	nc.a[c] = saveTo
	return min + wp, nil
}

func (nc *negCyc) dj(j int, v, v0 NI) float64 {
	for i := range nc.d0 {
		nc.d0[i] = math.Inf(1)
	}
	nc.d0[v0] = 0
	for ; j > 0; j-- {
		for i, d := range nc.d0 {
			nc.d1[i] = d
		}
		for vʹ, d0vʹ := range nc.d0 {
			if d0vʹ < math.Inf(1) {
				for _, to := range nc.a[vʹ] {
					if sum := d0vʹ + nc.w(to.Label); sum < nc.d1[to.To] {
						nc.d1[to.To] = sum
					}
				}
			}
		}
		nc.d0, nc.d1 = nc.d1, nc.d0
	}
	return nc.d0[v]
}

func (nc *negCyc) πj(j int, v, v0 NI) ([]Half, NI, float64) {
	for i := range nc.d0 {
		nc.d0[i] = math.Inf(1)
		nc.btLast[i] = -1
	}
	nc.d0[v0] = 0
	for k := 0; k < j; k++ {
		for i, d := range nc.d0 {
			nc.d1[i] = d
		}
		btk := nc.bt[k]
		for vʹ, d0vʹ := range nc.d0 {
			if d0vʹ < math.Inf(1) {
				for _, to := range nc.a[vʹ] {
					if sum := d0vʹ + nc.w(to.Label); sum < nc.d1[to.To] {
						nc.d1[to.To] = sum
						btk[to.To] = fromHalf{NI(vʹ), to.Label}
						nc.btLast[to.To] = k
					}
				}
			}
		}
		nc.d0, nc.d1 = nc.d1, nc.d0
	}
	p := make([]Half, nc.btLast[v]+1)
	m := map[NI]bool{}
	c := NI(-1)
	to := v
	for k := nc.btLast[v]; k >= 0; k-- {
		fh := nc.bt[k][to]
		p[k] = Half{to, fh.Label}
		to = fh.From
		if c < 0 {
			if m[to] {
				c = to
			} else {
				m[to] = true
			}
		}
	}
	return p, c, nc.d0[v]
}

func (nc *negCyc) zΓ(F LabeledPath, wp float64) []Half {
	p, d := nc.a.DijkstraPath(F.Path[len(F.Path)-1].To, F.Start, nc.w)
	if !(wp+d < 0) {
		return nil
	}
	return p.Path
}

// SpanTree builds a tree spanning nodes reachable from the given root.
//
// The component is spanned by breadth-first search from root.
// The resulting spanning tree in stored a FromList, and arc labels optionally
// stored in a slice.
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
// The labels slice will be populated only if it is same length as g.
// Nil can be passed for example if labels are not needed.
//
// Returned is the number of nodes spanned, which will be the number of nodes
// reachable from root, and a bool indicating if these nodes were found to be
// a simply connected tree in the receiver graph g.  Any cycles, loops,
// or parallel arcs in the component will cause simpleTree to be false, but
// FromList f will still be populated with a valid spanning tree.
func (g LabeledDirected) SpanTree(root NI, f *FromList, labels []LI) (nSpanned int, simpleTree bool) {
	a := g.LabeledAdjacencyList
	p := f.Paths
	if len(p) != len(a) {
		p = make([]PathEnd, len(a))
		for i := range p {
			p[i].From = -1
		}
		f.Paths = p
	}
	simpleTree = true
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
				if p[to.To].Len > 0 {
					simpleTree = false
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

// Transpose constructs a new adjacency list that is the transpose of g.
//
// For every arc from->to of g, the result will have an arc to->from.
// Transpose also counts arcs as it traverses and returns ma the number of
// arcs in g (equal to the number of arcs in the result.)
func (g LabeledDirected) Transpose() (t LabeledDirected, ma int) {
	ta := make(LabeledAdjacencyList, g.Order())
	for n, nbs := range g.LabeledAdjacencyList {
		for _, nb := range nbs {
			ta[nb.To] = append(ta[nb.To], Half{To: NI(n), Label: nb.Label})
			ma++
		}
	}
	return LabeledDirected{ta}, ma
}

// Undirected returns a new undirected graph derived from g, augmented as
// needed to make it undirected, with reciprocal arcs having matching labels.
func (g LabeledDirected) Undirected() LabeledUndirected {
	c, _ := g.LabeledAdjacencyList.Copy() // start with a copy
	// "reciprocals wanted"
	rw := make(LabeledAdjacencyList, g.Order())
	for fr, to := range g.LabeledAdjacencyList {
	arc: // for each arc in g
		for _, to := range to {
			if to.To == NI(fr) {
				continue // arc is a loop
			}
			// search wanted arcs
			wf := rw[fr]
			for i, w := range wf {
				if w == to { // found, remove
					last := len(wf) - 1
					wf[i] = wf[last]
					rw[fr] = wf[:last]
					continue arc
				}
			}
			// arc not found, add to reciprocal to wanted list
			rw[to.To] = append(rw[to.To], Half{To: NI(fr), Label: to.Label})
		}
	}
	// add missing reciprocals
	for fr, to := range rw {
		c[fr] = append(c[fr], to...)
	}
	return LabeledUndirected{c}
}

// Unlabeled constructs the unlabeled directed graph corresponding to g.
func (g LabeledDirected) Unlabeled() Directed {
	return Directed{g.LabeledAdjacencyList.Unlabeled()}
}

// UnlabeledTranspose constructs a new adjacency list that is the unlabeled
// transpose of g.
//
// For every arc from->to of g, the result will have an arc to->from.
// Transpose also counts arcs as it traverses and returns ma, the number of
// arcs in g (equal to the number of arcs in the result.)
//
// It is equivalent to g.Unlabeled().Transpose() but constructs the result
// directly.
func (g LabeledDirected) UnlabeledTranspose() (t Directed, ma int) {
	ta := make(AdjacencyList, g.Order())
	for n, nbs := range g.LabeledAdjacencyList {
		for _, nb := range nbs {
			ta[nb.To] = append(ta[nb.To], NI(n))
			ma++
		}
	}
	return Directed{ta}, ma
}

// DominanceFrontiers holds dominance frontiers for all nodes in some graph.
// The frontier for a given node is a set of nodes, represented here as a map.
type DominanceFrontiers []map[NI]struct{}

// Frontier computes the dominance frontier for a node set.
func (d DominanceFrontiers) Frontier(s map[NI]struct{}) map[NI]struct{} {
	fs := map[NI]struct{}{}
	for n := range s {
		for f := range d[n] {
			fs[f] = struct{}{}
		}
	}
	return fs
}

// Closure computes the closure, or iterated dominance frontier for a node set.
func (d DominanceFrontiers) Closure(s map[NI]struct{}) map[NI]struct{} {
	c := map[NI]struct{}{}
	e := map[NI]struct{}{}
	w := map[NI]struct{}{}
	var n NI
	for n = range s {
		e[n] = struct{}{}
		w[n] = struct{}{}
	}
	for len(w) > 0 {
		for n = range w {
			break
		}
		delete(w, n)
		for f := range d[n] {
			if _, ok := c[f]; !ok {
				c[f] = struct{}{}
				if _, ok := e[f]; !ok {
					e[f] = struct{}{}
					w[f] = struct{}{}
				}
			}
		}
	}
	return c
}

// Dominators holds immediate dominators.
//
// Dominators is a return type from methods Dominators, PostDominators, and
// Doms.  See those methods for construction examples.
//
// The list of immediate dominators represents the "dominator tree"
// (in the same way a FromList represents a tree, but somewhat lighter weight.)
//
// In addition to the exported immediate dominators, the type also retains
// the transpose graph that was used to compute the dominators.
// See PostDominators and Doms for a caution about modifying the transpose
// graph.
type Dominators struct {
	Immediate []NI
	from      interface { // either Directed or LabeledDirected
		domFrontiers(Dominators) DominanceFrontiers
	}
}

// Frontiers constructs the dominator frontier for each node.
//
// The frontier for a node is a set of nodes, represented as a map.  The
// returned slice has the length of d.Immediate, which is the length of
// the original graph.  The frontier is valid however only for nodes of the
// reachable subgraph.  Nodes not in the reachable subgraph, those with a
// d.Immediate value of -1, will have a nil map.
func (d Dominators) Frontiers() DominanceFrontiers {
	return d.from.domFrontiers(d)
}

// Set constructs the dominator set for a given node.
//
// The dominator set for a node always includes the node itself as the first
// node in the returned slice, as long as the node was in the subgraph
// reachable from the start node used to construct the dominators.
// If the argument n is a node not in the subgraph, Set returns nil.
func (d Dominators) Set(n NI) []NI {
	im := d.Immediate
	if im[n] < 0 {
		return nil
	}
	for s := []NI{n}; ; {
		if p := im[n]; p < 0 || p == n {
			return s
		} else {
			s = append(s, p)
			n = p
		}
	}
}

// starting at the node on the top of the stack, follow arcs until stuck.
// mark nodes visited, push nodes on stack, remove arcs from g.
func (e *eulerian) push() {
	for u := e.top(); ; {
		e.uv.SetBit(int(u), 0) // reset unvisited bit
		arcs := e.g[u]
		if len(arcs) == 0 {
			return // stuck
		}
		w := arcs[0] // follow first arc
		e.s++        // push followed node on stack
		e.p[e.s] = w
		e.g[u] = arcs[1:] // consume arc
		u = w
	}
}

func (e *labEulerian) push() {
	for u := e.top().To; ; {
		e.uv.SetBit(int(u), 0) // reset unvisited bit
		arcs := e.g[u]
		if len(arcs) == 0 {
			return // stuck
		}
		w := arcs[0] // follow first arc
		e.s++        // push followed node on stack
		e.p[e.s] = w
		e.g[u] = arcs[1:] // consume arc
		u = w.To
	}
}
