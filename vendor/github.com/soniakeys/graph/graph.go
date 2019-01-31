// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

package graph

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"text/template"

	"github.com/soniakeys/bits"
)

// graph.go contains type definitions for all graph types and components.
// Also, go generate directives for source transformations.
//
// For readability, the types are defined in a dependency order:
//
//  NI
//  AdjacencyList
//  Directed
//  Undirected
//  Bipartite
//  Subgraph
//  DirectedSubgraph
//  UndirectedSubgraph
//  LI
//  Half
//  fromHalf
//  LabeledAdjacencyList
//  LabeledDirected
//  LabeledUndirected
//  LabeledBipartite
//  LabeledSubgraph
//  LabeledDirectedSubgraph
//  LabeledUndirectedSubgraph
//  Edge
//  LabeledEdge
//  LabeledPath
//  WeightFunc
//  WeightedEdgeList
//  TraverseOption

//go:generate cp adj_cg.go adj_RO.go
//go:generate gofmt -r "LabeledAdjacencyList -> AdjacencyList" -w adj_RO.go
//go:generate gofmt -r "n.To -> n" -w adj_RO.go
//go:generate gofmt -r "Half -> NI" -w adj_RO.go
//go:generate gofmt -r "LabeledSubgraph -> Subgraph" -w adj_RO.go

//go:generate cp dir_cg.go dir_RO.go
//go:generate gofmt -r "LabeledDirected -> Directed" -w dir_RO.go
//go:generate gofmt -r "LabeledDirectedSubgraph -> DirectedSubgraph" -w dir_RO.go
//go:generate gofmt -r "LabeledAdjacencyList -> AdjacencyList" -w dir_RO.go
//go:generate gofmt -r "labEulerian -> eulerian" -w dir_RO.go
//go:generate gofmt -r "newLabEulerian -> newEulerian" -w dir_RO.go
//go:generate gofmt -r "Half{n, -1} -> n" -w dir_RO.go
//go:generate gofmt -r "n.To -> n" -w dir_RO.go
//go:generate gofmt -r "Half -> NI" -w dir_RO.go

//go:generate cp undir_cg.go undir_RO.go
//go:generate gofmt -r "LabeledUndirected -> Undirected" -w undir_RO.go
//go:generate gofmt -r "LabeledBipartite -> Bipartite" -w undir_RO.go
//go:generate gofmt -r "LabeledUndirectedSubgraph -> UndirectedSubgraph" -w undir_RO.go
//go:generate gofmt -r "LabeledAdjacencyList -> AdjacencyList" -w undir_RO.go
//go:generate gofmt -r "newLabEulerian -> newEulerian" -w undir_RO.go
//go:generate gofmt -r "Half{n, -1} -> n" -w undir_RO.go
//go:generate gofmt -r "n.To -> n" -w undir_RO.go
//go:generate gofmt -r "Half -> NI" -w undir_RO.go

// An AdjacencyList represents a graph as a list of neighbors for each node.
// The "node ID" of a node is simply it's slice index in the AdjacencyList.
// For an AdjacencyList g, g[n] represents arcs going from node n to nodes
// g[n].
//
// Adjacency lists are inherently directed but can be used to represent
// directed or undirected graphs.  See types Directed and Undirected.
type AdjacencyList [][]NI

// Directed represents a directed graph.
//
// Directed methods generally rely on the graph being directed, specifically
// that arcs do not have reciprocals.
type Directed struct {
	AdjacencyList // embedded to include AdjacencyList methods
}

// Undirected represents an undirected graph.
//
// In an undirected graph, for each arc between distinct nodes there is also
// a reciprocal arc, an arc in the opposite direction.  Loops do not have
// reciprocals.
//
// Undirected methods generally rely on the graph being undirected,
// specifically that every arc between distinct nodes has a reciprocal.
type Undirected struct {
	AdjacencyList // embedded to include AdjacencyList methods
}

// Bipartite represents a bipartite graph.
//
// In a bipartite graph, nodes are partitioned into two sets, or
// "colors," such that every edge in the graph goes from one set to the
// other.
//
// Member Color represents the partition with a bitmap of length the same
// as the number of nodes in the graph.  For convenience N0 stores the number
// of zero bits in Color.
//
// To construct a Bipartite object, if you can easily or efficiently use
// available information to construct the Color member, then you should do
// this and construct a Bipartite object with a Go struct literal.
//
// If partition information is not readily available, see the constructor
// Undirected.Bipartite.
//
// Alternatively, in some cases where the graph may have multiple connected
// components, the lower level Undirected.BipartiteComponent can be used to
// control color assignment by component.
type Bipartite struct {
	Undirected
	Color bits.Bits
	N0    int
}

// Subgraph represents a subgraph mapped to a supergraph.
//
// The subgraph is the embedded AdjacencyList and so the Subgraph type inherits
// all methods of Adjacency list.
//
// The embedded subgraph mapped relative to a specific supergraph, member
// Super.  A subgraph may have fewer nodes than its supergraph.
// Each node of the subgraph must map to a distinct node of the supergraph.
//
// The mapping giving the supergraph node for a given subgraph node is
// represented by member SuperNI, a slice parallel to the the subgraph.
//
// The mapping in the other direction, giving a subgraph NI for a given
// supergraph NI, is represented with map SubNI.
//
// Multiple Subgraphs can be created relative to a single supergraph.
// The Subgraph type represents a mapping to only a single supergraph however.
//
// See graph methods InduceList and InduceBits for construction of
// node-induced subgraphs.
//
// Alternatively an empty subgraph can be constructed with InduceList(nil).
// Arbitrary subgraphs can then be built up with methods AddNode and AddArc.
type Subgraph struct {
	AdjacencyList                // the subgraph
	Super         *AdjacencyList // the supergraph
	SubNI         map[NI]NI      // subgraph NIs, indexed by supergraph NIs
	SuperNI       []NI           // supergraph NIs indexed by subgraph NIs
}

// DirectedSubgraph represents a subgraph mapped to a supergraph.
//
// See additional doc at Subgraph type.
type DirectedSubgraph struct {
	Directed
	Super   *Directed
	SubNI   map[NI]NI
	SuperNI []NI
}

// UndirectedSubgraph represents a subgraph mapped to a supergraph.
//
// See additional doc at Subgraph type.
type UndirectedSubgraph struct {
	Undirected
	Super   *Undirected
	SubNI   map[NI]NI
	SuperNI []NI
}

// LI is a label integer, used for associating labels with arcs.
type LI int32

// Half is a half arc, representing a labeled arc and the "neighbor" node
// that the arc leads to.
//
// Halfs can be composed to form a labeled adjacency list.
type Half struct {
	To    NI // node ID, usable as a slice index
	Label LI // half-arc ID for application data, often a weight
}

// fromHalf is a half arc, representing a labeled arc and the "neighbor" node
// that the arc originates from.
//
// This used internally in a couple of places.  It used to be exported but is
// not currently needed anwhere in the API.
type fromHalf struct {
	From  NI
	Label LI
}

// A LabeledAdjacencyList represents a graph as a list of neighbors for each
// node, connected by labeled arcs.
//
// Arc labels are not necessarily unique arc IDs.  Different arcs can have
// the same label.
//
// Arc labels are commonly used to assocate a weight with an arc.  Arc labels
// are general purpose however and can be used to associate arbitrary
// information with an arc.
//
// Methods implementing weighted graph algorithms will commonly take a
// weight function that turns a label int into a float64 weight.
//
// If only a small amount of information -- such as an integer weight or
// a single printable character -- needs to be associated, it can sometimes
// be possible to encode the information directly into the label int.  For
// more generality, some lookup scheme will be needed.
//
// In an undirected labeled graph, reciprocal arcs must have identical labels.
// Note this does not preclude parallel arcs with different labels.
type LabeledAdjacencyList [][]Half

// LabeledDirected represents a directed labeled graph.
//
// This is the labeled version of Directed.  See types LabeledAdjacencyList
// and Directed.
type LabeledDirected struct {
	LabeledAdjacencyList // embedded to include LabeledAdjacencyList methods
}

// LabeledUndirected represents an undirected labeled graph.
//
// This is the labeled version of Undirected.  See types LabeledAdjacencyList
// and Undirected.
type LabeledUndirected struct {
	LabeledAdjacencyList // embedded to include LabeledAdjacencyList methods
}

// LabeledBipartite represents a bipartite graph.
//
// In a bipartite graph, nodes are partitioned into two sets, or
// "colors," such that every edge in the graph goes from one set to the
// other.
//
// Member Color represents the partition with a bitmap of length the same
// as the number of nodes in the graph.  For convenience N0 stores the number
// of zero bits in Color.
//
// To construct a LabeledBipartite object, if you can easily or efficiently use
// available information to construct the Color member, then you should do
// this and construct a LabeledBipartite object with a Go struct literal.
//
// If partition information is not readily available, see the constructor
// Undirected.LabeledBipartite.
//
// Alternatively, in some cases where the graph may have multiple connected
// components, the lower level LabeledUndirected.BipartiteComponent can be used
// to control color assignment by component.
type LabeledBipartite struct {
	LabeledUndirected
	Color bits.Bits
	N0    int
}

// LabeledSubgraph represents a subgraph mapped to a supergraph.
//
// See additional doc at Subgraph type.
type LabeledSubgraph struct {
	LabeledAdjacencyList
	Super   *LabeledAdjacencyList
	SubNI   map[NI]NI
	SuperNI []NI
}

// LabeledDirectedSubgraph represents a subgraph mapped to a supergraph.
//
// See additional doc at Subgraph type.
type LabeledDirectedSubgraph struct {
	LabeledDirected
	Super   *LabeledDirected
	SubNI   map[NI]NI
	SuperNI []NI
}

// LabeledUndirectedSubgraph represents a subgraph mapped to a supergraph.
//
// See additional doc at Subgraph type.
type LabeledUndirectedSubgraph struct {
	LabeledUndirected
	Super   *LabeledUndirected
	SubNI   map[NI]NI
	SuperNI []NI
}

// Edge is an undirected edge between nodes N1 and N2.
type Edge struct{ N1, N2 NI }

// LabeledEdge is an undirected edge with an associated label.
type LabeledEdge struct {
	Edge
	LI
}

// LabeledPath is a start node and a path of half arcs leading from start.
type LabeledPath struct {
	Start NI
	Path  []Half
}

// Distance returns total path distance given WeightFunc w.
func (p LabeledPath) Distance(w WeightFunc) float64 {
	d := 0.
	for _, h := range p.Path {
		d += w(h.Label)
	}
	return d
}

// WeightFunc returns a weight for a given label.
//
// WeightFunc is a parameter type for various search functions.  The intent
// is to return a weight corresponding to an arc label.  The name "weight"
// is an abstract term.  An arc "weight" will typically have some application
// specific meaning other than physical weight.
type WeightFunc func(label LI) (weight float64)

// WeightedEdgeList is a graph representation.
//
// It is a labeled edge list, with an associated weight function to return
// a weight given an edge label.
//
// Also associated is the order, or number of nodes of the graph.
// All nodes occurring in the edge list must be strictly less than Order.
//
// WeigtedEdgeList sorts by weight, obtained by calling the weight function.
// If weight computation is expensive, consider supplying a cached or
// memoized version.
type WeightedEdgeList struct {
	Order int
	WeightFunc
	Edges []LabeledEdge
}

// DistanceMatrix constructs a distance matrix corresponding to the weighted
// edges of l.
//
// An edge n1, n2 with WeightFunc return w is represented by both
// d[n1][n2] == w and d[n2][n1] = w.  In case of parallel edges, the lowest
// weight is stored.  The distance from any node to itself d[n][n] is 0, unless
// the node has a loop with a negative weight.  If g has no edge between n1 and
// distinct n2, +Inf is stored for d[n1][n2] and d[n2][n1].
//
// The returned DistanceMatrix is suitable for DistanceMatrix.FloydWarshall.
func (l WeightedEdgeList) DistanceMatrix() (d DistanceMatrix) {
	d = newDM(l.Order)
	for _, e := range l.Edges {
		n1 := e.Edge.N1
		n2 := e.Edge.N2
		wt := l.WeightFunc(e.LI)
		// < to pick min of parallel arcs (also nicely ignores NaN)
		if wt < d[n1][n2] {
			d[n1][n2] = wt
			d[n2][n1] = wt
		}
	}
	return
}

// A DistanceMatrix is a square matrix representing some distance between
// nodes of a graph.  If the graph is directected, d[from][to] represents
// some distance from node 'from' to node 'to'.  Depending on context, the
// distance may be an arc weight or path distance.  A value of +Inf typically
// means no arc or no path between the nodes.
type DistanceMatrix [][]float64

// little helper function, makes a blank distance matrix for FloydWarshall.
// could be exported?
func newDM(n int) DistanceMatrix {
	inf := math.Inf(1)
	d := make(DistanceMatrix, n)
	for i := range d {
		di := make([]float64, n)
		for j := range di {
			di[j] = inf
		}
		di[i] = 0
		d[i] = di
	}
	return d
}

// FloydWarshall finds all pairs shortest distances for a weighted graph
// without negative cycles.
//
// It operates on a distance matrix representing arcs of a graph and
// destructively replaces arc weights with shortest path distances.
//
// In receiver d, d[fr][to] will be the shortest distance from node
// 'fr' to node 'to'.  An element value of +Inf means no path exists.
// Any diagonal element < 0 indicates a negative cycle exists.
//
// See DistanceMatrix constructor methods of LabeledAdjacencyList and
// WeightedEdgeList for suitable inputs.
func (d DistanceMatrix) FloydWarshall() {
	for k, dk := range d {
		for _, di := range d {
			dik := di[k]
			for j := range d {
				if d2 := dik + dk[j]; d2 < di[j] {
					di[j] = d2
				}
			}
		}
	}
}

// PathMatrix is a return type for FloydWarshallPaths.
//
// It encodes all pairs shortest paths.
type PathMatrix [][]NI

// Path returns a shortest path from node start to end.
//
// Argument p is truncated, appended to, and returned as the result.
// Thus the underlying allocation is reused if possible.
// If there is no path from start to end, p is returned truncated to
// zero length.
//
// If receiver m is not a valid populated PathMatrix as returned by
// FloydWarshallPaths, behavior is undefined and a panic is likely.
func (m PathMatrix) Path(start, end NI, p []NI) []NI {
	p = p[:0]
	for {
		p = append(p, start)
		if start == end {
			return p
		}
		start = m[start][end]
		if start < 0 {
			return p[:0]
		}
	}
}

// FloydWarshallPaths finds all pairs shortest paths for a weighted graph
// without negative cycles.
//
// It operates on a distance matrix representing arcs of a graph and
// destructively replaces arc weights with shortest path distances.
//
// In receiver d, d[fr][to] will be the shortest distance from node
// 'fr' to node 'to'.  An element value of +Inf means no path exists.
// Any diagonal element < 0 indicates a negative cycle exists.
//
// The return value encodes the paths.  See PathMatrix.Path.
//
// See DistanceMatrix constructor methods of LabeledAdjacencyList and
// WeightedEdgeList for suitable inputs.
//
// See also similar method FloydWarshallFromLists which has a richer
// return value.
func (d DistanceMatrix) FloydWarshallPaths() PathMatrix {
	m := make(PathMatrix, len(d))
	inf := math.Inf(1)
	for i, di := range d {
		mi := make([]NI, len(d))
		for j, dij := range di {
			if dij == inf {
				mi[j] = -1
			} else {
				mi[j] = NI(j)
			}
		}
		m[i] = mi
	}
	for k, dk := range d {
		for i, di := range d {
			mi := m[i]
			dik := di[k]
			for j := range d {
				if d2 := dik + dk[j]; d2 < di[j] {
					di[j] = d2
					mi[j] = mi[k]
				}
			}
		}
	}
	return m
}

// FloydWarshallFromLists finds all pairs shortest paths for a weighted
// graph without negative cycles.
//
// It operates on a distance matrix representing arcs of a graph and
// destructively replaces arc weights with shortest path distances.
//
// In receiver d, d[fr][to] will be the shortest distance from node
// 'fr' to node 'to'.  An element value of +Inf means no path exists.
// Any diagonal element < 0 indicates a negative cycle exists.
//
// The return value encodes the paths.  The FromLists are fully populated
// with Leaves and Len values.  See for example FromList.PathTo for
// extracting paths.  Note though that for i'th FromList of the return
// value, PathTo(j) will return the path from j's root, which will not
// be i in the case that there is no path from i to j.  You must check
// the first node of the path to see if it is i.  If not, there is no
// path from i to j.  See example.
//
// See DistanceMatrix constructor methods of LabeledAdjacencyList and
// WeightedEdgeList for suitable inputs.
//
// See also similar method FloydWarshallPaths, which has a lighter
// weight return value.
func (d DistanceMatrix) FloydWarshallFromLists() []FromList {
	l := make([]FromList, len(d))
	inf := math.Inf(1)
	for i, di := range d {
		li := NewFromList(len(d))
		p := li.Paths
		for j, dij := range di {
			if i == j || dij == inf {
				p[j] = PathEnd{From: -1}
			} else {
				p[j] = PathEnd{From: NI(i)}
			}
		}
		l[i] = li
	}
	for k, dk := range d {
		pk := l[k].Paths
		for i, di := range d {
			dik := di[k]
			pi := l[i].Paths
			for j := range d {
				if d2 := dik + dk[j]; d2 < di[j] {
					di[j] = d2
					pi[j] = pk[j]
				}
			}
		}
	}
	for _, li := range l {
		li.RecalcLeaves()
		li.RecalcLen()
	}
	return l
}

// AddEdge adds an edge to a subgraph.
//
// For argument e, e.N1 and e.N2 must be NIs in supergraph s.Super.  As with
// AddNode, AddEdge panics if e.N1 and e.N2 are not valid node indexes of
// s.Super.
//
// Edge e must exist in s.Super.  Further, the number of
// parallel edges in the subgraph cannot exceed the number of corresponding
// parallel edges in the supergraph.  That is, each edge already added to the
// subgraph counts against the edges available in the supergraph.  If a matching
// edge is not available, AddEdge returns an error.
//
// If a matching edge is available, subgraph nodes are added as needed, the
// subgraph edge is added, and the method returns nil.
func (s *UndirectedSubgraph) AddEdge(n1, n2 NI) error {
	// verify supergraph NIs first, but without adding subgraph nodes just yet.
	if int(n1) < 0 || int(n1) >= s.Super.Order() {
		panic(fmt.Sprint("AddEdge: NI ", n1, " not in supergraph"))
	}
	if int(n2) < 0 || int(n2) >= s.Super.Order() {
		panic(fmt.Sprint("AddEdge: NI ", n2, " not in supergraph"))
	}
	// count existing matching edges in subgraph
	n := 0
	a := s.Undirected.AdjacencyList
	if b1, ok := s.SubNI[n1]; ok {
		if b2, ok := s.SubNI[n2]; ok {
			// both NIs already exist in subgraph, need to count edges
			for _, t := range a[b1] {
				if t == b2 {
					n++
				}
			}
			if b1 != b2 {
				// verify reciprocal arcs exist
				r := 0
				for _, t := range a[b2] {
					if t == b1 {
						r++
					}
				}
				if r < n {
					n = r
				}
			}
		}
	}
	// verify matching edges are available in supergraph
	m := 0
	for _, t := range (*s.Super).AdjacencyList[n1] {
		if t == n2 {
			if m == n {
				goto r // arc match after all existing arcs matched
			}
			m++
		}
	}
	return errors.New("edge not available in supergraph")
r:
	if n1 != n2 {
		// verify reciprocal arcs
		m = 0
		for _, t := range (*s.Super).AdjacencyList[n2] {
			if t == n1 {
				if m == n {
					goto good
				}
				m++
			}
		}
		return errors.New("edge not available in supergraph")
	}
good:
	// matched enough edges.  nodes can finally
	// be added as needed and then the edge can be added.
	b1 := s.AddNode(n1)
	b2 := s.AddNode(n2)
	s.Undirected.AddEdge(b1, b2)
	return nil // success
}

// AddEdge adds an edge to a subgraph.
//
// For argument e, e.N1 and e.N2 must be NIs in supergraph s.Super.  As with
// AddNode, AddEdge panics if e.N1 and e.N2 are not valid node indexes of
// s.Super.
//
// Edge e must exist in s.Super with label l.  Further, the number of
// parallel edges in the subgraph cannot exceed the number of corresponding
// parallel edges in the supergraph.  That is, each edge already added to the
// subgraph counts against the edges available in the supergraph.  If a matching
// edge is not available, AddEdge returns an error.
//
// If a matching edge is available, subgraph nodes are added as needed, the
// subgraph edge is added, and the method returns nil.
func (s *LabeledUndirectedSubgraph) AddEdge(e Edge, l LI) error {
	// verify supergraph NIs first, but without adding subgraph nodes just yet.
	if int(e.N1) < 0 || int(e.N1) >= s.Super.Order() {
		panic(fmt.Sprint("AddEdge: NI ", e.N1, " not in supergraph"))
	}
	if int(e.N2) < 0 || int(e.N2) >= s.Super.Order() {
		panic(fmt.Sprint("AddEdge: NI ", e.N2, " not in supergraph"))
	}
	// count existing matching edges in subgraph
	n := 0
	a := s.LabeledUndirected.LabeledAdjacencyList
	if b1, ok := s.SubNI[e.N1]; ok {
		if b2, ok := s.SubNI[e.N2]; ok {
			// both NIs already exist in subgraph, need to count edges
			h := Half{b2, l}
			for _, t := range a[b1] {
				if t == h {
					n++
				}
			}
			if b1 != b2 {
				// verify reciprocal arcs exist
				r := 0
				h.To = b1
				for _, t := range a[b2] {
					if t == h {
						r++
					}
				}
				if r < n {
					n = r
				}
			}
		}
	}
	// verify matching edges are available in supergraph
	m := 0
	h := Half{e.N2, l}
	for _, t := range (*s.Super).LabeledAdjacencyList[e.N1] {
		if t == h {
			if m == n {
				goto r // arc match after all existing arcs matched
			}
			m++
		}
	}
	return errors.New("edge not available in supergraph")
r:
	if e.N1 != e.N2 {
		// verify reciprocal arcs
		m = 0
		h.To = e.N1
		for _, t := range (*s.Super).LabeledAdjacencyList[e.N2] {
			if t == h {
				if m == n {
					goto good
				}
				m++
			}
		}
		return errors.New("edge not available in supergraph")
	}
good:
	// matched enough edges.  nodes can finally
	// be added as needed and then the edge can be added.
	n1 := s.AddNode(e.N1)
	n2 := s.AddNode(e.N2)
	s.LabeledUndirected.AddEdge(Edge{n1, n2}, l)
	return nil // success
}

// utility function called from all of the InduceList methods.
func mapList(l []NI) (sub map[NI]NI, sup []NI) {
	sub = map[NI]NI{}
	// one pass to collect unique NIs
	for _, p := range l {
		sub[NI(p)] = -1
	}
	if len(sub) == len(l) { // NIs in l are unique
		sup = append([]NI{}, l...) // just copy them
		for b, p := range l {
			sub[p] = NI(b) // and fill in map
		}
	} else { // NIs in l not unique
		sup = make([]NI, 0, len(sub))
		for _, p := range l { // preserve ordering of first occurrences in l
			if sub[p] < 0 {
				sub[p] = NI(len(sup))
				sup = append(sup, p)
			}
		}
	}
	return
}

// utility function called from all of the InduceBits methods.
func mapBits(t bits.Bits) (sub map[NI]NI, sup []NI) {
	sup = make([]NI, 0, t.OnesCount())
	sub = make(map[NI]NI, cap(sup))
	t.IterateOnes(func(n int) bool {
		sub[NI(n)] = NI(len(sup))
		sup = append(sup, NI(n))
		return true
	})
	return
}

// OrderMap formats maps for testable examples.
//
// OrderMap provides simple, no-frills formatting of maps in sorted order,
// convenient in some cases for output of testable examples.
func OrderMap(m interface{}) string {
	// in particular exclude slices, which template would happily accept but
	// which would probably represent a coding mistake
	if reflect.TypeOf(m).Kind() != reflect.Map {
		panic("not a map")
	}
	t := template.Must(template.New("").Parse(
		`map[{{range $k, $v := .}}{{$k}}:{{$v}} {{end}}]`))
	var b bytes.Buffer
	if err := t.Execute(&b, m); err != nil {
		panic(err)
	}
	return b.String()
}
