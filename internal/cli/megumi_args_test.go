package cli

import "testing"

func TestStartsWithRun(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{
		{[]string{"run", "hi"}, true},
		{[]string{"r"}, true},
		{[]string{"--yolo", "run", "hi"}, true}, // leading flags are skipped
		{[]string{"hello"}, false},
		{[]string{"-y", "hello"}, false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := startsWithRun(tc.in); got != tc.want {
			t.Errorf("startsWithRun(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestStartsInteractiveSession(t *testing.T) {
	// After splitMegumiArgs, -p/--print have already been rewritten to `run`,
	// so the banner-gating check only needs to recognize the run subcommand.
	if startsInteractiveSession([]string{"run", "hi"}) {
		t.Error("a `run` one-shot must not be treated as interactive")
	}
	if !startsInteractiveSession([]string{"--continue"}) {
		t.Error("a plain session launch must be treated as interactive")
	}
}
