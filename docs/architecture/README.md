# Architecture Documentation

This directory contains technical specifications, algorithm designs, and architectural decisions for the lldiscovery project.

## Documents

### Network Segment Detection

**[NETWORK_SEGMENTS.md](NETWORK_SEGMENTS.md)**
- Overview of network segment detection feature
- Use cases and benefits
- High-level algorithm description

**[NETWORK_SEGMENTS_DESIGN.md](NETWORK_SEGMENTS_DESIGN.md)**
- Detailed design document for segment detection
- Requirements and constraints
- Implementation approach

**[ALGORITHM_SUMMARY.md](ALGORITHM_SUMMARY.md)**
- Summary of the connected components algorithm
- BFS-based segment discovery
- Filtering and validation logic

### Clique Detection

**[CLIQUE_DETECTION.md](CLIQUE_DETECTION.md)**
- Bron-Kerbosch algorithm for maximal clique detection
- Why cliques represent network segments
- Algorithm complexity and optimization

**[CLIQUE_SUMMARY.md](CLIQUE_SUMMARY.md)**
- Historical context: transition from clique to connected components
- Comparison of approaches
- Lessons learned

## Key Concepts

### Network Segments
A network segment (VLAN/switch) is represented as a group of 3+ nodes that are all mutually reachable on the same interface. The algorithm uses:
- **Connected Components**: BFS to find reachable node groups
- **Interface-based**: Groups nodes by which local interface reaches them
- **Transitive Trust**: Works with incomplete edge visibility

### Algorithm Evolution
The project evolved through several iterations:
1. **Initial**: Simple connected components
2. **Clique-based**: Bron-Kerbosch for strict mutual connectivity
3. **Final**: Connected components with clique verification (balanced approach)

## Related Documentation

- **Features**: [../features/](../features/) - Feature implementations
- **Fixes**: [../fixes/](../fixes/) - Bug fixes and corrections
- **Development**: [../development/](../development/) - Development notes
