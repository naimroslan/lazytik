package termcap

import "testing"

func TestDetectFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Graphics
	}{
		{"kitty term", map[string]string{"TERM": "xterm-kitty"}, Kitty},
		{"ghostty term", map[string]string{"TERM": "xterm-ghostty"}, Kitty},
		{"kitty window id", map[string]string{"KITTY_WINDOW_ID": "1"}, Kitty},
		{"wezterm", map[string]string{"WEZTERM_PANE": "0"}, Kitty},
		{"iterm", map[string]string{"TERM_PROGRAM": "iTerm.app"}, Sixel},
		{"foot", map[string]string{"TERM": "foot"}, Sixel},
		{"plain xterm", map[string]string{"TERM": "xterm-256color"}, None},
		{"override wins", map[string]string{"TERM": "xterm-kitty", "LAZYTIK_GRAPHICS": "none"}, None},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Clear the vars we care about, then set the case's.
			for _, k := range []string{"LAZYTIK_GRAPHICS", "TERM", "TERM_PROGRAM", "KITTY_WINDOW_ID", "WEZTERM_PANE"} {
				t.Setenv(k, "")
			}
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			if got := Detect(); got != c.want {
				t.Errorf("Detect()=%v want %v", got, c.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	for in, want := range map[string]Graphics{"kitty": Kitty, "sixel": Sixel, "none": None, "halfblock": None} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q)=%v,%v want %v,true", in, got, ok, want)
		}
	}
	if _, ok := Parse("bogus"); ok {
		t.Error("Parse(bogus) should fail")
	}
}
