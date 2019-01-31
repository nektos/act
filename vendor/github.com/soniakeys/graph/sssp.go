// Copyright 2013 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

import (
	"container/heap"
	"fmt"
	"math"

	"github.com/soniakeys/bits"
)

// rNode holds data for a "reached" node
type rNode struct {
	nx    NI
	state int8    // state constants defined below
	f     float64 // "g+h", path dist + heuristic estimate
	fx    int     // heap.Fix index
}

// for rNode.state
const (
	unreached = 0
	reached   = 1
	open      = 1
	closed    = 2
)

type openHeap []*rNode

// A Heuristic is defined on a specific end node.  The function
// returns an estimate of the path distance from node argument
// "from" to the end node.  Two subclasses of heuristics are "admissible"
// and "monotonic."
//
// Admissible means the value returned is guaranteed to be less than or
// equal to the actual shortest path distance from the node to end.
//
// An admissible estimate may further be monotonic.
// Monotonic means that for any neighboring nodes A and B with half arc aB
// leading from A to B, and for heuristic h defined on some end node, then
// h(A) <= aB.ArcWeight + h(B).
//
// See AStarA for additional notes on implementing heuristic functions for
// AStar search methods.
type Heuristic func(from NI) float64

// Admissible returns true if heuristic h is admissible on graph g relative to
// the given end node.
//
// If h is inadmissible, the string result describes a counter example.
func (h Heuristic) Admissible(g LabeledAdjacencyList, w WeightFunc, end NI) (bool, string) {
	// invert graph
	inv := make(LabeledAdjacencyList, len(g))
	for from, nbs := range g {
		for _, nb := range nbs {
			inv[nb.To] = append(inv[nb.To],
				Half{To: NI(from), Label: nb.Label})
		}
	}
	// run dijkstra
	// Dijkstra.AllPaths takes a start node but after inverting the graph
	// argument end now represents the start node of the inverted graph.
	f, _, dist, _ := inv.Dijkstra(end, -1, w)
	// compare h to found shortest paths
	for n := range inv {
		if f.Paths[n].Len == 0 {
			continue // no path, any heuristic estimate is fine.
		}
		if !(h(NI(n)) <= dist[n]) {
			return false, fmt.Sprintf("h(%d) = %g, "+
				"required to be <= found shortest path (%g)",
				n, h(NI(n)), dist[n])
		}
	}
	return true, ""
}

// Monotonic returns true if heuristic h is monotonic on weighted graph g.
//
// If h is non-monotonic, the string result describes a counter example.
func (h Heuristic) Monotonic(g LabeledAdjacencyList, w WeightFunc) (bool, string) {
	// precompute
	hv := make([]float64, len(g))
	for n := range g {
		hv[n] = h(NI(n))
	}
	// iterate over all edges
	for from, nbs := range g {
		for _, nb := range nbs {
			arcWeight := w(nb.Label)
			if !(hv[from] <= arcWeight+hv[nb.To]) {
				return false, fmt.Sprintf("h(%d) = %g, "+
					"required to be <= arc weight + h(%d) (= %g + %g = %g)",
					from, hv[from],
					nb.To, arcWeight, hv[nb.To], arcWeight+hv[nb.To])
			}
		}
	}
	return true, ""
}

// AStarA finds a path between two nodes.
//
// AStarA implements both algorithm A and algorithm A*.  The difference in the
// two algorithms is strictly in the heuristic estimate returned by argument h.
// If h is an "admissible" heuristic estimate, then the algorithm is termed A*,
// otherwise it is algorithm A.
//
// Like Dijkstra's algorithm, AStarA with an admissible heuristic finds the
// shortest path between start and end.  AStarA generally runs faster than
// Dijkstra though, by using the heuristic distance estimate.
//
// AStarA with an inadmissible heuristic becomes algorithm A.  Algorithm A
// will find a path, but it is not guaranteed to be the shortest path.
// The heuristic still guides the search however, so a nearly admissible
// heuristic is likely to find a very good path, if not the best.  Quality
// of the path returned degrades gracefully with the quality of the heuristic.
//
// The heuristic function h should ideally be fairly inexpensive.  AStarA
// may call it more than once for the same node, especially as graph density
// increases.  In some cases it may be worth the effort to memoize or
// precompute values.
//
// Argument g is the graph to be searched, with arc weights returned by w.
// As usual for AStar, arc weights must be non-negative.
// Graphs may be directed or undirected.
//
// If AStarA finds a path it returns a FromList encoding the path, the arc
// labels for path nodes, the total path distance, and ok = true.
// Otherwise it returns ok = false.
func (g LabeledAdjacencyList) AStarA(w WeightFunc, start, end NI, h Heuristic) (f FromList, labels []LI, dist float64, ok bool) {
	// NOTE: AStarM is largely duplicate code.

	f = NewFromList(len(g))
	labels = make([]LI, len(g))
	d := make([]float64, len(g))
	r := make([]rNode, len(g))
	for i := range r {
		r[i].nx = NI(i)
	}
	// start node is reached initially
	cr := &r[start]
	cr.state = reached
	cr.f = h(start) // total path estimate is estimate from start
	rp := f.Paths
	rp[start] = PathEnd{Len: 1, From: -1} // path length at start is 1 node
	// oh is a heap of nodes "open" for exploration.  nodes go on the heap
	// when they get an initial or new "g" path distance, and therefore a
	// new "f" which serves as priority for exploration.
	oh := openHeap{cr}
	for len(oh) > 0 {
		bestPath := heap.Pop(&oh).(*rNode)
		bestNode := bestPath.nx
		if bestNode == end {
			return f, labels, d[end], true
		}
		bp := &rp[bestNode]
		nextLen := bp.Len + 1
		for _, nb := range g[bestNode] {
			alt := &r[nb.To]
			ap := &rp[alt.nx]
			// "g" path distance from start
			g := d[bestNode] + w(nb.Label)
			if alt.state == reached {
				if g > d[nb.To] {
					// candidate path to nb is longer than some alternate path
					continue
				}
				if g == d[nb.To] && nextLen >= ap.Len {
					// candidate path has identical length of some alternate
					// path but it takes no fewer hops.
					continue
				}
				// cool, we found a better way to get to this node.
				// record new path data for this node and
				// update alt with new data and make sure it's on the heap.
				*ap = PathEnd{From: bestNode, Len: nextLen}
				labels[nb.To] = nb.Label
				d[nb.To] = g
				alt.f = g + h(nb.To)
				if alt.fx < 0 {
					heap.Push(&oh, alt)
				} else {
					heap.Fix(&oh, alt.fx)
				}
			} else {
				// bestNode being reached for the first time.
				*ap = PathEnd{From: bestNode, Len: nextLen}
				labels[nb.To] = nb.Label
				d[nb.To] = g
				alt.f = g + h(nb.To)
				alt.state = reached
				heap.Push(&oh, alt) // and it's now open for exploration
			}
		}
	}
	return // no path
}

// AStarAPath finds a shortest path using the AStarA algorithm.
//
// This is a convenience method with a simpler result than the AStarA method.
// See documentation on the AStarA method.
//
// If a path is found, the non-nil node path is returned with the total path
// distance.  Otherwise the returned path will be nil.
func (g LabeledAdjacencyList) AStarAPath(start, end NI, h Heuristic, w WeightFunc) (LabeledPath, float64) {
	f, labels, d, _ := g.AStarA(w, start, end, h)
	return f.PathToLabeled(end, labels, nil), d
}

// AStarM is AStarA optimized for monotonic heuristic estimates.
//
// Note that this function requires a monotonic heuristic.  Results will
// not be meaningful if argument h is non-monotonic.
//
// See AStarA for general usage.  See Heuristic for notes on monotonicity.
func (g LabeledAdjacencyList) AStarM(w WeightFunc, start, end NI, h Heuristic) (f FromList, labels []LI, dist float64, ok bool) {
	// NOTE: AStarM is largely code duplicated from AStarA.
	// Differences are noted in comments in this method.

	f = NewFromList(len(g))
	labels = make([]LI, len(g))
	d := make([]float64, len(g))
	r := make([]rNode, len(g))
	for i := range r {
		r[i].nx = NI(i)
	}
	cr := &r[start]

	// difference from AStarA:
	// instead of a bit to mark a reached node, there are two states,
	// open and closed. open marks nodes "open" for exploration.
	// nodes are marked open as they are reached, then marked
	// closed as they are found to be on the best path.
	cr.state = open

	cr.f = h(start)
	rp := f.Paths
	rp[start] = PathEnd{Len: 1, From: -1}
	oh := openHeap{cr}
	for len(oh) > 0 {
		bestPath := heap.Pop(&oh).(*rNode)
		bestNode := bestPath.nx
		if bestNode == end {
			return f, labels, d[end], true
		}

		// difference from AStarA:
		// move nodes to closed list as they are found to be best so far.
		bestPath.state = closed

		bp := &rp[bestNode]
		nextLen := bp.Len + 1
		for _, nb := range g[bestNode] {
			alt := &r[nb.To]

			// difference from AStarA:
			// Monotonicity means that f cannot be improved.
			if alt.state == closed {
				continue
			}

			ap := &rp[alt.nx]
			g := d[bestNode] + w(nb.Label)

			// difference from AStarA:
			// test for open state, not just reached
			if alt.state == open {

				if g > d[nb.To] {
					continue
				}
				if g == d[nb.To] && nextLen >= ap.Len {
					continue
				}
				*ap = PathEnd{From: bestNode, Len: nextLen}
				labels[nb.To] = nb.Label
				d[nb.To] = g
				alt.f = g + h(nb.To)

				// difference from AStarA:
				// we know alt was on the heap because we found it marked open
				heap.Fix(&oh, alt.fx)
			} else {
				*ap = PathEnd{From: bestNode, Len: nextLen}
				labels[nb.To] = nb.Label
				d[nb.To] = g
				alt.f = g + h(nb.To)

				// difference from AStarA:
				// nodes are opened when first reached
				alt.state = open
				heap.Push(&oh, alt)
			}
		}
	}
	return
}

// AStarMPath finds a shortest path using the AStarM algorithm.
//
// This is a convenience method with a simpler result than the AStarM method.
// See documentation on the AStarM and AStarA methods.
//
// If a path is found, the non-nil node path is returned with the total path
// distance.  Otherwise the returned path will be nil.
func (g LabeledAdjacencyList) AStarMPath(start, end NI, h Heuristic, w WeightFunc) (LabeledPath, float64) {
	f, labels, d, _ := g.AStarM(w, start, end, h)
	return f.PathToLabeled(end, labels, nil), d
}

// implement container/heap
func (h openHeap) Len() int           { return len(h) }
func (h openHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h openHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].fx = i
	h[j].fx = j
}
func (p *openHeap) Push(x interface{}) {
	h := *p
	fx := len(h)
	h = append(h, x.(*rNode))
	h[fx].fx = fx
	*p = h
}

func (p *openHeap) Pop() interface{} {
	h := *p
	last := len(h) - 1
	*p = h[:last]
	h[last].fx = -1
	return h[last]
}

// BellmanFord finds shortest paths from a start node in a weighted directed
// graph using the Bellman-Ford-Moore algorithm.
//
// WeightFunc w must translate arc labels to arc weights.
// Negative arc weights are allowed but not negative cycles.
// Loops and parallel arcs are allowed.
//
// If the algorithm completes without encountering a negative cycle the method
// returns shortest paths encoded in a FromList, labels and path distances
// indexed by node, and return value end = -1.
//
// If it encounters a negative cycle reachable from start it returns end >= 0.
// In this case the cycle can be obtained by calling f.BellmanFordCycle(end).
//
// Negative cycles are only detected when reachable from start.  A negative
// cycle not reachable from start will not prevent the algorithm from finding
// shortest paths from start.
//
// See also NegativeCycle to find a cycle anywhere in the graph, see
// NegativeCycles for enumerating all negative cycles, and see
// HasNegativeCycle for lighter-weight negative cycle detection,
func (g LabeledDirected) BellmanFord(w WeightFunc, start NI) (f FromList, labels []LI, dist []float64, end NI) {
	a := g.LabeledAdjacencyList
	f = NewFromList(len(a))
	labels = make([]LI, len(a))
	dist = make([]float64, len(a))
	inf := math.Inf(1)
	for i := range dist {
		dist[i] = inf
	}
	rp := f.Paths
	rp[start] = PathEnd{Len: 1, From: -1}
	dist[start] = 0
	for _ = range a[1:] {
		imp := false
		for from, nbs := range a {
			fp := &rp[from]
			d1 := dist[from]
			for _, nb := range nbs {
				d2 := d1 + w(nb.Label)
				to := &rp[nb.To]
				// TODO improve to break ties
				if fp.Len > 0 && d2 < dist[nb.To] {
					*to = PathEnd{From: NI(from), Len: fp.Len + 1}
					labels[nb.To] = nb.Label
					dist[nb.To] = d2
					imp = true
				}
			}
		}
		if !imp {
			break
		}
	}
	for from, nbs := range a {
		d1 := dist[from]
		for _, nb := range nbs {
			if d1+w(nb.Label) < dist[nb.To] {
				// return nb as end of a path with negative cycle at root
				return f, labels, dist, NI(from)
			}
		}
	}
	return f, labels, dist, -1
}

// BellmanFordCycle decodes a negative cycle detected by BellmanFord.
//
// Receiver f and argument end must be results returned from BellmanFord.
func (f FromList) BellmanFordCycle(end NI) (c []NI) {
	p := f.Paths
	b := bits.New(len(p))
	for b.Bit(int(end)) == 0 {
		b.SetBit(int(end), 1)
		end = p[end].From
	}
	for b.Bit(int(end)) == 1 {
		c = append(c, end)
		b.SetBit(int(end), 0)
		end = p[end].From
	}
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
	}
	return
}

// HasNegativeCycle returns true if the graph contains any negative cycle.
//
// HasNegativeCycle uses a Bellman-Ford-like algorithm, but finds negative
// cycles anywhere in the graph.  Also path information is not computed,
// reducing memory use somewhat compared to BellmanFord.
//
// See also NegativeCycle to obtain the cycle, see NegativeCycles for
// enumerating all negative cycles, and see BellmanFord for single source
// shortest path searches with negative cycle detection.
func (g LabeledDirected) HasNegativeCycle(w WeightFunc) bool {
	a := g.LabeledAdjacencyList
	dist := make([]float64, len(a))
	for _ = range a[1:] {
		imp := false
		for from, nbs := range a {
			d1 := dist[from]
			for _, nb := range nbs {
				d2 := d1 + w(nb.Label)
				if d2 < dist[nb.To] {
					dist[nb.To] = d2
					imp = true
				}
			}
		}
		if !imp {
			break
		}
	}
	for from, nbs := range a {
		d1 := dist[from]
		for _, nb := range nbs {
			if d1+w(nb.Label) < dist[nb.To] {
				return true // negative cycle
			}
		}
	}
	return false
}

// NegativeCycle finds a negative cycle if one exists.
//
// NegativeCycle uses a Bellman-Ford-like algorithm, but finds negative
// cycles anywhere in the graph.  If a negative cycle exists, one will be
// returned.  The result is nil if no negative cycle exists.
//
// See also NegativeCycles for enumerating all negative cycles, see
// HasNegativeCycle for lighter-weight cycle detection, and see
// BellmanFord for single source shortest paths, also with negative cycle
// detection.
func (g LabeledDirected) NegativeCycle(w WeightFunc) (c []Half) {
	a := g.LabeledAdjacencyList
	f := NewFromList(len(a))
	p := f.Paths
	for n := range p {
		p[n] = PathEnd{From: -1, Len: 1}
	}
	labels := make([]LI, len(a))
	dist := make([]float64, len(a))
	for _ = range a {
		imp := false
		for from, nbs := range a {
			fp := &p[from]
			d1 := dist[from]
			for _, nb := range nbs {
				d2 := d1 + w(nb.Label)
				to := &p[nb.To]
				if fp.Len > 0 && d2 < dist[nb.To] {
					*to = PathEnd{From: NI(from), Len: fp.Len + 1}
					labels[nb.To] = nb.Label
					dist[nb.To] = d2
					imp = true
				}
			}
		}
		if !imp {
			return nil
		}
	}
	vis := bits.New(len(a))
a:
	for n := range a {
		end := n
		b := bits.New(len(a))
		for b.Bit(end) == 0 {
			if vis.Bit(end) == 1 {
				continue a
			}
			vis.SetBit(end, 1)
			b.SetBit(end, 1)
			end = int(p[end].From)
			if end < 0 {
				continue a
			}
		}
		for b.Bit(end) == 1 {
			c = append(c, Half{NI(end), labels[end]})
			b.SetBit(end, 0)
			end = int(p[end].From)
		}
		for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
			c[i], c[j] = c[j], c[i]
		}
		return c
	}
	return nil // no negative cycle
}

// DAGMinDistPath finds a single shortest path.
//
// Shortest means minimum sum of arc weights.
//
// Returned is the path and distance as returned by FromList.PathTo.
//
// This is a convenience method.  See DAGOptimalPaths for more options.
func (g LabeledDirected) DAGMinDistPath(start, end NI, w WeightFunc) (LabeledPath, float64, error) {
	return g.dagPath(start, end, w, false)
}

// DAGMaxDistPath finds a single longest path.
//
// Longest means maximum sum of arc weights.
//
// Returned is the path and distance as returned by FromList.PathTo.
//
// This is a convenience method.  See DAGOptimalPaths for more options.
func (g LabeledDirected) DAGMaxDistPath(start, end NI, w WeightFunc) (LabeledPath, float64, error) {
	return g.dagPath(start, end, w, true)
}

func (g LabeledDirected) dagPath(start, end NI, w WeightFunc, longest bool) (LabeledPath, float64, error) {
	o, _ := g.Topological()
	if o == nil {
		return LabeledPath{}, 0, fmt.Errorf("not a DAG")
	}
	f, labels, dist, _ := g.DAGOptimalPaths(start, end, o, w, longest)
	if f.Paths[end].Len == 0 {
		return LabeledPath{}, 0, fmt.Errorf("no path from %d to %d", start, end)
	}
	return f.PathToLabeled(end, labels, nil), dist[end], nil
}

// DAGOptimalPaths finds either longest or shortest distance paths in a
// directed acyclic graph.
//
// Path distance is the sum of arc weights on the path.
// Negative arc weights are allowed.
// Where multiple paths exist with the same distance, the path length
// (number of nodes) is used as a tie breaker.
//
// Receiver g must be a directed acyclic graph.  Argument o must be either nil
// or a topological ordering of g.  If nil, a topologcal ordering is
// computed internally.  If longest is true, an optimal path is a longest
// distance path.  Otherwise it is a shortest distance path.
//
// Argument start is the start node for paths, end is the end node.  If end
// is a valid node number, the method returns as soon as the optimal path
// to end is found.  If end is -1, all optimal paths from start are found.
//
// Paths and path distances are encoded in the returned FromList, labels,
// and dist slices.   The number of nodes reached is returned as nReached.
func (g LabeledDirected) DAGOptimalPaths(start, end NI, ordering []NI, w WeightFunc, longest bool) (f FromList, labels []LI, dist []float64, nReached int) {
	a := g.LabeledAdjacencyList
	f = NewFromList(len(a))
	f.Leaves = bits.New(len(a))
	labels = make([]LI, len(a))
	dist = make([]float64, len(a))
	if ordering == nil {
		ordering, _ = g.Topological()
	}
	// search ordering for start
	o := 0
	for ordering[o] != start {
		o++
	}
	var fBetter func(cand, ext float64) bool
	var iBetter func(cand, ext int) bool
	if longest {
		fBetter = func(cand, ext float64) bool { return cand > ext }
		iBetter = func(cand, ext int) bool { return cand > ext }
	} else {
		fBetter = func(cand, ext float64) bool { return cand < ext }
		iBetter = func(cand, ext int) bool { return cand < ext }
	}
	p := f.Paths
	p[start] = PathEnd{From: -1, Len: 1}
	f.MaxLen = 1
	leaves := &f.Leaves
	leaves.SetBit(int(start), 1)
	nReached = 1
	for n := start; n != end; n = ordering[o] {
		if p[n].Len > 0 && len(a[n]) > 0 {
			nDist := dist[n]
			candLen := p[n].Len + 1 // len for any candidate arc followed from n
			for _, to := range a[n] {
				leaves.SetBit(int(to.To), 1)
				candDist := nDist + w(to.Label)
				switch {
				case p[to.To].Len == 0: // first path to node to.To
					nReached++
				case fBetter(candDist, dist[to.To]): // better distance
				case candDist == dist[to.To] && iBetter(candLen, p[to.To].Len): // same distance but better path length
				default:
					continue
				}
				dist[to.To] = candDist
				p[to.To] = PathEnd{From: n, Len: candLen}
				labels[to.To] = to.Label
				if candLen > f.MaxLen {
					f.MaxLen = candLen
				}
			}
			leaves.SetBit(int(n), 0)
		}
		o++
		if o == len(ordering) {
			break
		}
	}
	return
}

// Dijkstra finds shortest paths by Dijkstra's algorithm.
//
// Shortest means shortest distance where distance is the
// sum of arc weights.  Where multiple paths exist with the same distance,
// a path with the minimum number of nodes is returned.
//
// As usual for Dijkstra's algorithm, arc weights must be non-negative.
// Graphs may be directed or undirected.  Loops and parallel arcs are
// allowed.
//
// Paths and path distances are encoded in the returned FromList and dist
// slice.   Returned labels are the labels of arcs followed to each node.
// The number of nodes reached is returned as nReached.
func (g LabeledAdjacencyList) Dijkstra(start, end NI, w WeightFunc) (f FromList, labels []LI, dist []float64, nReached int) {
	r := make([]tentResult, len(g))
	for i := range r {
		r[i].nx = NI(i)
	}
	f = NewFromList(len(g))
	labels = make([]LI, len(g))
	dist = make([]float64, len(g))
	current := start
	rp := f.Paths
	rp[current] = PathEnd{Len: 1, From: -1} // path length at start is 1 node
	cr := &r[current]
	cr.dist = 0    // distance at start is 0.
	cr.done = true // mark start done.  it skips the heap.
	nDone := 1     // accumulated for a return value
	var t tent
	for current != end {
		nextLen := rp[current].Len + 1
		for _, nb := range g[current] {
			// d.arcVis++
			hr := &r[nb.To]
			if hr.done {
				continue // skip nodes already done
			}
			dist := cr.dist + w(nb.Label)
			vl := rp[nb.To].Len
			visited := vl > 0
			if visited {
				if dist > hr.dist {
					continue // distance is worse
				}
				// tie breaker is a nice touch and doesn't seem to
				// impact performance much.
				if dist == hr.dist && nextLen >= vl {
					continue // distance same, but number of nodes is no better
				}
			}
			// the path through current to this node is shortest so far.
			// record new path data for this node and update tentative set.
			hr.dist = dist
			rp[nb.To].Len = nextLen
			rp[nb.To].From = current
			labels[nb.To] = nb.Label
			if visited {
				heap.Fix(&t, hr.fx)
			} else {
				heap.Push(&t, hr)
			}
		}
		//d.ndVis++
		if len(t) == 0 {
			// no more reachable nodes. AllPaths normal return
			return f, labels, dist, nDone
		}
		// new current is node with smallest tentative distance
		cr = heap.Pop(&t).(*tentResult)
		cr.done = true
		nDone++
		current = cr.nx
		dist[current] = cr.dist // store final distance
	}
	// normal return for single shortest path search
	return f, labels, dist, -1
}

// DijkstraPath finds a single shortest path.
//
// Returned is the path as returned by FromList.LabeledPathTo and the total
// path distance.
func (g LabeledAdjacencyList) DijkstraPath(start, end NI, w WeightFunc) (LabeledPath, float64) {
	f, labels, dist, _ := g.Dijkstra(start, end, w)
	return f.PathToLabeled(end, labels, nil), dist[end]
}

// tent implements container/heap
func (t tent) Len() int           { return len(t) }
func (t tent) Less(i, j int) bool { return t[i].dist < t[j].dist }
func (t tent) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
	t[i].fx = i
	t[j].fx = j
}
func (s *tent) Push(x interface{}) {
	nd := x.(*tentResult)
	nd.fx = len(*s)
	*s = append(*s, nd)
}
func (s *tent) Pop() interface{} {
	t := *s
	last := len(t) - 1
	*s = t[:last]
	return t[last]
}

type tentResult struct {
	dist float64 // tentative distance, sum of arc weights
	nx   NI      // slice index, "node id"
	fx   int     // heap.Fix index
	done bool
}

type tent []*tentResult
