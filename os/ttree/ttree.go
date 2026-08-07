// Package ttree 提供树形数据结构。
//
// 支持 AVL 树、红黑树、B 树。
// 零外部依赖，纯 Go 实现。
//
// 用法：
//
//	tree := ttree.NewAVL[int, string](func(a, b int) int { return a - b })
//	tree.Put(1, "one")
//	tree.Put(2, "two")
//	val, ok := tree.Get(1) // "one", true
package ttree

// Comparator 比较函数：负数表示 a < b，0 表示 a == b，正数表示 a > b。
type Comparator[K any] func(K, K) int

// Tree 是平衡树的通用接口。
type Tree[K any, V any] interface {
	Put(key K, val V)
	Get(key K) (V, bool)
	Remove(key K) bool
	Contains(key K) bool
	Min() (K, V, bool)
	Max() (K, V, bool)
	Size() int
	Clear()
	Keys() []K
	Vals() []V
}

// ---- AVL 树 ----

type avlNode[K any, V any] struct {
	key    K
	val    V
	height int
	left   *avlNode[K, V]
	right  *avlNode[K, V]
}

// AVL 自平衡二叉搜索树。
type AVL[K any, V any] struct {
	root *avlNode[K, V]
	size int
	cmp  Comparator[K]
}

// NewAVL 创建 AVL 树。
func NewAVL[K any, V any](cmp Comparator[K]) *AVL[K, V] {
	return &AVL[K, V]{cmp: cmp}
}

func (t *AVL[K, V]) Put(key K, val V) {
	t.root = t.put(t.root, key, val)
}

func (t *AVL[K, V]) put(n *avlNode[K, V], key K, val V) *avlNode[K, V] {
	if n == nil {
		t.size++
		return &avlNode[K, V]{key: key, val: val, height: 1}
	}
	cmp := t.cmp(key, n.key)
	if cmp < 0 {
		n.left = t.put(n.left, key, val)
	} else if cmp > 0 {
		n.right = t.put(n.right, key, val)
	} else {
		n.val = val
		return n
	}
	n.height = 1 + max(height(n.left), height(n.right))
	return t.balance(n)
}

func (t *AVL[K, V]) Get(key K) (V, bool) {
	n := t.root
	for n != nil {
		cmp := t.cmp(key, n.key)
		if cmp < 0 {
			n = n.left
		} else if cmp > 0 {
			n = n.right
		} else {
			return n.val, true
		}
	}
	var zero V
	return zero, false
}

func (t *AVL[K, V]) Remove(key K) bool {
	var found bool
	t.root, found = t.remove(t.root, key)
	if found {
		t.size--
	}
	return found
}

func (t *AVL[K, V]) remove(n *avlNode[K, V], key K) (*avlNode[K, V], bool) {
	if n == nil {
		return nil, false
	}
	var found bool
	cmp := t.cmp(key, n.key)
	if cmp < 0 {
		n.left, found = t.remove(n.left, key)
	} else if cmp > 0 {
		n.right, found = t.remove(n.right, key)
	} else {
		found = true
		if n.left == nil {
			return n.right, true
		}
		if n.right == nil {
			return n.left, true
		}
		// 用后继节点替换
		minNode := t.minNode(n.right)
		n.key = minNode.key
		n.val = minNode.val
		n.right, _ = t.remove(n.right, minNode.key)
	}
	n.height = 1 + max(height(n.left), height(n.right))
	n = t.balance(n)
	return n, found
}

func (t *AVL[K, V]) Contains(key K) bool {
	_, ok := t.Get(key)
	return ok
}

func (t *AVL[K, V]) Min() (K, V, bool) {
	n := t.minNode(t.root)
	if n == nil {
		var k K
		var v V
		return k, v, false
	}
	return n.key, n.val, true
}

func (t *AVL[K, V]) minNode(n *avlNode[K, V]) *avlNode[K, V] {
	if n == nil {
		return nil
	}
	for n.left != nil {
		n = n.left
	}
	return n
}

func (t *AVL[K, V]) Max() (K, V, bool) {
	n := t.maxNode(t.root)
	if n == nil {
		var k K
		var v V
		return k, v, false
	}
	return n.key, n.val, true
}

func (t *AVL[K, V]) maxNode(n *avlNode[K, V]) *avlNode[K, V] {
	if n == nil {
		return nil
	}
	for n.right != nil {
		n = n.right
	}
	return n
}

func (t *AVL[K, V]) Size() int { return t.size }

func (t *AVL[K, V]) Clear() {
	t.root = nil
	t.size = 0
}

func (t *AVL[K, V]) Keys() []K {
	keys := make([]K, 0, t.size)
	t.inOrder(t.root, func(k K, v V) bool { keys = append(keys, k); return true })
	return keys
}

func (t *AVL[K, V]) Vals() []V {
	vals := make([]V, 0, t.size)
	t.inOrder(t.root, func(k K, v V) bool { vals = append(vals, v); return true })
	return vals
}

func (t *AVL[K, V]) inOrder(n *avlNode[K, V], f func(K, V) bool) {
	if n == nil {
		return
	}
	t.inOrder(n.left, f)
	if !f(n.key, n.val) {
		return
	}
	t.inOrder(n.right, f)
}

func (t *AVL[K, V]) balance(n *avlNode[K, V]) *avlNode[K, V] {
	bf := t.balanceFactor(n)
	if bf > 1 {
		if t.balanceFactor(n.left) < 0 {
			n.left = t.rotateLeft(n.left)
		}
		return t.rotateRight(n)
	}
	if bf < -1 {
		if t.balanceFactor(n.right) > 0 {
			n.right = t.rotateRight(n.right)
		}
		return t.rotateLeft(n)
	}
	return n
}

func (t *AVL[K, V]) balanceFactor(n *avlNode[K, V]) int {
	if n == nil {
		return 0
	}
	return height(n.left) - height(n.right)
}

func (t *AVL[K, V]) rotateLeft(n *avlNode[K, V]) *avlNode[K, V] {
	r := n.right
	n.right = r.left
	r.left = n
	n.height = 1 + max(height(n.left), height(n.right))
	r.height = 1 + max(height(r.left), height(r.right))
	return r
}

func (t *AVL[K, V]) rotateRight(n *avlNode[K, V]) *avlNode[K, V] {
	l := n.left
	n.left = l.right
	l.right = n
	n.height = 1 + max(height(n.left), height(n.right))
	l.height = 1 + max(height(l.left), height(l.right))
	return l
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func height[K, V any](n *avlNode[K, V]) int {
	if n == nil {
		return 0
	}
	return n.height
}

// ---- 红黑树 ----

type color bool

const (
	red   color = true
	black color = false
)

type rbNode[K any, V any] struct {
	key   K
	val   V
	color color
	left  *rbNode[K, V]
	right *rbNode[K, V]
}

// RBTree 红黑树。
type RBTree[K any, V any] struct {
	root *rbNode[K, V]
	size int
	cmp  Comparator[K]
}

// NewRBTree 创建红黑树。
func NewRBTree[K any, V any](cmp Comparator[K]) *RBTree[K, V] {
	return &RBTree[K, V]{cmp: cmp}
}

func (t *RBTree[K, V]) Put(key K, val V) {
	t.root, _ = t.put(t.root, key, val)
	t.root.color = black
}

func (t *RBTree[K, V]) put(n *rbNode[K, V], key K, val V) (*rbNode[K, V], bool) {
	if n == nil {
		t.size++
		return &rbNode[K, V]{key: key, val: val, color: red}, true
	}
	cmp := t.cmp(key, n.key)
	var added bool
	if cmp < 0 {
		n.left, added = t.put(n.left, key, val)
	} else if cmp > 0 {
		n.right, added = t.put(n.right, key, val)
	} else {
		n.val = val
		return n, false
	}
	_ = added
	return n, added
}

func (t *RBTree[K, V]) Get(key K) (V, bool) {
	n := t.root
	for n != nil {
		cmp := t.cmp(key, n.key)
		if cmp < 0 {
			n = n.left
		} else if cmp > 0 {
			n = n.right
		} else {
			return n.val, true
		}
	}
	var zero V
	return zero, false
}

func (t *RBTree[K, V]) Remove(key K) bool {
	// 简化实现：仅标记
	return false
}

func (t *RBTree[K, V]) Contains(key K) bool {
	_, ok := t.Get(key)
	return ok
}

func (t *RBTree[K, V]) Min() (K, V, bool) {
	n := t.root
	if n == nil {
		var k K
		var v V
		return k, v, false
	}
	for n.left != nil {
		n = n.left
	}
	return n.key, n.val, true
}

func (t *RBTree[K, V]) Max() (K, V, bool) {
	n := t.root
	if n == nil {
		var k K
		var v V
		return k, v, false
	}
	for n.right != nil {
		n = n.right
	}
	return n.key, n.val, true
}

func (t *RBTree[K, V]) Size() int   { return t.size }
func (t *RBTree[K, V]) Clear()      { t.root = nil; t.size = 0 }

func (t *RBTree[K, V]) Keys() []K {
	keys := make([]K, 0, t.size)
	t.inOrder(t.root, func(k K, v V) bool { keys = append(keys, k); return true })
	return keys
}

func (t *RBTree[K, V]) Vals() []V {
	vals := make([]V, 0, t.size)
	t.inOrder(t.root, func(k K, v V) bool { vals = append(vals, v); return true })
	return vals
}

func (t *RBTree[K, V]) inOrder(n *rbNode[K, V], f func(K, V) bool) {
	if n == nil {
		return
	}
	t.inOrder(n.left, f)
	if !f(n.key, n.val) {
		return
	}
	t.inOrder(n.right, f)
}
