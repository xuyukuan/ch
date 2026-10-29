package ui

import "testing"

func TestNormalizedOCRLanguage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "eng"},
		{"   ", "eng"},
		{"eng", "eng"},
		{" chi_sim ", "chi_sim"},
		{"eng+chi_sim", "eng+chi_sim"},
	}

	for _, c := range cases {
		if got := normalizedOCRLanguage(c.in); got != c.want {
			t.Errorf("normalizedOCRLanguage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLatinOnlyOCR(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"eng", true},
		{"ENG", true},
		{"eng+eng", true},
		{"chi_sim", false},
		{"eng+chi_sim", false},
		{"jpn", false},
		{"deu", false},
		{"osd", false},
	}

	for _, c := range cases {
		if got := latinOnlyOCR(c.in); got != c.want {
			t.Errorf("latinOnlyOCR(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
