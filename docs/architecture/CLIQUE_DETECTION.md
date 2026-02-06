# Clique-Based Network Segment Detection

## Overview

lldiscovery uses **maximal clique detection** to identify network segments (switches/VLANs) in the discovered topology. This document explains the algorithm, why it's necessary, and how it works.

## Why Cliques?

### Physical Reality of Network Segments

A network segment (switch/VLAN) has a key property: **all-to-all connectivity**. If hosts A, B, and C are connected to the same switch:
- A can reach B directly
- B can reach C directly
- **A can reach C directly** (through the switch)

This is the definition of a **clique** in graph theory: a complete subgraph where every node connects to every other node.

### Why Not Connected Components?

A **connected component** is any set of nodes with paths between them. This is too broad:

```
Linear chain: A → B → C
- A and C are in the same connected component
- But A cannot reach C directly (no direct edge)
- This is NOT a network segment!

Clique: A ↔ B ↔ C (with A ↔ C)
- All pairs directly connected
- This IS a network segment (switch)
```

With transitive discovery (neighbor sharing), connected components would incorrectly merge distinct switches connected through a gateway.

## Algorithm: Bron-Kerbosch with Pivoting

### Basic Concept

The Bron-Kerbosch algorithm recursively explores the graph to find all maximal cliques:
- **R**: Nodes in the current clique being built
- **P**: Candidate nodes that can extend the clique
- **X**: Nodes already processed (prevents duplicates)

### Pseudocode

```
BronKerbosch(R, P, X):
  if P and X are both empty:
    report R as a maximal clique (if |R| ≥ 3)
  
  choose pivot u from P ∪ X with most neighbors in P
  
  for each vertex v in P \ N(u):
    BronKerbosch(
      R ∪ {v},
      P ∩ N(v),
      X ∩ N(v)
    )
    P := P \ {v}
    X := X ∪ {v}
```

### Pivoting Optimization

The pivot u is chosen to have the most connections in P. This optimization:
- Reduces the number of recursive calls
- Skips vertices that can't extend the clique
- Dramatically improves performance on dense graphs

### Complexity

- **Without pivoting**: O(3^(n/3)) - exponential
- **With pivoting**: O(d * n * 3^(d/3)) where d is degeneracy
- **In practice**: Very fast for network topologies (sparse graphs)

## Implementation Details

### Input: Connectivity Graph

We build a bidirectional adjacency map of `node:interface` pairs:

```
connectivity["hostA:eth0"] = {"hostB:eth0", "hostC:eth0"}
connectivity["hostB:eth0"] = {"hostA:eth0", "hostC:eth0"}
connectivity["hostC:eth0"] = {"hostA:eth0", "hostB:eth0"}
```

This represents that hostA, hostB, and hostC are mutually connected on their eth0 interfaces.

### Output: Maximal Cliques

Each maximal clique with 3+ unique nodes becomes a network segment:

```
Clique: ["hostA:eth0", "hostB:eth0", "hostC:eth0"]
→ Segment with 3 nodes: hostA, hostB, hostC
→ Label: "eth0"
```

### Handling Mixed Interfaces

If nodes use different interface names but are mutually connected:

```
Clique: ["hostA:em1", "hostB:br112", "hostC:eth0"]
→ Segment with 3 nodes: hostA, hostB, hostC
→ Label: "br112+em1+eth0" or "mixed(3)"
```

## Examples

### Example 1: Simple Triangle

```
Edges:
  A:eth0 ↔ B:eth0
  B:eth0 ↔ C:eth0
  A:eth0 ↔ C:eth0

Clique found: {A:eth0, B:eth0, C:eth0}
Segment: 3 nodes (A, B, C) on eth0
```

### Example 2: Overlapping Segments

```
Edges:
  A:eth0 ↔ B:eth0, A:eth0 ↔ C:eth0, B:eth0 ↔ C:eth0  (clique on eth0)
  A:eth1 ↔ D:eth1, A:eth1 ↔ E:eth1, D:eth1 ↔ E:eth1  (clique on eth1)

Cliques found:
  1. {A:eth0, B:eth0, C:eth0}
  2. {A:eth1, D:eth1, E:eth1}

Segments:
  - Segment 0: 3 nodes (A, B, C) on eth0
  - Segment 1: 3 nodes (A, D, E) on eth1
  
Note: A participates in both segments using different interfaces
```

### Example 3: Not a Segment

```
Edges:
  A:eth0 ↔ B:eth0
  B:eth0 ↔ C:eth0
  (A:eth0 ↔ C:eth0 MISSING!)

No clique found (only pairs)
No segment detected

This correctly represents a linear topology, not a shared switch.
```

### Example 4: Large Heterogeneous Segment

```
8 hosts all mutually connected through a 10G switch:
  - Host A uses em1
  - Hosts B, C use br112
  - Hosts D, E use eth0
  - Hosts F, G, H use p2p1

All 28 edges exist (8 choose 2 = 28)

Clique found: {A:em1, B:br112, C:br112, D:eth0, E:eth0, F:p2p1, G:p2p1, H:p2p1}
Segment: 8 nodes on "mixed(4)" interfaces
```

## Benefits

1. **Accurate Detection**: Only detects actual network segments (switches)
2. **No False Positives**: Linear chains and trees don't create segments
3. **Handles Heterogeneity**: Mixed interface names are grouped correctly
4. **Overlapping Support**: Nodes can be in multiple segments
5. **Works with Transitive Discovery**: Indirect edges contribute to cliques
6. **Efficient**: Pivoting makes it practical even for large topologies

## Comparison with Previous Approaches

### Version 1: Interface Name Grouping (Original)

```
Detection: Group by interface name
Problem: 9 segments for one switch with mixed interfaces
Example: em1 segment, br112 segment, eth0 segment (all same switch!)
```

### Version 2: Connected Components (BFS)

```
Detection: Find reachable nodes with BFS
Problem: Merges distinct switches connected through gateway
Example: Switch1 (A-B-C) + Gateway (C-D) + Switch2 (D-E-F) → ONE component
```

### Version 3: Maximal Cliques (Bron-Kerbosch) ✅

```
Detection: Find complete subgraphs
Benefit: Only actual network segments detected
Example: Switch1 clique (A,B,C), Switch2 clique (D,E,F), no merge
```

## Testing

The test suite creates proper cliques using transitive discovery edges:

```go
// Direct edges from A to B, C, D
g.AddOrUpdate("B", ..., "eth0")
g.AddOrUpdate("C", ..., "eth0")
g.AddOrUpdate("D", ..., "eth0")

// Indirect edges to complete the clique
g.AddOrUpdateIndirectEdge("C", ..., learned from "B")  // B→C
g.AddOrUpdateIndirectEdge("D", ..., learned from "B")  // B→D
g.AddOrUpdateIndirectEdge("B", ..., learned from "C")  // C→B
g.AddOrUpdateIndirectEdge("D", ..., learned from "C")  // C→D
g.AddOrUpdateIndirectEdge("B", ..., learned from "D")  // D→B
g.AddOrUpdateIndirectEdge("C", ..., learned from "D")  // D→C

// Result: Complete clique {A,B,C,D}
```

## Future Optimizations

1. **Degeneracy Ordering**: Pre-order vertices by degree to improve pivot selection
2. **Pruning**: Skip cliques already covered by larger cliques
3. **Incremental Updates**: Maintain cliques as topology changes
4. **Parallel Processing**: Process independent subgraphs in parallel

## References

- Bron, C. and Kerbosch, J. (1973). "Algorithm 457: finding all cliques of an undirected graph"
- Eppstein, D. et al. (2010). "Listing All Maximal Cliques in Sparse Graphs in Near-optimal Time"
- Tomita, E. et al. (2006). "The worst-case time complexity for generating all maximal cliques and computational experiments"

## See Also

- [NETWORK_SEGMENTS.md](NETWORK_SEGMENTS.md) - User-facing documentation
- [TRANSITIVE_DISCOVERY.md](TRANSITIVE_DISCOVERY.md) - How indirect edges are discovered
- [SUBGRAPH_VISUALIZATION.md](SUBGRAPH_VISUALIZATION.md) - How segments are visualized
