package common

import "testing"

func TestIsHostPlatform(t *testing.T) {
	host := []string{
		"-self-hosted",
		"-SELF-HOSTED",
		"-Self-Hosted",
		"-self-HOSTED",
		" -self-hosted",
		"-self-hosted ",
		"\t-SELF-HOSTED\r\n",
		"\u00a0-SELF-HOSTED\u00a0",
		"-\u017felf-hosted",
		"-self-ho\u017fted",
		"-\u017fELF-ho\u017fTED",
	}
	for _, image := range host {
		if !IsHostPlatform(image) {
			t.Errorf("IsHostPlatform(%q) = false, want true", image)
		}
	}

	other := []string{
		"",
		"self-hosted",
		"-self-hosted-extra",
		"-self-hosted=more",
		"node:16-buster-slim",
		"-\uff53elf-hosted",
		"-self\u2010hosted",
		"-SELF-HOSTED\u200b",
		"\ufeff-SELF-HOSTED",
	}
	for _, image := range other {
		if IsHostPlatform(image) {
			t.Errorf("IsHostPlatform(%q) = true, want false", image)
		}
	}
}
