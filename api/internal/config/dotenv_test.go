package config

import "testing"

func TestParseDotEnvValue(t *testing.T) {
	cases := map[string]string{
		`text`:                          "text",
		`  text   # json in production`: "text",
		"8080\t# port":                  "8080",
		`"query: "`:                     "query: ",
		`"has # inside" # comment`:      "has # inside",
		`'single'`:                      "single",
		`postgres://u:p@h/db?x=1#frag`:  "postgres://u:p@h/db?x=1#frag",
		``:                              "",
	}
	for in, want := range cases {
		if got := parseDotEnvValue(in); got != want {
			t.Errorf("parseDotEnvValue(%q) = %q, want %q", in, got, want)
		}
	}
}
