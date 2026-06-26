package deps

import "testing"

func TestPackageManagerCommand(t *testing.T) {
	cases := []struct {
		mgr  PackageManager
		want string
	}{
		{
			PackageManager{"apt", "apt-get", true, []string{"install", "-y"}},
			"sudo apt-get install -y mpv yt-dlp",
		},
		{
			PackageManager{"pacman", "pacman", true, []string{"-S", "--needed", "--noconfirm"}},
			"sudo pacman -S --needed --noconfirm mpv yt-dlp",
		},
		{
			PackageManager{"brew", "brew", false, []string{"install"}},
			"brew install mpv yt-dlp",
		},
	}
	for _, c := range cases {
		got := c.mgr.CommandString([]string{"mpv", "yt-dlp"})
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.mgr.Name, got, c.want)
		}
	}
}

func TestNonSudoManagerOmitsSudo(t *testing.T) {
	brew := PackageManager{"brew", "brew", false, []string{"install"}}
	if got := brew.Command([]string{"mpv"}); got[0] == "sudo" {
		t.Errorf("brew command should not start with sudo: %v", got)
	}
}

func TestInstallHintEmptyWhenNothingMissing(t *testing.T) {
	if got := InstallHint(nil); got != "" {
		t.Errorf("InstallHint(nil) = %q, want empty", got)
	}
}
