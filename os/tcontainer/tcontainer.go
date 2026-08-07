// Package tcontainer 提供泛型容器数据结构。
// 设计要点：
//   - 基于 Go 泛型，零反射、零装箱。
//   - 提供 Safe 和 Unsafe 两种模式（通过 mutex 内部可选加锁）。
//   - 提供 Safe 和 Unsafe 两种模式（通过 mutex 内部可选加锁）。
package tcontainer
