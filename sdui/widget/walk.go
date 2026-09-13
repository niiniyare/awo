package widget

// WalkFunc is called for each node during a Walk traversal.
// Return false to stop traversal of the current node's subtree (children
// will not be visited). Return true to continue.
type WalkFunc func(n *Node) bool

// Walk performs a depth-first pre-order traversal of the widget tree rooted
// at n, calling fn for each non-nil node. If fn returns false for a node,
// that node's children are not visited. Walk does not visit nil nodes.
//
// Walk is safe for concurrent reads when the tree is not being modified.
func Walk(n *Node, fn WalkFunc) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for _, child := range n.Children {
		Walk(child, fn)
	}
}

// WalkAll traverses every node in the tree, ignoring the return value of fn.
// Use Walk when you need to prune subtrees; use WalkAll when you need to visit
// every node unconditionally.
func WalkAll(n *Node, fn func(n *Node)) {
	Walk(n, func(node *Node) bool {
		fn(node)
		return true
	})
}

// Collect returns all nodes in the tree (depth-first pre-order) for which
// predicate returns true.
func Collect(n *Node, predicate func(n *Node) bool) []*Node {
	var result []*Node
	WalkAll(n, func(node *Node) {
		if predicate(node) {
			result = append(result, node)
		}
	})
	return result
}

// FindByID returns the first node with the given ID, or nil if not found.
// ID comparison is exact (case-sensitive).
func FindByID(root *Node, id string) *Node {
	var found *Node
	Walk(root, func(n *Node) bool {
		if n.ID == id {
			found = n
			return false // stop traversal
		}
		return true
	})
	return found
}

// CountNodes returns the total number of nodes in the tree rooted at n.
func CountNodes(n *Node) int {
	count := 0
	WalkAll(n, func(_ *Node) { count++ })
	return count
}
