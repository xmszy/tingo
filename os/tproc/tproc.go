// Package tproc 提供进程管理。
// 设计要点：
//   - 基于标准库 os/exec/os/signal，零外部依赖。
//   - 提供 PID、命令行参数、环境变量、信号监听等工具。
package tproc

import (
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

// PID 返回当前进程 ID。
func PID() int { return os.Getpid() }

// PPID 返回父进程 ID。
func PPID() int { return os.Getppid() }

// Args 返回命令行参数。
func Args() []string { return os.Args }

// Cwd 返回当前工作目录。
func Cwd() string { wd, _ := os.Getwd(); return wd }

// Executable 返回可执行文件路径。
func Executable() string { p, _ := os.Executable(); return p }

// Env 获取环境变量。
func Env(key string, def ...string) string {
	v, ok := os.LookupEnv(key)
	if !ok && len(def) > 0 {
		return def[0]
	}
	return v
}

// EnvAll 返回所有环境变量。
func EnvAll() []string { return os.Environ() }

// SetEnv 设置环境变量。
func SetEnv(key, value string) error { return os.Setenv(key, value) }

// UnsetEnv 删除环境变量。
func UnsetEnv(key string) error { return os.Unsetenv(key) }

// Signal 监听信号。
func Signal(sigs ...os.Signal) <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	return ch
}

// Run 执行外部命令，返回输出。
func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

// Shell 通过 shell 执行命令。
func Shell(cmd string) (string, error) {
	return Run("cmd", "/C", cmd)
}

// Kill 向进程发送信号。
func Kill(pid int, sig os.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(sig)
}

// Exit 退出进程。
func Exit(code int) { os.Exit(code) }
