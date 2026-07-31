// Package execpipe runs an *exec.Cmd while capturing stdout/stderr through
// manually-owned pipes, so Wait returns as soon as the direct child exits
// rather than blocking until every holder of those file descriptors closes
// them.
//
// os/exec ties Cmd.Wait to pipe EOF whenever Stdout/Stderr are set to a
// non-*os.File Writer: Wait will not return until every process holding the
// write end has closed it, including a daemon the child forks into the
// background that inherits the fd across fork/exec (this is what
// wl-copy does — the child that runs holds the clipboard selection
// forever). Passing *os.File pipe ends instead bypasses that: exec wires
// the fd directly into the child, so Wait never depends on the pipe being
// drained. Reads happen concurrently so a chatty child's own output is
// still captured in full; only a lingering forked-off holder is bounded,
// by a grace period after the direct child exits.
package execpipe

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"time"
)

// Run starts cmd with its Stdout and Stderr connected to fresh pipes, waits
// for it to exit, and returns whatever was written to each stream.
// stdoutLimit caps how many bytes of stdout are kept (0 means unlimited);
// stderr is never capped, since callers only use it for short diagnostic
// messages.
//
// grace bounds how long Run keeps reading after the direct child exits. It
// only matters when something the child forked off is still holding a
// pipe open — the direct child's own output is always captured in full,
// since reads run concurrently with the process rather than starting
// after it exits.
func Run(cmd *exec.Cmd, grace time.Duration, stdoutLimit int64) (stdout, stderr []byte, err error) {
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		stdoutR.Close()
		stdoutW.Close()
		return nil, nil, err
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	if err := cmd.Start(); err != nil {
		stdoutR.Close()
		stdoutW.Close()
		stderrR.Close()
		stderrW.Close()
		return nil, nil, err
	}
	// Our own copies of the write ends; the child (and anything it forks)
	// keeps them open until it's done with them.
	stdoutW.Close()
	stderrW.Close()

	stdoutDone := readAsync(stdoutR, stdoutLimit)
	stderrDone := readAsync(stderrR, 0)

	runErr := cmd.Wait()

	stdout = collect(stdoutR, stdoutDone, grace)
	stderr = collect(stderrR, stderrDone, grace)
	stdoutR.Close()
	stderrR.Close()

	return stdout, stderr, runErr
}

// readAsync drains f continuously until EOF or a forced read error,
// keeping only the first limit bytes (0 means unlimited) and discarding
// the rest. Draining unconditionally, rather than stopping once limit is
// reached, matters: the process on the other end of the pipe can still be
// writing, and a pipe nobody reads from fills up and blocks that write —
// which would keep the process running and Wait from returning.
func readAsync(f *os.File, limit int64) <-chan []byte {
	done := make(chan []byte, 1)
	go func() {
		sink := &capBuffer{limit: limit}
		io.Copy(sink, f)
		done <- sink.buf.Bytes()
	}()
	return done
}

// capBuffer is an io.Writer that keeps only the first limit bytes it sees
// (0 means unlimited), while still reporting every byte as written so its
// caller (io.Copy) never stops draining early.
type capBuffer struct {
	buf   bytes.Buffer
	limit int64
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.buf.Write(p)
		return len(p), nil
	}
	if room := b.limit - int64(b.buf.Len()); room > 0 {
		if room > int64(len(p)) {
			room = int64(len(p))
		}
		b.buf.Write(p[:room])
	}
	return len(p), nil
}

// collect waits for f's reader goroutine to finish, or forces it to stop
// after grace. The direct child has already exited by the time this runs
// (Run calls it after cmd.Wait returns); anything still unread past the
// grace period belongs to a process the child forked that inherited the
// write end and will never close it, so there is nothing more to wait for.
func collect(f *os.File, done <-chan []byte, grace time.Duration) []byte {
	select {
	case out := <-done:
		return out
	case <-time.After(grace):
		f.SetReadDeadline(time.Now())
		return <-done
	}
}
