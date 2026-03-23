package scopes

import (
	"fmt"
	"sort"
	"strings"
)

type Scope string

type Info struct {
	Name        Scope
	Description string
}

const (
	ScopeClientIdentityRead  Scope = "client.identity.read"
	ScopeClientStatusRead    Scope = "client.status.read"
	ScopeServerRead          Scope = "server.read"
	ScopeUsersRead           Scope = "users.read"
	ScopeUsersCreate         Scope = "users.create"
	ScopeRoomsRead           Scope = "rooms.read"
	ScopeRoomsAliasRead      Scope = "rooms.alias.read"
	ScopeRoomsDirectoryRead  Scope = "rooms.directory.read"
	ScopeRoomsCreate         Scope = "rooms.create"
	ScopeRoomsAliasWrite     Scope = "rooms.alias.write"
	ScopeRoomsDirectoryWrite Scope = "rooms.directory.write"
	ScopeRoomsJoin           Scope = "rooms.join"
	ScopeRoomsInvite         Scope = "rooms.invite"
	ScopeRoomsLeave          Scope = "rooms.leave"
	ScopeRoomMembersRead     Scope = "room.members.read"
	ScopeRoomStateRead       Scope = "room.state.read"
	ScopeTimelineRead        Scope = "timeline.read"
	ScopeMessagesSend        Scope = "messages.send"
	ScopeMessagesReply       Scope = "messages.reply"
	ScopeMessagesEdit        Scope = "messages.edit"
	ScopeMessagesReact       Scope = "messages.react"
	ScopeMessagesRedact      Scope = "messages.redact"
)

var defaultScopes = []Scope{
	ScopeClientIdentityRead,
	ScopeClientStatusRead,
	ScopeServerRead,
	ScopeUsersRead,
	ScopeRoomsRead,
	ScopeRoomMembersRead,
	ScopeRoomStateRead,
	ScopeTimelineRead,
}

var allScopes = []Info{
	{Name: ScopeClientIdentityRead, Description: "Read the active Matrix client identity."},
	{Name: ScopeClientStatusRead, Description: "Read Matrix client readiness and login status."},
	{Name: ScopeServerRead, Description: "Read homeserver versions, capabilities, and feature metadata."},
	{Name: ScopeUsersRead, Description: "Search users, inspect profiles, and check username availability."},
	{Name: ScopeUsersCreate, Description: "Create new users through the homeserver registration API."},
	{Name: ScopeRoomsRead, Description: "List joined rooms and inspect or preview room summaries."},
	{Name: ScopeRoomsAliasRead, Description: "Resolve room aliases to room IDs and routing servers."},
	{Name: ScopeRoomsDirectoryRead, Description: "Inspect room-directory visibility for rooms."},
	{Name: ScopeRoomsCreate, Description: "Create new rooms."},
	{Name: ScopeRoomsAliasWrite, Description: "Create and delete room aliases."},
	{Name: ScopeRoomsDirectoryWrite, Description: "Publish and unpublish rooms in the room directory."},
	{Name: ScopeRoomsJoin, Description: "Join rooms by room ID or alias."},
	{Name: ScopeRoomsInvite, Description: "Invite users into existing rooms."},
	{Name: ScopeRoomsLeave, Description: "Leave rooms the active account is currently joined to."},
	{Name: ScopeRoomMembersRead, Description: "Read joined-member information for a room."},
	{Name: ScopeRoomStateRead, Description: "Read room state events."},
	{Name: ScopeTimelineRead, Description: "Read room timelines, events, context, and relations."},
	{Name: ScopeMessagesSend, Description: "Send new text or notice messages."},
	{Name: ScopeMessagesReply, Description: "Send reply messages."},
	{Name: ScopeMessagesEdit, Description: "Edit previously sent messages."},
	{Name: ScopeMessagesReact, Description: "Add reactions to events."},
	{Name: ScopeMessagesRedact, Description: "Redact events."},
}

type Set struct {
	allowed map[Scope]struct{}
}

func Default() Set {
	allowed := make(map[Scope]struct{}, len(defaultScopes))
	for _, scope := range defaultScopes {
		allowed[scope] = struct{}{}
	}
	return Set{allowed: allowed}
}

func Parse(raw string) (Set, error) {
	if strings.TrimSpace(raw) == "" {
		return Default(), nil
	}

	allowed := make(map[Scope]struct{})
	for _, token := range strings.Split(raw, ",") {
		name := strings.TrimSpace(token)
		if name == "" {
			continue
		}
		if name == "default" {
			for _, scope := range defaultScopes {
				allowed[scope] = struct{}{}
			}
			continue
		}
		scope := Scope(name)
		if !IsKnown(scope) {
			return Set{}, fmt.Errorf("unknown scope %q", name)
		}
		allowed[scope] = struct{}{}
	}
	if len(allowed) == 0 {
		return Default(), nil
	}
	return Set{allowed: allowed}, nil
}

func (s Set) Allows(scope Scope) bool {
	_, ok := s.allowed[scope]
	return ok
}

func (s Set) Names() []string {
	result := make([]string, 0, len(s.allowed))
	for scope := range s.allowed {
		result = append(result, string(scope))
	}
	sort.Strings(result)
	return result
}

func Available() []Info {
	result := make([]Info, len(allScopes))
	copy(result, allScopes)
	return result
}

func DefaultNames() []string {
	result := make([]string, 0, len(defaultScopes))
	for _, scope := range defaultScopes {
		result = append(result, string(scope))
	}
	return result
}

func IsKnown(scope Scope) bool {
	for _, info := range allScopes {
		if info.Name == scope {
			return true
		}
	}
	return false
}
