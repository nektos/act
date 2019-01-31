// Copyright 2016 Sonia Keys
// License MIT: https://opensource.org/licenses/MIT

package graph

import (
	"errors"
	"math"
	"math/rand"

	"github.com/soniakeys/bits"
)

// ChungLu constructs a random simple undirected graph.
//
// The Chung Lu model is similar to a "configuration model" where each
// node has a specified degree.  In the Chung Lu model the degree specified
// for each node is taken as an expected degree, not an exact degree.
//
// Argument w is "weight," the expected degree for each node.
// The values of w must be given in decreasing order.
//
// The constructed graph will have node 0 with expected degree w[0] and so on
// so degree will decrease with node number.  To randomize degree across
// node numbers, consider using the Permute method with a rand.Perm.
//
// Also returned is the actual size m of constructed graph g.
//
// If Rand r is nil, the rand package default shared source is used.
func ChungLu(w []float64, rr *rand.Rand) (g Undirected, m int) {
	// Ref: "Efficient Generation of Networks with Given Expected Degrees"
	// Joel C. Miller and Aric Hagberg
	// accessed at http://aric.hagberg.org/papers/miller-2011-efficient.pdf
	rf := rand.Float64
	if rr != nil {
		rf = rr.Float64
	}
	a := make(AdjacencyList, len(w))
	S := 0.
	for i := len(w) - 1; i >= 0; i-- {
		S += w[i]
	}
	for u := 0; u < len(w)-1; u++ {
		v := u + 1
		p := w[u] * w[v] / S
		if p > 1 {
			p = 1
		}
		for v < len(w) && p > 0 {
			if p != 1 {
				v += int(math.Log(rf()) / math.Log(1-p))
			}
			if v < len(w) {
				q := w[u] * w[v] / S
				if q > 1 {
					q = 1
				}
				if rf() < q/p {
					a[u] = append(a[u], NI(v))
					a[v] = append(a[v], NI(u))
					m++
				}
				p = q
				v++
			}
		}
	}
	return Undirected{a}, m
}

// Euclidean generates a random simple graph on the Euclidean plane.
//
// Nodes are associated with coordinates uniformly distributed on a unit
// square.  Arcs are added between random nodes with a bias toward connecting
// nearer nodes.
//
// Unfortunately the function has a few "knobs".
// The returned graph will have order nNodes and arc size nArcs.  The affinity
// argument controls the bias toward connecting nearer nodes.  The function
// selects random pairs of nodes as a candidate arc then rejects the candidate
// if the nodes fail an affinity test.  Also parallel arcs are rejected.
// As more affine or denser graphs are requested, rejections increase,
// increasing run time.  The patience argument controls the number of arc
// rejections allowed before the function gives up and returns an error.
// Note that higher affinity will require more patience and that some
// combinations of nNodes and nArcs cannot be achieved with any amount of
// patience given that the returned graph must be simple.
//
// If Rand r is nil, the rand package default shared source is used.
//
// Returned is a directed simple graph and associated positions indexed by
// node number.  In the arc list for each node, to-nodes are in random
// order.
//
// See also LabeledEuclidean.
func Euclidean(nNodes, nArcs int, affinity float64, patience int, rr *rand.Rand) (g Directed, pos []struct{ X, Y float64 }, err error) {
	a := make(AdjacencyList, nNodes) // graph
	ri, rf, re := rand.Intn, rand.Float64, rand.ExpFloat64
	if rr != nil {
		ri, rf, re = rr.Intn, rr.Float64, rr.ExpFloat64
	}
	// generate random positions
	pos = make([]struct{ X, Y float64 }, nNodes)
	for i := range pos {
		pos[i].X = rf()
		pos[i].Y = rf()
	}
	// arcs
	var tooFar, dup int
arc:
	for i := 0; i < nArcs; {
		if tooFar == nArcs*patience {
			err = errors.New("affinity not found")
			return
		}
		if dup == nArcs*patience {
			err = errors.New("overcrowding")
			return
		}
		n1 := NI(ri(nNodes))
		var n2 NI
		for {
			n2 = NI(ri(nNodes))
			if n2 != n1 { // no graph loops
				break
			}
		}
		c1 := &pos[n1]
		c2 := &pos[n2]
		dist := math.Hypot(c2.X-c1.X, c2.Y-c1.Y)
		if dist*affinity > re() { // favor near nodes
			tooFar++
			continue
		}
		for _, nb := range a[n1] {
			if nb == n2 { // no parallel arcs
				dup++
				continue arc
			}
		}
		a[n1] = append(a[n1], n2)
		i++
	}
	g = Directed{a}
	return
}

// LabeledEuclidean generates a random simple graph on the Euclidean plane.
//
// Arc label values in the returned graph g are indexes into the return value
// wt.  Wt is the Euclidean distance between the from and to nodes of the arc.
//
// Otherwise the function arguments and return values are the same as for
// function Euclidean.  See Euclidean.
func LabeledEuclidean(nNodes, nArcs int, affinity float64, patience int, rr *rand.Rand) (g LabeledDirected, pos []struct{ X, Y float64 }, wt []float64, err error) {
	a := make(LabeledAdjacencyList, nNodes) // graph
	wt = make([]float64, nArcs)             // arc weights
	ri, rf, re := rand.Intn, rand.Float64, rand.ExpFloat64
	if rr != nil {
		ri, rf, re = rr.Intn, rr.Float64, rr.ExpFloat64
	}
	// generate random positions
	pos = make([]struct{ X, Y float64 }, nNodes)
	for i := range pos {
		pos[i].X = rf()
		pos[i].Y = rf()
	}
	// arcs
	var tooFar, dup int
arc:
	for i := 0; i < nArcs; {
		if tooFar == nArcs*patience {
			err = errors.New("affinity not found")
			return
		}
		if dup == nArcs*patience {
			err = errors.New("overcrowding")
			return
		}
		n1 := NI(ri(nNodes))
		var n2 NI
		for {
			n2 = NI(ri(nNodes))
			if n2 != n1 { // no graph loops
				break
			}
		}
		c1 := &pos[n1]
		c2 := &pos[n2]
		dist := math.Hypot(c2.X-c1.X, c2.Y-c1.Y)
		if dist*affinity > re() { // favor near nodes
			tooFar++
			continue
		}
		for _, nb := range a[n1] {
			if nb.To == n2 { // no parallel arcs
				dup++
				continue arc
			}
		}
		wt[i] = dist
		a[n1] = append(a[n1], Half{n2, LI(i)})
		i++
	}
	g = LabeledDirected{a}
	return
}

// Geometric generates a random geometric graph (RGG) on the Euclidean plane.
//
// An RGG is an undirected simple graph.  Nodes are associated with coordinates
// uniformly distributed on a unit square.  Edges are added between all nodes
// falling within a specified distance or radius of each other.
//
// The resulting number of edges is somewhat random but asymptotically
// approaches m = πr²n²/2.   The method accumulates and returns the actual
// number of edges constructed.  In the arc list for each node, to-nodes are
// ordered.  Consider using ShuffleArcLists if random order is important.
//
// If Rand r is nil, the rand package default shared source is used.
//
// See also LabeledGeometric.
func Geometric(nNodes int, radius float64, rr *rand.Rand) (g Undirected, pos []struct{ X, Y float64 }, m int) {
	// Expected degree is approximately nπr².
	a := make(AdjacencyList, nNodes)
	rf := rand.Float64
	if rr != nil {
		rf = rr.Float64
	}
	pos = make([]struct{ X, Y float64 }, nNodes)
	for i := range pos {
		pos[i].X = rf()
		pos[i].Y = rf()
	}
	for u, up := range pos {
		for v := u + 1; v < len(pos); v++ {
			vp := pos[v]
			dx := math.Abs(up.X - vp.X)
			if dx >= radius {
				continue
			}
			dy := math.Abs(up.Y - vp.Y)
			if dy >= radius {
				continue
			}
			if math.Hypot(dx, dy) < radius {
				a[u] = append(a[u], NI(v))
				a[v] = append(a[v], NI(u))
				m++
			}
		}
	}
	g = Undirected{a}
	return
}

// LabeledGeometric generates a random geometric graph (RGG) on the Euclidean
// plane.
//
// Edge label values in the returned graph g are indexes into the return value
// wt.  Wt is the Euclidean distance between nodes of the edge.  The graph
// size m is len(wt).
//
// See Geometric for additional description.
func LabeledGeometric(nNodes int, radius float64, rr *rand.Rand) (g LabeledUndirected, pos []struct{ X, Y float64 }, wt []float64) {
	a := make(LabeledAdjacencyList, nNodes)
	rf := rand.Float64
	if rr != nil {
		rf = rr.Float64
	}
	pos = make([]struct{ X, Y float64 }, nNodes)
	for i := range pos {
		pos[i].X = rf()
		pos[i].Y = rf()
	}
	for u, up := range pos {
		for v := u + 1; v < len(pos); v++ {
			vp := pos[v]
			if w := math.Hypot(up.X-vp.X, up.Y-vp.Y); w < radius {
				a[u] = append(a[u], Half{NI(v), LI(len(wt))})
				a[v] = append(a[v], Half{NI(u), LI(len(wt))})
				wt = append(wt, w)
			}
		}
	}
	g = LabeledUndirected{a}
	return
}

// GnmUndirected constructs a random simple undirected graph.
//
// Construction is by the Erdős–Rényi model where the specified number of
// distinct edges is selected from all possible edges with equal probability.
//
// Argument n is number of nodes, m is number of edges and must be <= n(n-1)/2.
//
// If Rand r is nil, the rand package default shared source is used.
//
// In the generated arc list for each node, to-nodes are ordered.
// Consider using ShuffleArcLists if random order is important.
//
// See also Gnm3Undirected, a method producing a statistically equivalent
// result, but by an algorithm with somewhat different performance properties.
// Performance of the two methods is expected to be similar in most cases but
// it may be worth trying both with your data to see if one has a clear
// advantage.
func GnmUndirected(n, m int, rr *rand.Rand) Undirected {
	// based on Alg. 2 from "Efficient Generation of Large Random Networks",
	// Vladimir Batagelj and Ulrik Brandes.
	// accessed at http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	ri := rand.Intn
	if rr != nil {
		ri = rr.Intn
	}
	re := n * (n - 1) / 2
	ml := m
	if m*2 > re {
		ml = re - m
	}
	e := map[int]struct{}{}
	for len(e) < ml {
		e[ri(re)] = struct{}{}
	}
	a := make(AdjacencyList, n)
	if m*2 > re {
		i := 0
		for v := 1; v < n; v++ {
			for w := 0; w < v; w++ {
				if _, ok := e[i]; !ok {
					a[v] = append(a[v], NI(w))
					a[w] = append(a[w], NI(v))
				}
				i++
			}
		}
	} else {
		for i := range e {
			v := 1 + int(math.Sqrt(.25+float64(2*i))-.5)
			w := i - (v * (v - 1) / 2)
			a[v] = append(a[v], NI(w))
			a[w] = append(a[w], NI(v))
		}
	}
	return Undirected{a}
}

// GnmDirected constructs a random simple directed graph.
//
// Construction is by the Erdős–Rényi model where the specified number of
// distinct arcs is selected from all possible arcs with equal probability.
//
// Argument n is number of nodes, ma is number of arcs and must be <= n(n-1).
//
// If Rand r is nil, the rand package default shared source is used.
//
// In the generated arc list for each node, to-nodes are ordered.
// Consider using ShuffleArcLists if random order is important.
//
// See also Gnm3Directed, a method producing a statistically equivalent
// result, but by
// an algorithm with somewhat different performance properties.  Performance
// of the two methods is expected to be similar in most cases but it may be
// worth trying both with your data to see if one has a clear advantage.
func GnmDirected(n, ma int, rr *rand.Rand) Directed {
	// based on Alg. 2 from "Efficient Generation of Large Random Networks",
	// Vladimir Batagelj and Ulrik Brandes.
	// accessed at http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	ri := rand.Intn
	if rr != nil {
		ri = rr.Intn
	}
	re := n * (n - 1)
	ml := ma
	if ma*2 > re {
		ml = re - ma
	}
	e := map[int]struct{}{}
	for len(e) < ml {
		e[ri(re)] = struct{}{}
	}
	a := make(AdjacencyList, n)
	if ma*2 > re {
		i := 0
		for v := 0; v < n; v++ {
			for w := 0; w < n; w++ {
				if w == v {
					continue
				}
				if _, ok := e[i]; !ok {
					a[v] = append(a[v], NI(w))
				}
				i++
			}
		}
	} else {
		for i := range e {
			v := i / (n - 1)
			w := i % (n - 1)
			if w >= v {
				w++
			}
			a[v] = append(a[v], NI(w))
		}
	}
	return Directed{a}
}

// Gnm3Undirected constructs a random simple undirected graph.
//
// Construction is by the Erdős–Rényi model where the specified number of
// distinct edges is selected from all possible edges with equal probability.
//
// Argument n is number of nodes, m is number of edges and must be <= n(n-1)/2.
//
// If Rand r is nil, the rand package default shared source is used.
//
// In the generated arc list for each node, to-nodes are ordered.
// Consider using ShuffleArcLists if random order is important.
//
// See also GnmUndirected, a method producing a statistically equivalent
// result, but by an algorithm with somewhat different performance properties.
// Performance of the two methods is expected to be similar in most cases but
// it may be worth trying both with your data to see if one has a clear
// advantage.
func Gnm3Undirected(n, m int, rr *rand.Rand) Undirected {
	// based on Alg. 3 from "Efficient Generation of Large Random Networks",
	// Vladimir Batagelj and Ulrik Brandes.
	// accessed at http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	//
	// I like this algorithm for its elegance.  Pitty it tends to run a
	// a little slower than the retry algorithm of Gnm.
	ri := rand.Intn
	if rr != nil {
		ri = rr.Intn
	}
	a := make(AdjacencyList, n)
	re := n * (n - 1) / 2
	rm := map[int]int{}
	for i := 0; i < m; i++ {
		er := i + ri(re-i)
		eNew := er
		if rp, ok := rm[er]; ok {
			eNew = rp
		}
		if rp, ok := rm[i]; !ok {
			rm[er] = i
		} else {
			rm[er] = rp
		}
		v := 1 + int(math.Sqrt(.25+float64(2*eNew))-.5)
		w := eNew - (v * (v - 1) / 2)
		a[v] = append(a[v], NI(w))
		a[w] = append(a[w], NI(v))
	}
	return Undirected{a}
}

// Gnm3Directed constructs a random simple directed graph.
//
// Construction is by the Erdős–Rényi model where the specified number of
// distinct arcs is selected from all possible arcs with equal probability.
//
// Argument n is number of nodes, ma is number of arcs and must be <= n(n-1).
//
// If Rand r is nil, the rand package default shared source is used.
//
// In the generated arc list for each node, to-nodes are ordered.
// Consider using ShuffleArcLists if random order is important.
//
// See also GnmDirected, a method producing a statistically equivalent result,
// but by an algorithm with somewhat different performance properties.
// Performance of the two methods is expected to be similar in most cases
// but it may be worth trying both with your data to see if one has a clear
// advantage.
func Gnm3Directed(n, ma int, rr *rand.Rand) Directed {
	// based on Alg. 3 from "Efficient Generation of Large Random Networks",
	// Vladimir Batagelj and Ulrik Brandes.
	// accessed at http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	ri := rand.Intn
	if rr != nil {
		ri = rr.Intn
	}
	a := make(AdjacencyList, n)
	re := n * (n - 1)
	rm := map[int]int{}
	for i := 0; i < ma; i++ {
		er := i + ri(re-i)
		eNew := er
		if rp, ok := rm[er]; ok {
			eNew = rp
		}
		if rp, ok := rm[i]; !ok {
			rm[er] = i
		} else {
			rm[er] = rp
		}
		v := eNew / (n - 1)
		w := eNew % (n - 1)
		if w >= v {
			w++
		}
		a[v] = append(a[v], NI(w))
	}
	return Directed{a}
}

// GnpUndirected constructs a random simple undirected graph.
//
// Construction is by the Gilbert model, an Erdős–Rényi like model where
// distinct edges are independently selected from all possible edges with
// the specified probability.
//
// Argument n is number of nodes, p is probability for selecting an edge.
//
// If Rand r is nil, the rand package default shared source is used.
//
// In the generated arc list for each node, to-nodes are ordered.
// Consider using ShuffleArcLists if random order is important.
//
// Also returned is the actual size m of constructed graph g.
func GnpUndirected(n int, p float64, rr *rand.Rand) (g Undirected, m int) {
	a := make(AdjacencyList, n)
	if n < 2 {
		return Undirected{a}, 0
	}
	rf := rand.Float64
	if rr != nil {
		rf = rr.Float64
	}
	// based on Alg. 1 from "Efficient Generation of Large Random Networks",
	// Vladimir Batagelj and Ulrik Brandes.
	// accessed at http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	var v, w NI = 1, -1
g:
	for c := 1 / math.Log(1-p); ; {
		w += 1 + NI(c*math.Log(1-rf()))
		for {
			if w < v {
				a[v] = append(a[v], w)
				a[w] = append(a[w], v)
				m++
				continue g
			}
			w -= v
			v++
			if v == NI(n) {
				break g
			}
		}
	}
	return Undirected{a}, m
}

// GnpDirected constructs a random simple directed graph.
//
// Construction is by the Gilbert model, an Erdős–Rényi like model where
// distinct arcs are independently selected from all possible arcs with
// the specified probability.
//
// Argument n is number of nodes, p is probability for selecting an arc.
//
// If Rand r is nil, the rand package default shared source is used.
//
// In the generated arc list for each node, to-nodes are ordered.
// Consider using ShuffleArcLists if random order is important.
//
// Also returned is the actual arc size m of constructed graph g.
func GnpDirected(n int, p float64, rr *rand.Rand) (g Directed, ma int) {
	a := make(AdjacencyList, n)
	if n < 2 {
		return Directed{a}, 0
	}
	rf := rand.Float64
	if rr != nil {
		rf = rr.Float64
	}
	// based on Alg. 1 from "Efficient Generation of Large Random Networks",
	// Vladimir Batagelj and Ulrik Brandes.
	// accessed at http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	var v, w NI = 0, -1
g:
	for c := 1 / math.Log(1-p); ; {
		w += 1 + NI(c*math.Log(1-rf()))
		for ; ; w -= NI(n) {
			if w == v {
				w++
			}
			if w < NI(n) {
				a[v] = append(a[v], w)
				ma++
				continue g
			}
			v++
			if v == NI(n) {
				break g
			}
		}
	}
	return Directed{a}, ma
}

// KroneckerDirected generates a Kronecker-like random directed graph.
//
// The returned graph g is simple and has no isolated nodes but is not
// necessarily fully connected.  The number of of nodes will be <= 2^scale,
// and will be near 2^scale for typical values of arcFactor, >= 2.
// ArcFactor * 2^scale arcs are generated, although loops and duplicate arcs
// are rejected.  In the arc list for each node, to-nodes are in random
// order.
//
// If Rand r is nil, the rand package default shared source is used.
//
// Return value ma is the number of arcs retained in the result graph.
func KroneckerDirected(scale uint, arcFactor float64, rr *rand.Rand) (g Directed, ma int) {
	a, m := kronecker(scale, arcFactor, true, rr)
	return Directed{a}, m
}

// KroneckerUndirected generates a Kronecker-like random undirected graph.
//
// The returned graph g is simple and has no isolated nodes but is not
// necessarily fully connected.  The number of of nodes will be <= 2^scale,
// and will be near 2^scale for typical values of edgeFactor, >= 2.
// EdgeFactor * 2^scale edges are generated, although loops and duplicate edges
// are rejected.  In the arc list for each node, to-nodes are in random
// order.
//
// If Rand r is nil, the rand package default shared source is used.
//
// Return value m is the true number of edges--not arcs--retained in the result
// graph.
func KroneckerUndirected(scale uint, edgeFactor float64, rr *rand.Rand) (g Undirected, m int) {
	al, s := kronecker(scale, edgeFactor, false, rr)
	return Undirected{al}, s
}

// Styled after the Graph500 example code.  Not well tested currently.
// Graph500 example generates undirected only.  No idea if the directed variant
// here is meaningful or not.
//
// note mma returns arc size ma for dir=true, but returns size m for dir=false
func kronecker(scale uint, edgeFactor float64, dir bool, rr *rand.Rand) (g AdjacencyList, mma int) {
	rf, ri, rp := rand.Float64, rand.Intn, rand.Perm
	if rr != nil {
		rf, ri, rp = rr.Float64, rr.Intn, rr.Perm
	}
	N := 1 << scale                      // node extent
	M := int(edgeFactor*float64(N) + .5) // number of arcs/edges to generate
	a, b, c := 0.57, 0.19, 0.19          // initiator probabilities
	ab := a + b
	cNorm := c / (1 - ab)
	aNorm := a / ab
	ij := make([][2]NI, M)
	bm := bits.New(N)
	var nNodes int
	for k := range ij {
		var i, j int
		for b := 1; b < N; b <<= 1 {
			if rf() > ab {
				i |= b
				if rf() > cNorm {
					j |= b
				}
			} else if rf() > aNorm {
				j |= b
			}
		}
		if bm.Bit(i) == 0 {
			bm.SetBit(i, 1)
			nNodes++
		}
		if bm.Bit(j) == 0 {
			bm.SetBit(j, 1)
			nNodes++
		}
		r := ri(k + 1) // shuffle edges as they are generated
		ij[k] = ij[r]
		ij[r] = [2]NI{NI(i), NI(j)}
	}
	p := rp(nNodes) // mapping to shuffle IDs of non-isolated nodes
	px := 0
	rn := make([]NI, N)
	for i := range rn {
		if bm.Bit(i) == 1 {
			rn[i] = NI(p[px]) // fill lookup table
			px++
		}
	}
	g = make(AdjacencyList, nNodes)
ij:
	for _, e := range ij {
		if e[0] == e[1] {
			continue // skip loops
		}
		ri, rj := rn[e[0]], rn[e[1]]
		for _, nb := range g[ri] {
			if nb == rj {
				continue ij // skip parallel edges
			}
		}
		g[ri] = append(g[ri], rj)
		mma++
		if !dir {
			g[rj] = append(g[rj], ri)
		}
	}
	return
}
