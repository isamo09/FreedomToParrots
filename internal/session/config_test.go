package session

import "testing"

func TestNormalizeRoom(t *testing.T) {
	cases := []struct {
		provider, raw, want string
	}{
		{"telemost", "https://telemost.yandex.ru/j/68715972174269", "68715972174269"},
		{"telemost", "68715972174269", "68715972174269"},
		{"wbstream", "https://stream.wb.ru/room/abc-123", "abc-123"},
		{"wbstream", "abc-123", "abc-123"},
		{"jitsi", "https://meet.example.org/myroom?x=1#y", "https://meet.example.org/myroom"},
		{"jitsi", "https://meet.example.org/myroom/", "https://meet.example.org/myroom"},
		{"telemost", "", ""},
	}

	for _, c := range cases {
		if got := normalizeRoom(c.provider, c.raw); got != c.want {
			t.Errorf("normalizeRoom(%q, %q) = %q, want %q", c.provider, c.raw, got, c.want)
		}
	}
}

func TestCleanFieldsRejectsTelemostDatachannel(t *testing.T) {
	_, err := cleanFields(Fields{Name: "x", Provider: "telemost", Transport: "datachannel", Room: "1"}, Fields{})
	if err == nil {
		t.Fatal("expected an error for telemost+datachannel, got nil")
	}
}

func TestCleanFieldsFillsFromExisting(t *testing.T) {
	existing := Fields{Name: "old", Provider: "jitsi", Transport: "datachannel", Room: "https://meet.example.org/r"}
	got, err := cleanFields(Fields{LimitB: 42}, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "old" || got.Provider != "jitsi" || got.Room != "https://meet.example.org/r" || got.LimitB != 42 {
		t.Errorf("got %+v", got)
	}
}

func TestBackoffForCapsAtMax(t *testing.T) {
	if got := backoffFor(1); got.Seconds() != 5 {
		t.Errorf("backoffFor(1) = %v, want 5s", got)
	}

	if got := backoffFor(100); got != maxBackoff {
		t.Errorf("backoffFor(100) = %v, want %v", got, maxBackoff)
	}
}
