package loop

import "testing"

func TestIsSystemProviderID(t *testing.T) {
	if !IsSystemProviderID("ensis-system") {
		t.Fatal("expected system id")
	}
	if IsSystemProviderID("openai") {
		t.Fatal("user provider")
	}
}
