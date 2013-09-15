package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func fmtBytes(n int64) string {
	l := make([]string, 0, 4)
	pairs := []struct {
		s string
		n int64
	}{
		{"G", 1 << 30},
		{"M", 1 << 20},
		{"K", 1 << 10},
		{"B", 1},
	}

	for _, p := range pairs {
		if n >= p.n {
			l = append(l, fmt.Sprintf("%d%s", n/p.n, p.s))
			n %= p.n
		}
	}

	if len(l) > 0 {
		return strings.Join(l, ", ")
	}
	return "0B"
}

func exit(code int, out string, dur time.Duration, bytes int64) {
	tm := "0ms"
	if dur > 0 {
		dur += time.Millisecond - (dur % time.Millisecond)
		tm = dur.String()
	}

	fmt.Fprintln(os.Stderr)
	if out != "" {
		fmt.Fprintln(os.Stderr, out)
	}
	fmt.Fprintln(os.Stderr, tm, fmtBytes(bytes))
	os.Exit(code)
}

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		exit(0, "", 0, 0)
	}

	fn, err := exec.LookPath(args[0])
	if err != nil {
		exit(127, args[0]+": command not found", 0, 0)
	}

	cmd := exec.Command(fn, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		exit(1, err.Error(), 0, 0)
	}

	// start timing after the command starts to elliminate out own(Go's) overhead
	start := time.Now()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch)

	go func() {
		for sig := range ch {
			if i, ok := sig.(syscall.Signal); ok {
				switch i {
				case syscall.SIGTSTP:
					syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
				default:
					cmd.Process.Signal(sig)
				}
			}
		}
	}()

	ps, err := cmd.Process.Wait()
	dur := time.Since(start)
	out := ""
	if err != nil {
		out = err.Error()
	}

	ws := ps.Sys().(syscall.WaitStatus)
	ru := ps.SysUsage().(*syscall.Rusage)
	bytes := ru.Maxrss * 1024

	exit(ws.ExitStatus(), out, dur, bytes)
}
