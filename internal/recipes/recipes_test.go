package recipes

import "testing"

func TestLoadStarterRecipes(t *testing.T) {
	s, errs := Load("../../recipes")
	for _, e := range errs {
		t.Errorf("parse error: %v", e)
	}
	if len(s.All()) == 0 {
		t.Fatal("expected at least one recipe to load")
	}
	r, ok := s.BySlug("tomato-basil-pasta")
	if !ok {
		t.Fatal("expected tomato-basil-pasta to be loadable by slug")
	}
	if r.Category != "dinner" {
		t.Errorf("category = %q, want dinner", r.Category)
	}
	if len(r.Ingredients) == 0 {
		t.Error("expected ingredients to be parsed")
	}
	if r.InstructionsHTML == "" {
		t.Error("expected instructions to render to HTML")
	}
}

func TestFilterIncludeExclude(t *testing.T) {
	s, errs := Load("../../recipes")
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}

	matches := s.Filter(FilterOpts{Query: "tomato basil pasta", Include: []string{"garlic"}, Limit: 100})
	if len(matches) != 1 || matches[0].Slug != "tomato-basil-pasta" {
		t.Errorf("include filter = %v, want [tomato-basil-pasta]", matches)
	}

	matches = s.Filter(FilterOpts{Category: "breakfast", Limit: 100})
	for _, m := range matches {
		if m.Category != "breakfast" {
			t.Errorf("got category %q in breakfast filter", m.Category)
		}
	}

	matches = s.Filter(FilterOpts{Exclude: []string{"garlic"}, Limit: 100})
	for _, m := range matches {
		if m.Slug == "tomato-basil-pasta" {
			t.Error("exclude filter should have removed tomato-basil-pasta")
		}
	}
}

func TestParseQuantity(t *testing.T) {
	cases := map[string]struct {
		want float64
		ok   bool
	}{
		"2":        {2, true},
		"1/2":      {0.5, true},
		"to taste": {0, false},
		"":         {0, false},
		"  1.5 ":   {1.5, true},
	}
	for in, want := range cases {
		got, ok := ParseQuantity(in)
		if ok != want.ok || (ok && got != want.want) {
			t.Errorf("ParseQuantity(%q) = (%v, %v), want (%v, %v)", in, got, ok, want.want, want.ok)
		}
	}
}
