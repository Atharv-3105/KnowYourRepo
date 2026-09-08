package answer

import "testing"

func TestWantsReingestion(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"what is the latest state of this repo?", true},
		{"has this changed recently?", true},
		{"is this up to date?", true},
		{"what does the setup function do?", false},
		{"", false},
	}

	for _, c := range cases {
		if got := WantsReingestion(c.query); got != c.want {
			t.Errorf("WantsReingestion(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}

func TestWantsArchitectureOverview(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"give me a high-level overview of this repo", true},
		{"what are the entrypoints?", true},
		{"what languages does this repository use?", true},
		{"what does the setup function do?", false},
		{"who calls configure?", false},
	}

	for _, c := range cases {
		if got := WantsArchitectureOverview(c.query); got != c.want {
			t.Errorf("WantsArchitectureOverview(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}
