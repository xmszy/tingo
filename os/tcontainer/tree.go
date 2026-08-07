package tcontainer

// Tree 泛型二叉树。基于红黑树实现，键值有序。
type Tree[K, V any] struct {
	less   func(a, b K) bool
	root   *treeNode[K, V]
	length int
}

type treeNode[K, V any] struct {
	key         K
	value       V
	color       bool // true=red, false=black
	left, right *treeNode[K, V]
}

// NewTree 创建红黑树。
func NewTree[K, V any](less func(a, b K) bool) *Tree[K, V] {
	return &Tree[K, V]{less: less}
}

// Set 插入键值。
func (t *Tree[K, V]) Set(key K, value V) {
	t.root = t.insert(t.root, key, value)
	t.root.color = false // 根始终黑
}

// Get 获取键值。
func (t *Tree[K, V]) Get(key K) (V, bool) {
	node := t.find(t.root, key)
	if node == nil {
		var zero V
		return zero, false
	}
	return node.value, true
}

// MustGet 获取键值，不存在返回零值。
func (t *Tree[K, V]) MustGet(key K) V {
	v, _ := t.Get(key)
	return v
}

// Remove 删除键。
func (t *Tree[K, V]) Remove(key K) {
	if t.find(t.root, key) == nil {
		return
	}
	t.root = t.remove(t.root, key)
	if t.root != nil {
		t.root.color = false
	}
	t.length--
}

// Has 判断键是否存在。
func (t *Tree[K, V]) Has(key K) bool {
	_, ok := t.Get(key)
	return ok
}

// Len 返回节点数。
func (t *Tree[K, V]) Len() int { return t.length }

// Range 中序遍历。
func (t *Tree[K, V]) Range(fn func(key K, value V) bool) {
	t.rangeNode(t.root, fn)
}

// Keys 返回所有键（升序）。
func (t *Tree[K, V]) Keys() []K {
	keys := make([]K, 0, t.length)
	t.Range(func(k K, _ V) bool {
		keys = append(keys, k)
		return true
	})
	return keys
}

// Values 返回所有值（按键升序）。
func (t *Tree[K, V]) Values() []V {
	vals := make([]V, 0, t.length)
	t.Range(func(_ K, v V) bool {
		vals = append(vals, v)
		return true
	})
	return vals
}

// Min 返回最小键值。
func (t *Tree[K, V]) Min() (K, V, bool) {
	if t.root == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	n := t.root
	for n.left != nil {
		n = n.left
	}
	return n.key, n.value, true
}

// Max 返回最大键值。
func (t *Tree[K, V]) Max() (K, V, bool) {
	if t.root == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	n := t.root
	for n.right != nil {
		n = n.right
	}
	return n.key, n.value, true
}

// 左旋
func (t *Tree[K, V]) rotateLeft(h *treeNode[K, V]) *treeNode[K, V] {
	x := h.right
	h.right = x.left
	x.left = h
	x.color = h.color
	h.color = true
	return x
}

// 右旋
func (t *Tree[K, V]) rotateRight(h *treeNode[K, V]) *treeNode[K, V] {
	x := h.left
	h.left = x.right
	x.right = h
	x.color = h.color
	h.color = true
	return x
}

// 翻转颜色
func flipColors[K, V any](h *treeNode[K, V]) {
	h.color = !h.color
	h.left.color = !h.left.color
	h.right.color = !h.right.color
}

// 是否为红色节点
func isRed[K, V any](n *treeNode[K, V]) bool {
	return n != nil && n.color
}

// 插入
func (t *Tree[K, V]) insert(h *treeNode[K, V], key K, value V) *treeNode[K, V] {
	if h == nil {
		t.length++
		return &treeNode[K, V]{key: key, value: value, color: true}
	}
	if t.less(key, h.key) {
		h.left = t.insert(h.left, key, value)
	} else if t.less(h.key, key) {
		h.right = t.insert(h.right, key, value)
	} else {
		h.value = value // 更新
		return h
	}
	if isRed(h.right) && !isRed(h.left) {
		h = t.rotateLeft(h)
	}
	if isRed(h.left) && isRed(h.left.left) {
		h = t.rotateRight(h)
	}
	if isRed(h.left) && isRed(h.right) {
		flipColors(h)
	}
	return h
}

// 查找
func (t *Tree[K, V]) find(h *treeNode[K, V], key K) *treeNode[K, V] {
	if h == nil {
		return nil
	}
	if t.less(key, h.key) {
		return t.find(h.left, key)
	}
	if t.less(h.key, key) {
		return t.find(h.right, key)
	}
	return h
}

// 删除（简化实现：标记惰性删除）
func (t *Tree[K, V]) remove(h *treeNode[K, V], key K) *treeNode[K, V] {
	if h == nil {
		return nil
	}
	if t.less(key, h.key) {
		h.left = t.remove(h.left, key)
	} else if t.less(h.key, key) {
		h.right = t.remove(h.right, key)
	} else {
		if h.left == nil {
			return h.right
		}
		if h.right == nil {
			return h.left
		}
		// 用右子树最小节点替换
		min := h.right
		for min.left != nil {
			min = min.left
		}
		h.key = min.key
		h.value = min.value
		h.right = t.remove(h.right, min.key)
	}
	return h
}

// 中序遍历
func (t *Tree[K, V]) rangeNode(n *treeNode[K, V], fn func(K, V) bool) bool {
	if n == nil {
		return true
	}
	if !t.rangeNode(n.left, fn) {
		return false
	}
	if !fn(n.key, n.value) {
		return false
	}
	return t.rangeNode(n.right, fn)
}
