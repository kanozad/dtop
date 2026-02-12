package process

import (
	"fmt"
	"syscall"
)

const (
	minNice = -20
	maxNice = 19
)

func sendSignal(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	return syscall.Kill(pid, sig)
}

func reniceProcess(pid int, currentNice int, delta int) (int, error) {
	if pid <= 0 {
		return currentNice, fmt.Errorf("invalid pid")
	}
	newNice := currentNice + delta
	if newNice < minNice {
		newNice = minNice
	}
	if newNice > maxNice {
		newNice = maxNice
	}
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, newNice); err != nil {
		return currentNice, err
	}
	return newNice, nil
}
