// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// maxShownLine is the most characters of a program's last line or a wait's last try that a progress line shows.
const maxShownLine = 160

// shownAs returns program shown under its step as line.
func (p Program) shownAs(line string) Program {
	p.Shown = line
	return p
}

// shownLine returns the line program is shown by under its step, its own name and arguments unless it has one.
func (p Program) shownLine() string {
	if p.Shown != "" {
		return p.Shown
	}
	return strings.Join(append([]string{filepath.Base(p.Name)}, p.Args...), " ")
}

// runNamed prints the line program is shown by, then runs it, printing a still-running line every progress interval.
func (r *installing) runNamed(ctx context.Context, program Program) (string, error) {
	shown := program.shownLine()
	if _, err := fmt.Fprintf(r.call.Stdout, "  %s\n", shown); err != nil {
		return "", err
	}
	seen := &tail{}
	program.Output = seen
	var printed string
	var err error
	r.watched("still running "+shown, 0, seen.last, func() { printed, err = r.machine.Run(ctx, program) })
	return printed, err
}

// watched does work, printing a progress line that says doing and what last returns every interval before limit.
func (r *installing) watched(doing string, limit time.Duration, last func() string, work func()) {
	interval := r.site.settings.Checks.ProgressInterval
	if interval == 0 {
		work()
		return
	}
	done := make(chan struct{})
	var watching sync.WaitGroup
	watching.Go(func() { r.watch(progress{doing: doing, limit: limit, last: last, interval: interval}, done) })
	work()
	close(done)
	watching.Wait()
}

// progress is what a watch prints: what it says it is doing, the time it stops at, its last text and how often.
type progress struct {
	doing    string
	limit    time.Duration
	last     func() string
	interval time.Duration
}

// watch prints a progress line for shown every interval until done closes, none at or past a limit above 0.
func (r *installing) watch(shown progress, done <-chan struct{}) {
	started := time.Now()
	ticker := time.NewTicker(shown.interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			elapsed := time.Since(started).Round(time.Second)
			if shown.limit > 0 && elapsed >= shown.limit {
				continue
			}
			_, _ = fmt.Fprintln(r.call.Stdout, shown.line(elapsed))
		}
	}
}

// line returns the progress line that says what p is doing after elapsed, then its last text when it holds any.
func (p progress) line(elapsed time.Duration) string {
	line := "  " + p.doing + " after " + elapsed.String()
	if last := p.last(); last != "" {
		return line + ": " + last
	}
	return line
}

// lastTry is the error the last try of a wait ended with, kept while another goroutine reads it.
type lastTry struct {
	mu  sync.Mutex
	err error
}

// kept returns try, keeping the error each of its tries ends with for last to show.
func (l *lastTry) kept(try func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		err := try(ctx)
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = err
		return err
	}
}

// last returns the error the last try ended with as a still-waiting line shows it, empty while it has none.
func (l *lastTry) last() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err == nil {
		return ""
	}
	return shownText(l.err.Error())
}

// tail is what a program printed so far, kept while another goroutine reads its last line.
type tail struct {
	mu      sync.Mutex
	printed []byte
}

// Write keeps p after what the program printed before it.
func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.printed = append(t.printed, p...)
	return len(p), nil
}

// last returns the last line the program printed that holds more than spaces, as a still-running line shows it.
func (t *tail) last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return lastLine(string(t.printed))
}

// lastLine returns the last line of printed that holds more than spaces, a carriage return ending a line too, as shown.
func lastLine(printed string) string {
	lines := strings.FieldsFunc(printed, func(r rune) bool { return r == '\n' || r == '\r' })
	for _, line := range slices.Backward(lines) {
		if shown := shownText(line); shown != "" {
			return shown
		}
	}
	return ""
}

// shownText returns text as a progress line shows it, each control character a space, trimmed and cut.
func shownText(text string) string {
	return cut(strings.TrimSpace(strings.Map(spaced, text)))
}

// spaced returns r, or a space in place of a control character.
func spaced(r rune) rune {
	if unicode.IsControl(r) {
		return ' '
	}
	return r
}

// cut returns line, or its first maxShownLine characters followed by ... when it holds more.
func cut(line string) string {
	runes := []rune(line)
	if len(runes) <= maxShownLine {
		return line
	}
	return string(runes[:maxShownLine]) + "..."
}
