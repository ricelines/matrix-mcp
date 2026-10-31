package modules

import (
	"testing"

	matrixclient "github.com/ricelines/matrix-mcp/internal/matrix"
)

func TestFilterRooms(t *testing.T) {
	rooms := []matrixclient.RoomSummary{
		{RoomID: "!a:example.com", DisplayName: "Mom (WA)"},
		{RoomID: "!b:example.com", Name: "Family chat", DisplayName: "Family chat"},
		{RoomID: "!c:example.com", CanonicalAlias: "#ops:example.com", DisplayName: "#ops:example.com"},
		{RoomID: "!d:example.com", Topic: "weekend plans"},
	}

	cases := []struct {
		query string
		want  []string
	}{
		{"", []string{"!a:example.com", "!b:example.com", "!c:example.com", "!d:example.com"}},
		{"mom", []string{"!a:example.com"}},
		{"  FAMILY ", []string{"!b:example.com"}},
		{"#ops", []string{"!c:example.com"}},
		{"weekend", []string{"!d:example.com"}},
		{"!b:", []string{"!b:example.com"}},
		{"nobody", nil},
	}
	for _, tc := range cases {
		got := filterRooms(rooms, tc.query)
		if len(got) != len(tc.want) {
			t.Fatalf("filterRooms(%q) returned %d rooms, want %d", tc.query, len(got), len(tc.want))
		}
		for i, room := range got {
			if room.RoomID != tc.want[i] {
				t.Fatalf("filterRooms(%q)[%d] = %s, want %s", tc.query, i, room.RoomID, tc.want[i])
			}
		}
	}
}
