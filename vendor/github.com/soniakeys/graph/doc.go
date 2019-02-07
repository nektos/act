// Copyright 2014 Sonia Keys
// License MIT: http://opensource.org/licenses/MIT

// Graph algorithms: Dijkstra, A*, Bellman Ford, Floyd Warshall;
// Kruskal and Prim minimal spanning tree; topological sort and DAG longest
// and shortest paths; Eulerian cycle and path; degeneracy and k-cores;
// Bron Kerbosch clique finding; connected components; dominance; and others.
//
// This is a graph library of integer indexes.  To use it with application
// data, you associate data with integer indexes, perform searches or other
// operations with the library, and then use the integer index results to refer
// back to your application data.
//
// Thus it does not store application data, pointers to application data,
// or require you to implement an interface on your application data.
// The idea is to keep the library methods fast and lean.
//
// Representation overview
//
// The package defines a type for a node index (NI) which is just an integer
// type.  It defines types for a number of number graph representations using
// NI.  The fundamental graph type is AdjacencyList, which is the
// common "list of lists" graph representation.  It is a list as a slice
// with one element for each node of the graph.  Each element is a list
// itself, a list of neighbor nodes, implemented as an NI slice.  Methods
// on an AdjacencyList generally work on any representable graph, including
// directed or undirected graphs, simple graphs or multigraphs.
//
// The type Undirected embeds an AdjacencyList adding methods specific to
// undirected graphs.  Similarly the type Directed adds methods meaningful
// for directed graphs.
//
// Similar to NI, the type LI is a "label index" which labels a
// node-to-neighbor "arc" or edge.  Just as an NI can index arbitrary node
// data, an LI can index arbitrary arc or edge data.  A number of algorithms
// use a "weight" associated with an arc.  This package does not represent
// weighted arcs explicitly, but instead uses the LI as a more general
// mechanism allowing not only weights but arbitrary data to be associated
// with arcs.  While AdjacencyList represents an arc with simply an NI,
// the type LabeledAdjacencyList uses a type that pairs an NI with an LI.
// This type is named Half, for half-arc.  (A full arc would represent
// both ends.)  Types LabeledDirected and LabeledUndirected embed a
// LabeledAdjacencyList.
//
// In contrast to Half, the type Edge represents both ends of an edge (but
// no label.)  The type LabeledEdge adds the label.  The type WeightedEdgeList
// bundles a list of LabeledEdges with a WeightFunc.  (WeightedEdgeList has
// few methods.  It exists primarily to support the Kruskal algorithm.)
//
// FromList is a compact rooted tree (or forest) respresentation.  Like
// AdjacencyList and LabeledAdjacencyList, it is a list with one element for
// each node of the graph.  Each element contains only a single neighbor
// however, its parent in the tree, the "from" node.
//
// Code generation
//
// A number of methods on AdjacencyList, Directed, and Undirected are
// applicable to LabeledAdjacencyList, LabeledDirected, and LabeledUndirected
// simply by ignoring the label.  In these cases code generation provides
// methods on both types from a single source implementation. These methods
// are documented with the sentence "There are equivalent labeled and unlabeled
// versions of this method."
//
// Terminology
//
// This package uses the term "node" rather than "vertex."  It uses "arc"
// to mean a directed edge, and uses "from" and "to" to refer to the ends
// of an arc.  It uses "start" and "end" to refer to endpoints of a search
// or traversal.
//
// The usage of "to" and "from" is perhaps most strange.  In common speech
// they are prepositions, but throughout this package they are used as
// adjectives, for example to refer to the "from node" of an arc or the
// "to node".  The type "FromList" is named to indicate it stores a list of
// "from" values.
//
// A "half arc" refers to just one end of an arc, either the to or from end.
//
// Two arcs are "reciprocal" if they connect two distinct nodes n1 and n2,
// one arc leading from n1 to n2 and the other arc leading from n2 to n1.
// Undirected graphs are represented with reciprocal arcs.
//
// A node with an arc to itself represents a "loop."  Duplicate arcs, where
// a node has multiple arcs to another node, are termed "parallel arcs."
// A graph with no loops or parallel arcs is "simple."  A graph that allows
// parallel arcs is a "multigraph"
//
// The "size" of a graph traditionally means the number of undirected edges.
// This package uses "arc size" to mean the number of arcs in a graph.  For an
// undirected graph without loops, arc size is 2 * size.
//
// The "order" of a graph is the number of nodes.  An "ordering" though means
// an ordered list of nodes.
//
// A number of graph search algorithms use a concept of arc "weights."
// The sum of arc weights along a path is a "distance."  In contrast, the
// number of nodes in a path, including start and end nodes, is the path's
// "length."  (Yes, mixing weights and lengths would be nonsense physically,
// but the terms used here are just distinct terms for abstract values.
// The actual meaning to an application is likely to be something else
// entirely and is not relevant within this package.)
//
// Finally, this package documentation takes back the word "object" in some
// places to refer to a Go value, especially a value of a type with methods.
//
// Shortest path searches
//
// This package implements a number of shortest path searches.  Most work
// with weighted graphs that are directed or undirected, and with graphs
// that may have loops or parallel arcs.  For weighted graphs, "shortest"
// is defined as the path distance (sum of arc weights) with path length
// (number of nodes) breaking ties.  If multiple paths have the same minimum
// distance with the same minimum length, search methods are free to return
// any of them.
//
//  Algorithm      Description
//  Dijkstra       Non-negative arc weights, single or all paths.
//  AStar          Non-negative arc weights, heuristic guided, single path.
//  BellmanFord    Negative arc weights allowed, no negative cycles, all paths.
//  DAGPath        O(n) algorithm for DAGs, arc weights of any sign.
//  FloydWarshall  all pairs distances, no negative cycles.
package graph
