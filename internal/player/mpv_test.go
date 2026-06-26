package player

import "testing"

func TestPauseCommand(t *testing.T) {
	cases := map[bool]string{
		true:  `{"command":["set_property","pause",true]}` + "\n",
		false: `{"command":["set_property","pause",false]}` + "\n",
	}
	for paused, want := range cases {
		if got := string(pauseCommand(paused)); got != want {
			t.Errorf("pauseCommand(%v) = %q, want %q", paused, got, want)
		}
	}
}

// SetPaused / Close must tolerate a nil receiver (no audio started).
func TestNilAudioSafe(t *testing.T) {
	var a *Audio
	a.SetPaused(true)
	a.Close()
}
