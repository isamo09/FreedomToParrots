//go:build !linux && !windows

package procio

const supported = false

func read(int) (int64, bool) {
	return 0, false
}
