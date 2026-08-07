// Package tlb 提供负载均衡算法。
//
// 支持多种负载均衡策略：随机、轮询、加权轮询、最小连接、一致性哈希。
//
// 用法：
//
//	lb := tlb.NewRoundRobin[string]([]string{"node1", "node2", "node3"})
//	for i := 0; i < 10; i++ {
//	    node := lb.Next()
//	    fmt.Println(node)
//	}
package tlb

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
)

// Balancer 是负载均衡器接口。
type Balancer[T any] interface {
	// Next 返回下一个节点。
	Next() T
	// Add 添加节点。
	Add(item T)
	// Remove 移除节点。
	Remove(item T) bool
	// All 返回所有节点。
	All() []T
	// Count 返回节点数量。
	Count() int
}

// ---- 随机 ----

// Random 随机选择均衡器。
type Random[T comparable] struct {
	mu    sync.RWMutex
	items []T
	index map[T]int
	rng   *rand.Rand
}

// NewRandom 创建随机均衡器。
func NewRandom[T comparable](items []T) *Random[T] {
	lb := &Random[T]{index: make(map[T]int), rng: rand.New(rand.NewSource(rand.Int63()))}
	for _, item := range items {
		lb.Add(item)
	}
	return lb
}

func (r *Random[T]) Next() T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.items) == 0 {
		var zero T
		return zero
	}
	return r.items[r.rng.Intn(len(r.items))]
}

func (r *Random[T]) Add(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.index[item]; ok {
		return
	}
	r.index[item] = len(r.items)
	r.items = append(r.items, item)
}

func (r *Random[T]) Remove(item T) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx, ok := r.index[item]
	if !ok {
		return false
	}
	last := len(r.items) - 1
	if idx != last {
		r.items[idx] = r.items[last]
		r.index[r.items[idx]] = idx
	}
	r.items = r.items[:last]
	delete(r.index, item)
	return true
}

func (r *Random[T]) All() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]T, len(r.items))
	copy(result, r.items)
	return result
}

func (r *Random[T]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// ---- 轮询 ----

// RoundRobin 轮询均衡器。
type RoundRobin[T comparable] struct {
	mu      sync.RWMutex
	items   []T
	index   map[T]int
	counter atomic.Uint64
}

// NewRoundRobin 创建轮询均衡器。
func NewRoundRobin[T comparable](items []T) *RoundRobin[T] {
	lb := &RoundRobin[T]{index: make(map[T]int)}
	for _, item := range items {
		lb.Add(item)
	}
	return lb
}

func (rr *RoundRobin[T]) Next() T {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	if len(rr.items) == 0 {
		var zero T
		return zero
	}
	idx := rr.counter.Add(1) % uint64(len(rr.items))
	return rr.items[idx]
}

func (rr *RoundRobin[T]) Add(item T) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if _, ok := rr.index[item]; ok {
		return
	}
	rr.index[item] = len(rr.items)
	rr.items = append(rr.items, item)
}

func (rr *RoundRobin[T]) Remove(item T) bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	idx, ok := rr.index[item]
	if !ok {
		return false
	}
	last := len(rr.items) - 1
	if idx != last {
		rr.items[idx] = rr.items[last]
		rr.index[rr.items[idx]] = idx
	}
	rr.items = rr.items[:last]
	delete(rr.index, item)
	return true
}

func (rr *RoundRobin[T]) All() []T {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	result := make([]T, len(rr.items))
	copy(result, rr.items)
	return result
}

func (rr *RoundRobin[T]) Count() int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return len(rr.items)
}

// ---- 加权轮询 ----

// Weighted 加权节点。
type Weighted[T any] struct {
	Item   T
	Weight int
}

// WeightedRoundRobin 加权轮询均衡器。
type WeightedRoundRobin[T comparable] struct {
	mu         sync.RWMutex
	items      []T
	index      map[T]int
	weights    map[T]int
	initWeight map[T]int // 保存初始权重，用于耗尽后重置
}

// NewWeightedRoundRobin 创建加权轮询均衡器。
func NewWeightedRoundRobin[T comparable](items []Weighted[T]) *WeightedRoundRobin[T] {
	lb := &WeightedRoundRobin[T]{
		index:      make(map[T]int),
		weights:    make(map[T]int),
		initWeight: make(map[T]int),
	}
	for _, w := range items {
		lb.Add(w.Item)
		lb.SetWeight(w.Item, w.Weight)
	}
	return lb
}

func (wr *WeightedRoundRobin[T]) Next() T {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	if len(wr.items) == 0 {
		var zero T
		return zero
	}

	// 选择权重最大的节点
	best := wr.items[0]
	bestW := wr.weights[best]
	for _, item := range wr.items[1:] {
		if w := wr.weights[item]; w > bestW {
			bestW = w
			best = item
		}
	}

	// 降低选中节点权重
	wr.weights[best]--

	// 如果所有权重降到 0，重置为初始权重
	totalW := 0
	for _, item := range wr.items {
		totalW += wr.weights[item]
	}
	if totalW <= 0 {
		for _, item := range wr.items {
			wr.weights[item] = wr.initWeight[item]
		}
	}

	return best
}

func (wr *WeightedRoundRobin[T]) Add(item T) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	if _, ok := wr.index[item]; ok {
		return
	}
	wr.index[item] = len(wr.items)
	wr.items = append(wr.items, item)
	wr.weights[item] = 1
	wr.initWeight[item] = 1
}

func (wr *WeightedRoundRobin[T]) Remove(item T) bool {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	idx, ok := wr.index[item]
	if !ok {
		return false
	}
	last := len(wr.items) - 1
	if idx != last {
		wr.items[idx] = wr.items[last]
		wr.index[wr.items[idx]] = idx
	}
	wr.items = wr.items[:last]
	delete(wr.index, item)
	delete(wr.weights, item)
	return true
}

func (wr *WeightedRoundRobin[T]) All() []T {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	result := make([]T, len(wr.items))
	copy(result, wr.items)
	return result
}

func (wr *WeightedRoundRobin[T]) Count() int {
	wr.mu.RLock()
	defer wr.mu.RUnlock()
	return len(wr.items)
}

// SetWeight 更新节点权重，同时更新初始权重（用于耗尽后重置）。
func (wr *WeightedRoundRobin[T]) SetWeight(item T, weight int) {
	wr.mu.Lock()
	defer wr.mu.Unlock()
	wr.weights[item] = weight
	wr.initWeight[item] = weight
}

// ---- 最小连接 ----

// LeastConnection 最小连接均衡器。
type LeastConnection[T comparable] struct {
	mu          sync.RWMutex
	items       []T
	index       map[T]int
	connections map[T]*atomic.Int64
}

// NewLeastConnection 创建最小连接均衡器。
func NewLeastConnection[T comparable](items []T) *LeastConnection[T] {
	lc := &LeastConnection[T]{
		index:       make(map[T]int),
		connections: make(map[T]*atomic.Int64),
	}
	for _, item := range items {
		lc.Add(item)
	}
	return lc
}

func (lc *LeastConnection[T]) Next() T {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if len(lc.items) == 0 {
		var zero T
		return zero
	}

	best := lc.items[0]
	minConn := lc.connections[best].Load()
	for _, item := range lc.items[1:] {
		conn := lc.connections[item].Load()
		if conn < minConn {
			minConn = conn
			best = item
		}
	}
	return best
}

func (lc *LeastConnection[T]) Add(item T) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if _, ok := lc.index[item]; ok {
		return
	}
	lc.index[item] = len(lc.items)
	lc.items = append(lc.items, item)
	lc.connections[item] = &atomic.Int64{}
}

func (lc *LeastConnection[T]) Remove(item T) bool {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	idx, ok := lc.index[item]
	if !ok {
		return false
	}
	last := len(lc.items) - 1
	if idx != last {
		lc.items[idx] = lc.items[last]
		lc.index[lc.items[idx]] = idx
	}
	lc.items = lc.items[:last]
	delete(lc.index, item)
	delete(lc.connections, item)
	return true
}

func (lc *LeastConnection[T]) All() []T {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	result := make([]T, len(lc.items))
	copy(result, lc.items)
	return result
}

func (lc *LeastConnection[T]) Count() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return len(lc.items)
}

// Inc 增加节点连接数。
func (lc *LeastConnection[T]) Inc(item T) {
	if c, ok := lc.connections[item]; ok {
		c.Add(1)
	}
}

// Dec 减少节点连接数。
func (lc *LeastConnection[T]) Dec(item T) {
	if c, ok := lc.connections[item]; ok {
		c.Add(-1)
	}
}

// ---- 一致性哈希 ----

// ConsistentHash 一致性哈希均衡器。
type ConsistentHash[T comparable] struct {
	mu       sync.RWMutex
	items    []T
	index    map[T]int
	replicas int
	rr       uint64 // 无 key 时的轮询兜底计数器（原子访问）
	keys     []uint32
	hashMap  map[uint32]T
}

// NewConsistentHash 创建一致性哈希均衡器。
func NewConsistentHash[T comparable](items []T, replicas int) *ConsistentHash[T] {
	if replicas <= 0 {
		replicas = 100
	}
	ch := &ConsistentHash[T]{
		index:    make(map[T]int),
		replicas: replicas,
		hashMap:  make(map[uint32]T),
	}
	for _, item := range items {
		ch.Add(item)
	}
	return ch
}

func (ch *ConsistentHash[T]) Next() T {
	ch.mu.RLock()
	n := len(ch.items)
	ch.mu.RUnlock()
	if n == 0 {
		var zero T
		return zero
	}
	// 一致性哈希无显式 key 时，按轮询兜底选一个节点，避免恒定返回零值。
	idx := atomic.AddUint64(&ch.rr, 1) % uint64(n)
	return ch.items[idx]
}

// NextByKey 根据 key 选择节点。
func (ch *ConsistentHash[T]) NextByKey(key string) T {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.keys) == 0 {
		var zero T
		return zero
	}

	hash := hash32(key)
	idx := searchUint32(ch.keys, hash)
	return ch.hashMap[ch.keys[idx]]
}

func (ch *ConsistentHash[T]) Add(item T) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if _, ok := ch.index[item]; ok {
		return
	}
	ch.index[item] = len(ch.items)
	ch.items = append(ch.items, item)
	for i := 0; i < ch.replicas; i++ {
		h := hash32(fmt.Sprintf("%v:%d", item, i))
		ch.keys = append(ch.keys, h)
		ch.hashMap[h] = item
	}
	sortUint32(ch.keys)
}

func (ch *ConsistentHash[T]) Remove(item T) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	idx, ok := ch.index[item]
	if !ok {
		return false
	}
	last := len(ch.items) - 1
	if idx != last {
		ch.items[idx] = ch.items[last]
		ch.index[ch.items[idx]] = idx
	}
	ch.items = ch.items[:last]
	delete(ch.index, item)

	for i := 0; i < ch.replicas; i++ {
		h := hash32(fmt.Sprintf("%v:%d", item, i))
		delete(ch.hashMap, h)
		ch.keys = removeUint32(ch.keys, h)
	}
	sortUint32(ch.keys)
	return true
}

func (ch *ConsistentHash[T]) All() []T {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	result := make([]T, len(ch.items))
	copy(result, ch.items)
	return result
}

func (ch *ConsistentHash[T]) Count() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.items)
}

// ---- 哈希与排序工具 ----

func hash32(key string) uint32 {
	var h uint32 = 0
	for _, c := range []byte(key) {
		h = h*31 + uint32(c)
	}
	return h
}

func sortUint32(s []uint32) {
	// 插入排序
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func removeUint32(s []uint32, val uint32) []uint32 {
	for i, v := range s {
		if v == val {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func searchUint32(s []uint32, target uint32) int {
	low, high := 0, len(s)-1
	for low <= high {
		mid := (low + high) / 2
		if s[mid] < target {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if low >= len(s) {
		return 0
	}
	return low
}
