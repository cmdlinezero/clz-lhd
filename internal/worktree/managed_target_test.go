package worktree

import "testing"

func TestManagedTargetPort(t *testing.T) {
	cases := []struct {
		target string
		want   int
	}{
		{"http://127.0.0.1:20000", 20000},
		{"http://127.0.0.1:49999", 49999},
		{"http://localhost:20000", 0},
		{"http://127.0.0.1:19999", 0},
		{"http://127.0.0.1:50000", 0},
		{"https://127.0.0.1:20000", 0},
		{"http://127.0.0.1:20000/extra", 0},
		{"http://127.0.0.1:20000?token=secret", 0},
		{"http://user@127.0.0.1:20000", 0},
		{"http://127.0.0.1:020000", 0},
		{"http://127.0.0.1:20000#fragment", 0},
	}
	for _, tc := range cases {
		got, ok := ManagedTargetPort(tc.target)
		if (tc.want != 0) != ok || got != tc.want {
			t.Errorf("ManagedTargetPort(%q) = (%d, %v); want (%d, %v)", tc.target, got, ok, tc.want, tc.want != 0)
		}
	}
}
