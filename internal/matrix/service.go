package matrix

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ricelines/matrix-mcp/internal/config"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const registrationTokenAuthType = mautrix.AuthType("m.login.registration_token")

const (
	RoomDirectoryVisibilityPrivate = "private"
	RoomDirectoryVisibilityPublic  = "public"
	loginRetryInterval             = 250 * time.Millisecond
	loginRetryTimeout              = 15 * time.Second
)

type Identity struct {
	UserID        string
	DeviceID      string
	HomeserverURL string
}

type VersionInfo struct {
	Versions []string
	Features []string
}

type SearchUser struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type UserProfile struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type CreateUserRequest struct {
	Username                 string
	Password                 string
	InitialDeviceDisplayName string
	InhibitLogin             bool
}

type CreateUserResult struct {
	UserID            string `json:"user_id"`
	DeviceID          string `json:"device_id,omitempty"`
	AccessToken       string `json:"access_token,omitempty"`
	Password          string `json:"password,omitempty"`
	PasswordGenerated bool   `json:"password_generated"`
}

type RoomSummary struct {
	RoomID           string   `json:"room_id,omitempty"`
	AvatarURL        string   `json:"avatar_url,omitempty"`
	CanonicalAlias   string   `json:"canonical_alias,omitempty"`
	GuestCanJoin     bool     `json:"guest_can_join"`
	JoinRule         string   `json:"join_rule,omitempty"`
	Name             string   `json:"name,omitempty"`
	NumJoinedMembers int      `json:"num_joined_members"`
	RoomType         string   `json:"room_type,omitempty"`
	Topic            string   `json:"topic,omitempty"`
	WorldReadable    bool     `json:"world_readable"`
	RoomVersion      string   `json:"room_version,omitempty"`
	Encryption       string   `json:"encryption,omitempty"`
	AllowedRoomIDs   []string `json:"allowed_room_ids,omitempty"`
	Membership       string   `json:"membership,omitempty"`
}

type MemberInfo struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type EventSummary struct {
	EventID     string         `json:"event_id,omitempty"`
	RoomID      string         `json:"room_id,omitempty"`
	Sender      string         `json:"sender,omitempty"`
	Type        string         `json:"type"`
	StateKey    *string        `json:"state_key,omitempty"`
	TimestampMS int64          `json:"timestamp_ms,omitempty"`
	Content     map[string]any `json:"content,omitempty"`
	Redacts     string         `json:"redacts,omitempty"`
}

type CreateRoomRequest struct {
	Name     string
	Topic    string
	IsPublic bool
	Invite   []string
	IsDirect bool
}

type CreateRoomResult struct {
	RoomID string `json:"room_id"`
}

type JoinRoomRequest struct {
	RoomIDOrAlias string
	Via           []string
	Reason        string
}

type JoinRoomResult struct {
	RoomID string `json:"room_id"`
}

type InviteRoomMemberRequest struct {
	RoomID string
	UserID string
	Reason string
}

type InviteRoomMemberResult struct {
	RoomID string `json:"room_id"`
	UserID string `json:"user_id"`
}

type LeaveRoomRequest struct {
	RoomID string
	Reason string
}

type LeaveRoomResult struct {
	RoomID string `json:"room_id"`
}

type CreateRoomAliasRequest struct {
	RoomAlias string
	RoomID    string
}

type CreateRoomAliasResult struct {
	RoomAlias string `json:"room_alias"`
	RoomID    string `json:"room_id"`
}

type RoomAliasResult struct {
	RoomAlias string   `json:"room_alias"`
	RoomID    string   `json:"room_id"`
	Servers   []string `json:"servers,omitempty"`
}

type DeleteRoomAliasResult struct {
	RoomAlias string `json:"room_alias"`
}

type SetRoomDirectoryVisibilityRequest struct {
	RoomID     string
	Visibility string
}

type RoomDirectoryVisibilityResult struct {
	RoomID     string `json:"room_id"`
	Visibility string `json:"visibility"`
}

type roomDirectoryVisibilityPayload struct {
	Visibility string `json:"visibility"`
}

type SendTextRequest struct {
	RoomID string
	Body   string
	Notice bool
}

type ReplyTextRequest struct {
	RoomID  string
	EventID string
	Body    string
	Notice  bool
}

type EditTextRequest struct {
	RoomID  string
	EventID string
	Body    string
	Notice  bool
}

type ReactRequest struct {
	RoomID  string
	EventID string
	Key     string
}

type RedactRequest struct {
	RoomID  string
	EventID string
	Reason  string
}

type EventWriteResult struct {
	EventID string `json:"event_id"`
}

type ListMessagesRequest struct {
	RoomID    string
	From      string
	To        string
	Direction string
	Limit     int
}

type MessagesResult struct {
	Start  string         `json:"start,omitempty"`
	End    string         `json:"end,omitempty"`
	Events []EventSummary `json:"events"`
	State  []EventSummary `json:"state,omitempty"`
}

type EventContextResult struct {
	Event        EventSummary   `json:"event"`
	EventsBefore []EventSummary `json:"events_before"`
	EventsAfter  []EventSummary `json:"events_after"`
	State        []EventSummary `json:"state,omitempty"`
	Start        string         `json:"start,omitempty"`
	End          string         `json:"end,omitempty"`
}

type ListRelationsRequest struct {
	RoomID       string
	EventID      string
	RelationType string
	EventType    string
	Direction    string
	From         string
	To           string
	Limit        int
	Recurse      bool
}

type RelationsResult struct {
	Events         []EventSummary `json:"events"`
	NextBatch      string         `json:"next_batch,omitempty"`
	PrevBatch      string         `json:"prev_batch,omitempty"`
	RecursionDepth int            `json:"recursion_depth,omitempty"`
}

type API interface {
	Identity() Identity
	IsActive() bool
	Versions(context.Context) (VersionInfo, error)
	Capabilities(context.Context) (map[string]any, error)
	RegisterAvailable(context.Context, string) (bool, error)
	CreateUser(context.Context, CreateUserRequest) (CreateUserResult, error)
	SearchUsers(context.Context, string, int) ([]SearchUser, bool, error)
	GetProfile(context.Context, string) (UserProfile, error)
	ListRooms(context.Context) ([]RoomSummary, error)
	GetRoom(context.Context, string) (RoomSummary, error)
	PreviewRoom(context.Context, string, []string) (RoomSummary, error)
	ListRoomMembers(context.Context, string) ([]MemberInfo, error)
	GetRoomMember(context.Context, string, string) (MemberInfo, error)
	GetStateEvent(context.Context, string, string, string) (EventSummary, error)
	ListStateEvents(context.Context, string) ([]EventSummary, error)
	ListMessages(context.Context, ListMessagesRequest) (MessagesResult, error)
	GetEvent(context.Context, string, string) (EventSummary, error)
	GetEventContext(context.Context, string, string, int) (EventContextResult, error)
	ListRelations(context.Context, ListRelationsRequest) (RelationsResult, error)
	CreateRoom(context.Context, CreateRoomRequest) (CreateRoomResult, error)
	JoinRoom(context.Context, JoinRoomRequest) (JoinRoomResult, error)
	InviteRoomMember(context.Context, InviteRoomMemberRequest) (InviteRoomMemberResult, error)
	LeaveRoom(context.Context, LeaveRoomRequest) (LeaveRoomResult, error)
	CreateRoomAlias(context.Context, CreateRoomAliasRequest) (CreateRoomAliasResult, error)
	GetRoomAlias(context.Context, string) (RoomAliasResult, error)
	DeleteRoomAlias(context.Context, string) (DeleteRoomAliasResult, error)
	GetRoomDirectoryVisibility(context.Context, string) (RoomDirectoryVisibilityResult, error)
	SetRoomDirectoryVisibility(context.Context, SetRoomDirectoryVisibilityRequest) (RoomDirectoryVisibilityResult, error)
	SendText(context.Context, SendTextRequest) (EventWriteResult, error)
	ReplyText(context.Context, ReplyTextRequest) (EventWriteResult, error)
	EditText(context.Context, EditTextRequest) (EventWriteResult, error)
	React(context.Context, ReactRequest) (EventWriteResult, error)
	Redact(context.Context, RedactRequest) (EventWriteResult, error)
}

type Service struct {
	client                *mautrix.Client
	homeserverURL         string
	registrationToken     string
	newRegistrationClient func(string) (*mautrix.Client, error)
}

type registrationTokenAuthData struct {
	mautrix.BaseAuthData
	Token string `json:"token"`
}

func New(ctx context.Context, cfg config.Config) (*Service, error) {
	client, err := mautrix.NewClient(cfg.HomeserverURL, "", "")
	if err != nil {
		return nil, fmt.Errorf("build matrix client: %w", err)
	}

	err = loginWithPassword(ctx, client, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("matrix login: %w", err)
	}

	return &Service{
		client:            client,
		homeserverURL:     cfg.HomeserverURL,
		registrationToken: cfg.RegistrationToken,
	}, nil
}

func loginWithPassword(ctx context.Context, client *mautrix.Client, username, password string) error {
	loginCtx, cancel := context.WithTimeout(ctx, loginRetryTimeout)
	defer cancel()

	for {
		_, err := client.Login(loginCtx, &mautrix.ReqLogin{
			Type: mautrix.AuthTypePassword,
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: username,
			},
			Password:         password,
			StoreCredentials: true,
		})
		if err == nil {
			return nil
		}
		if !isTransientLoginError(err) || loginCtx.Err() != nil {
			return err
		}

		select {
		case <-loginCtx.Done():
			return err
		case <-time.After(loginRetryInterval):
		}
	}
}

func isTransientLoginError(err error) bool {
	var httpErr mautrix.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Response != nil {
			if httpErr.Response.StatusCode >= http.StatusInternalServerError {
				return true
			}
			if httpErr.Response.StatusCode == http.StatusTooManyRequests {
				return true
			}
		}
		if httpErr.RespError != nil && httpErr.RespError.CanRetry {
			return true
		}
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

func (s *Service) Identity() Identity {
	return Identity{
		UserID:        s.client.UserID.String(),
		DeviceID:      s.client.DeviceID.String(),
		HomeserverURL: s.homeserverURL,
	}
}

func (s *Service) IsActive() bool {
	return s.client != nil && s.client.AccessToken != "" && s.client.UserID != ""
}

func (s *Service) Versions(ctx context.Context) (VersionInfo, error) {
	resp, err := s.client.Versions(ctx)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("get versions: %w", err)
	}

	versions := make([]string, 0, len(resp.Versions))
	for _, version := range resp.Versions {
		versions = append(versions, version.String())
	}
	sort.Strings(versions)

	features := make([]string, 0, len(resp.UnstableFeatures))
	for name, enabled := range resp.UnstableFeatures {
		if enabled {
			features = append(features, name)
		}
	}
	sort.Strings(features)

	return VersionInfo{Versions: versions, Features: features}, nil
}

func (s *Service) Capabilities(ctx context.Context) (map[string]any, error) {
	resp, err := s.client.Capabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("get capabilities: %w", err)
	}
	result, err := marshalMap(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal capabilities: %w", err)
	}
	return result, nil
}

func (s *Service) RegisterAvailable(ctx context.Context, username string) (bool, error) {
	resp, err := s.client.RegisterAvailable(ctx, username)
	if err != nil {
		if errors.Is(err, mautrix.MUserInUse) {
			return false, nil
		}
		return false, fmt.Errorf("check registration availability: %w", err)
	}
	return resp.Available, nil
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (CreateUserResult, error) {
	password := strings.TrimSpace(req.Password)
	generated := false
	if password == "" {
		password = randomPassword()
		generated = true
	}

	newClient := s.newRegistrationClient
	if newClient == nil {
		newClient = func(homeserverURL string) (*mautrix.Client, error) {
			return mautrix.NewClient(homeserverURL, "", "")
		}
	}
	client, err := newClient(s.homeserverURL)
	if err != nil {
		return CreateUserResult{}, fmt.Errorf("build registration client: %w", err)
	}

	registerReq := &mautrix.ReqRegister{
		Username:                 req.Username,
		Password:                 password,
		InitialDeviceDisplayName: req.InitialDeviceDisplayName,
		InhibitLogin:             req.InhibitLogin,
	}

	resp, uia, err := client.Register(ctx, registerReq)
	if err != nil && uia == nil {
		return CreateUserResult{}, fmt.Errorf("start register user: %w", err)
	}
	if resp == nil {
		auth, authErr := buildRegistrationAuth(uia, s.registrationToken)
		if authErr != nil {
			return CreateUserResult{}, authErr
		}
		registerReq.Auth = auth

		resp, uia, err = client.Register(ctx, registerReq)
		if err != nil && uia == nil {
			return CreateUserResult{}, fmt.Errorf("complete register user: %w", err)
		}
		if resp == nil {
			if uia != nil && uia.Error != "" {
				return CreateUserResult{}, fmt.Errorf("registration did not complete: %s", uia.Error)
			}
			return CreateUserResult{}, errors.New("registration did not complete")
		}
	}

	result := CreateUserResult{
		UserID:            resp.UserID.String(),
		DeviceID:          resp.DeviceID.String(),
		AccessToken:       resp.AccessToken,
		PasswordGenerated: generated,
	}
	if generated {
		result.Password = password
	}
	return result, nil
}

func (s *Service) SearchUsers(ctx context.Context, query string, limit int) ([]SearchUser, bool, error) {
	if limit <= 0 {
		limit = 10
	}
	resp, err := s.client.SearchUserDirectory(ctx, query, limit)
	if err != nil {
		return nil, false, fmt.Errorf("search users: %w", err)
	}

	results := make([]SearchUser, 0, len(resp.Results))
	for _, result := range resp.Results {
		if result == nil {
			continue
		}
		results = append(results, SearchUser{
			UserID:      result.UserID.String(),
			DisplayName: result.DisplayName,
			AvatarURL:   result.AvatarURL.String(),
		})
	}
	return results, resp.Limited, nil
}

func (s *Service) GetProfile(ctx context.Context, userID string) (UserProfile, error) {
	resp, err := s.client.GetProfile(ctx, id.UserID(userID))
	if err != nil {
		return UserProfile{}, fmt.Errorf("get profile: %w", err)
	}
	return UserProfile{
		UserID:      userID,
		DisplayName: resp.DisplayName,
		AvatarURL:   resp.AvatarURL.String(),
	}, nil
}

func (s *Service) ListRooms(ctx context.Context) ([]RoomSummary, error) {
	resp, err := s.client.JoinedRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("list joined rooms: %w", err)
	}

	rooms := make([]RoomSummary, 0, len(resp.JoinedRooms))
	for _, roomID := range resp.JoinedRooms {
		summary, err := s.client.GetRoomSummary(ctx, roomID.String())
		if err != nil {
			rooms = append(rooms, RoomSummary{RoomID: roomID.String()})
			continue
		}
		rooms = append(rooms, toRoomSummary(summary, roomID.String()))
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].RoomID < rooms[j].RoomID })
	return rooms, nil
}

func (s *Service) GetRoom(ctx context.Context, roomID string) (RoomSummary, error) {
	resp, err := s.client.GetRoomSummary(ctx, roomID)
	if err != nil {
		return RoomSummary{}, fmt.Errorf("get room summary: %w", err)
	}
	return toRoomSummary(resp, roomID), nil
}

func (s *Service) PreviewRoom(ctx context.Context, roomIDOrAlias string, via []string) (RoomSummary, error) {
	resp, err := s.client.GetRoomSummary(ctx, roomIDOrAlias, via...)
	if err != nil {
		return RoomSummary{}, fmt.Errorf("preview room: %w", err)
	}
	return toRoomSummary(resp, roomIDOrAlias), nil
}

func (s *Service) ListRoomMembers(ctx context.Context, roomID string) ([]MemberInfo, error) {
	resp, err := s.client.JoinedMembers(ctx, id.RoomID(roomID))
	if err != nil {
		return nil, fmt.Errorf("list room members: %w", err)
	}
	members := make([]MemberInfo, 0, len(resp.Joined))
	for userID, member := range resp.Joined {
		members = append(members, MemberInfo{
			UserID:      userID.String(),
			DisplayName: member.DisplayName,
			AvatarURL:   member.AvatarURL,
		})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (s *Service) GetRoomMember(ctx context.Context, roomID string, userID string) (MemberInfo, error) {
	members, err := s.ListRoomMembers(ctx, roomID)
	if err != nil {
		return MemberInfo{}, err
	}
	for _, member := range members {
		if member.UserID == userID {
			return member, nil
		}
	}
	return MemberInfo{}, fmt.Errorf("member %s is not joined to room %s", userID, roomID)
}

func (s *Service) GetStateEvent(ctx context.Context, roomID string, eventType string, stateKey string) (EventSummary, error) {
	evt, err := s.client.FullStateEvent(ctx, id.RoomID(roomID), event.NewEventType(eventType), stateKey)
	if err != nil {
		return EventSummary{}, fmt.Errorf("get state event: %w", err)
	}
	return summarizeEvent(evt), nil
}

func (s *Service) ListStateEvents(ctx context.Context, roomID string) ([]EventSummary, error) {
	stateMap, err := s.client.State(ctx, id.RoomID(roomID))
	if err != nil {
		return nil, fmt.Errorf("list state events: %w", err)
	}
	result := make([]EventSummary, 0)
	for _, byStateKey := range stateMap {
		for _, evt := range byStateKey {
			if evt == nil {
				continue
			}
			result = append(result, summarizeEvent(evt))
		}
	}
	sortEvents(result)
	return result, nil
}

func (s *Service) ListMessages(ctx context.Context, req ListMessagesRequest) (MessagesResult, error) {
	direction, err := parseDirection(req.Direction)
	if err != nil {
		return MessagesResult{}, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	resp, err := s.client.Messages(ctx, id.RoomID(req.RoomID), req.From, req.To, direction, nil, limit)
	if err != nil {
		return MessagesResult{}, fmt.Errorf("list room messages: %w", err)
	}
	return MessagesResult{
		Start:  resp.Start,
		End:    resp.End,
		Events: summarizeEvents(resp.Chunk),
		State:  summarizeEvents(resp.State),
	}, nil
}

func (s *Service) GetEvent(ctx context.Context, roomID string, eventID string) (EventSummary, error) {
	evt, err := s.client.GetEvent(ctx, id.RoomID(roomID), id.EventID(eventID))
	if err != nil {
		return EventSummary{}, fmt.Errorf("get event: %w", err)
	}
	return summarizeEvent(evt), nil
}

func (s *Service) GetEventContext(ctx context.Context, roomID string, eventID string, limit int) (EventContextResult, error) {
	if limit <= 0 {
		limit = 5
	}
	resp, err := s.client.Context(ctx, id.RoomID(roomID), id.EventID(eventID), nil, limit)
	if err != nil {
		return EventContextResult{}, fmt.Errorf("get event context: %w", err)
	}
	return EventContextResult{
		Event:        summarizeEvent(resp.Event),
		EventsBefore: summarizeEvents(resp.EventsBefore),
		EventsAfter:  summarizeEvents(resp.EventsAfter),
		State:        summarizeEvents(resp.State),
		Start:        resp.Start,
		End:          resp.End,
	}, nil
}

func (s *Service) ListRelations(ctx context.Context, req ListRelationsRequest) (RelationsResult, error) {
	direction, err := parseDirection(req.Direction)
	if err != nil {
		return RelationsResult{}, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	relationsReq := &mautrix.ReqGetRelations{
		RelationType: event.RelationType(req.RelationType),
		EventType:    event.NewEventType(req.EventType),
		Dir:          direction,
		From:         req.From,
		To:           req.To,
		Limit:        limit,
		Recurse:      req.Recurse,
	}
	if req.EventType == "" {
		relationsReq.EventType = event.Type{}
	}
	resp, err := s.client.GetRelations(ctx, id.RoomID(req.RoomID), id.EventID(req.EventID), relationsReq)
	if err != nil {
		return RelationsResult{}, fmt.Errorf("list event relations: %w", err)
	}
	return RelationsResult{
		Events:         summarizeEvents(resp.Chunk),
		NextBatch:      resp.NextBatch,
		PrevBatch:      resp.PrevBatch,
		RecursionDepth: resp.RecursionDepth,
	}, nil
}

func (s *Service) CreateRoom(ctx context.Context, req CreateRoomRequest) (CreateRoomResult, error) {
	invite := make([]id.UserID, 0, len(req.Invite))
	for _, userID := range req.Invite {
		invite = append(invite, id.UserID(userID))
	}

	createReq := &mautrix.ReqCreateRoom{
		Name:     req.Name,
		Topic:    req.Topic,
		Invite:   invite,
		IsDirect: req.IsDirect,
	}
	if req.IsPublic {
		createReq.Visibility = "public"
		createReq.Preset = "public_chat"
	} else {
		createReq.Visibility = "private"
		createReq.Preset = "private_chat"
	}

	resp, err := s.client.CreateRoom(ctx, createReq)
	if err != nil {
		return CreateRoomResult{}, fmt.Errorf("create room: %w", err)
	}
	return CreateRoomResult{RoomID: resp.RoomID.String()}, nil
}

func (s *Service) JoinRoom(ctx context.Context, req JoinRoomRequest) (JoinRoomResult, error) {
	resp, err := s.client.JoinRoom(ctx, req.RoomIDOrAlias, &mautrix.ReqJoinRoom{Via: req.Via, Reason: req.Reason})
	if err != nil {
		return JoinRoomResult{}, fmt.Errorf("join room: %w", err)
	}
	return JoinRoomResult{RoomID: resp.RoomID.String()}, nil
}

func (s *Service) InviteRoomMember(ctx context.Context, req InviteRoomMemberRequest) (InviteRoomMemberResult, error) {
	_, err := s.client.InviteUser(ctx, id.RoomID(req.RoomID), &mautrix.ReqInviteUser{
		Reason: req.Reason,
		UserID: id.UserID(req.UserID),
	})
	if err != nil {
		return InviteRoomMemberResult{}, fmt.Errorf("invite room member: %w", err)
	}
	return InviteRoomMemberResult{RoomID: req.RoomID, UserID: req.UserID}, nil
}

func (s *Service) LeaveRoom(ctx context.Context, req LeaveRoomRequest) (LeaveRoomResult, error) {
	_, err := s.client.LeaveRoom(ctx, id.RoomID(req.RoomID), &mautrix.ReqLeave{Reason: req.Reason})
	if err != nil {
		return LeaveRoomResult{}, fmt.Errorf("leave room: %w", err)
	}
	return LeaveRoomResult{RoomID: req.RoomID}, nil
}

func (s *Service) CreateRoomAlias(ctx context.Context, req CreateRoomAliasRequest) (CreateRoomAliasResult, error) {
	_, err := s.client.CreateAlias(ctx, id.RoomAlias(req.RoomAlias), id.RoomID(req.RoomID))
	if err != nil {
		return CreateRoomAliasResult{}, fmt.Errorf("create room alias: %w", err)
	}
	return CreateRoomAliasResult{RoomAlias: req.RoomAlias, RoomID: req.RoomID}, nil
}

func (s *Service) GetRoomAlias(ctx context.Context, roomAlias string) (RoomAliasResult, error) {
	resp, err := s.client.ResolveAlias(ctx, id.RoomAlias(roomAlias))
	if err != nil {
		return RoomAliasResult{}, fmt.Errorf("resolve room alias: %w", err)
	}
	servers := make([]string, 0, len(resp.Servers))
	for _, server := range resp.Servers {
		servers = append(servers, server)
	}
	sort.Strings(servers)
	return RoomAliasResult{
		RoomAlias: roomAlias,
		RoomID:    resp.RoomID.String(),
		Servers:   servers,
	}, nil
}

func (s *Service) DeleteRoomAlias(ctx context.Context, roomAlias string) (DeleteRoomAliasResult, error) {
	_, err := s.client.DeleteAlias(ctx, id.RoomAlias(roomAlias))
	if err != nil {
		return DeleteRoomAliasResult{}, fmt.Errorf("delete room alias: %w", err)
	}
	return DeleteRoomAliasResult{RoomAlias: roomAlias}, nil
}

func (s *Service) GetRoomDirectoryVisibility(ctx context.Context, roomID string) (RoomDirectoryVisibilityResult, error) {
	var resp roomDirectoryVisibilityPayload
	if err := s.doRoomDirectoryVisibilityRequest(ctx, http.MethodGet, roomID, nil, &resp); err != nil {
		return RoomDirectoryVisibilityResult{}, err
	}
	return RoomDirectoryVisibilityResult{RoomID: roomID, Visibility: resp.Visibility}, nil
}

func (s *Service) SetRoomDirectoryVisibility(ctx context.Context, req SetRoomDirectoryVisibilityRequest) (RoomDirectoryVisibilityResult, error) {
	if err := s.doRoomDirectoryVisibilityRequest(ctx, http.MethodPut, req.RoomID, roomDirectoryVisibilityPayload{Visibility: req.Visibility}, &struct{}{}); err != nil {
		return RoomDirectoryVisibilityResult{}, err
	}
	return RoomDirectoryVisibilityResult{RoomID: req.RoomID, Visibility: req.Visibility}, nil
}

func (s *Service) doRoomDirectoryVisibilityRequest(ctx context.Context, method string, roomID string, reqBody any, resBody any) error {
	urlPath := s.client.BuildClientURL("v3", "directory", "list", "room", id.RoomID(roomID))
	_, err := s.client.MakeRequest(ctx, method, urlPath, reqBody, resBody)
	if err != nil {
		action := "get"
		if method == http.MethodPut {
			action = "set"
		}
		return fmt.Errorf("%s room directory visibility: %w", action, err)
	}
	return nil
}

func (s *Service) SendText(ctx context.Context, req SendTextRequest) (EventWriteResult, error) {
	var (
		resp *mautrix.RespSendEvent
		err  error
	)
	if req.Notice {
		resp, err = s.client.SendNotice(ctx, id.RoomID(req.RoomID), req.Body)
	} else {
		resp, err = s.client.SendText(ctx, id.RoomID(req.RoomID), req.Body)
	}
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("send text: %w", err)
	}
	return EventWriteResult{EventID: resp.EventID.String()}, nil
}

func (s *Service) ReplyText(ctx context.Context, req ReplyTextRequest) (EventWriteResult, error) {
	original, err := s.client.GetEvent(ctx, id.RoomID(req.RoomID), id.EventID(req.EventID))
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("load replied-to event: %w", err)
	}
	content := buildMessageContent(req.Body, req.Notice)
	content.SetReply(original)
	resp, err := s.client.SendMessageEvent(ctx, id.RoomID(req.RoomID), event.EventMessage, content)
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("send reply: %w", err)
	}
	return EventWriteResult{EventID: resp.EventID.String()}, nil
}

func (s *Service) EditText(ctx context.Context, req EditTextRequest) (EventWriteResult, error) {
	content := buildMessageContent(req.Body, req.Notice)
	content.SetEdit(id.EventID(req.EventID))
	resp, err := s.client.SendMessageEvent(ctx, id.RoomID(req.RoomID), event.EventMessage, content)
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("send edit: %w", err)
	}
	return EventWriteResult{EventID: resp.EventID.String()}, nil
}

func (s *Service) React(ctx context.Context, req ReactRequest) (EventWriteResult, error) {
	resp, err := s.client.SendReaction(ctx, id.RoomID(req.RoomID), id.EventID(req.EventID), req.Key)
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("send reaction: %w", err)
	}
	return EventWriteResult{EventID: resp.EventID.String()}, nil
}

func (s *Service) Redact(ctx context.Context, req RedactRequest) (EventWriteResult, error) {
	resp, err := s.client.RedactEvent(ctx, id.RoomID(req.RoomID), id.EventID(req.EventID), mautrix.ReqRedact{Reason: req.Reason})
	if err != nil {
		return EventWriteResult{}, fmt.Errorf("redact event: %w", err)
	}
	return EventWriteResult{EventID: resp.EventID.String()}, nil
}

func buildRegistrationAuth(uia *mautrix.RespUserInteractive, registrationToken string) (any, error) {
	if uia == nil {
		return nil, errors.New("homeserver did not provide a registration auth flow")
	}
	if strings.TrimSpace(registrationToken) != "" {
		if uia.HasSingleStageFlow(registrationTokenAuthType) {
			return registrationTokenAuthData{
				BaseAuthData: mautrix.BaseAuthData{Type: registrationTokenAuthType, Session: uia.Session},
				Token:        registrationToken,
			}, nil
		}
		if uia.HasSingleStageFlow(mautrix.AuthTypeDummy) {
			return mautrix.BaseAuthData{Type: mautrix.AuthTypeDummy, Session: uia.Session}, nil
		}
		return nil, fmt.Errorf("homeserver does not accept registration tokens for this flow")
	}
	if uia.HasSingleStageFlow(mautrix.AuthTypeDummy) {
		return mautrix.BaseAuthData{Type: mautrix.AuthTypeDummy, Session: uia.Session}, nil
	}
	if uia.HasSingleStageFlow(registrationTokenAuthType) {
		return nil, errors.New("homeserver requires a registration token for account creation, but matrix-mcp was started without one")
	}
	return nil, errors.New("unsupported registration auth flow")
}

func buildMessageContent(body string, notice bool) *event.MessageEventContent {
	msgType := event.MsgText
	if notice {
		msgType = event.MsgNotice
	}
	return &event.MessageEventContent{MsgType: msgType, Body: body}
}

func toRoomSummary(resp *mautrix.RespRoomSummary, fallback string) RoomSummary {
	allowed := make([]string, 0, len(resp.AllowedRoomIDs))
	for _, roomID := range resp.AllowedRoomIDs {
		allowed = append(allowed, roomID.String())
	}
	roomID := resp.RoomID.String()
	if roomID == "" {
		roomID = fallback
	}
	return RoomSummary{
		RoomID:           roomID,
		AvatarURL:        string(resp.AvatarURL),
		CanonicalAlias:   resp.CanonicalAlias.String(),
		GuestCanJoin:     resp.GuestCanJoin,
		JoinRule:         string(resp.JoinRule),
		Name:             resp.Name,
		NumJoinedMembers: resp.NumJoinedMembers,
		RoomType:         string(resp.RoomType),
		Topic:            resp.Topic,
		WorldReadable:    resp.WorldReadable,
		RoomVersion:      string(resp.RoomVersion),
		Encryption:       string(resp.Encryption),
		AllowedRoomIDs:   allowed,
		Membership:       string(resp.Membership),
	}
}

func summarizeEvents(events []*event.Event) []EventSummary {
	result := make([]EventSummary, 0, len(events))
	for _, evt := range events {
		result = append(result, summarizeEvent(evt))
	}
	sortEvents(result)
	return result
}

func summarizeEvent(evt *event.Event) EventSummary {
	if evt == nil {
		return EventSummary{}
	}
	var stateKey *string
	if evt.StateKey != nil {
		value := *evt.StateKey
		stateKey = &value
	}
	summary := EventSummary{
		EventID:     evt.ID.String(),
		RoomID:      evt.RoomID.String(),
		Sender:      evt.Sender.String(),
		Type:        evt.Type.Type,
		StateKey:    stateKey,
		TimestampMS: evt.Timestamp,
		Content:     cloneContentMap(evt.Content),
	}
	if evt.Redacts != "" {
		summary.Redacts = evt.Redacts.String()
	}
	return summary
}

func sortEvents(events []EventSummary) {
	sort.Slice(events, func(i, j int) bool {
		left, right := events[i], events[j]
		if left.TimestampMS != right.TimestampMS {
			return left.TimestampMS < right.TimestampMS
		}
		if left.Type != right.Type {
			return left.Type < right.Type
		}
		leftState, rightState := "", ""
		if left.StateKey != nil {
			leftState = *left.StateKey
		}
		if right.StateKey != nil {
			rightState = *right.StateKey
		}
		if leftState != rightState {
			return leftState < rightState
		}
		return left.EventID < right.EventID
	})
}

func cloneContentMap(content event.Content) map[string]any {
	if content.Raw != nil {
		result := make(map[string]any, len(content.Raw))
		for key, value := range content.Raw {
			result[key] = value
		}
		return result
	}
	if len(content.VeryRaw) == 0 {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(content.VeryRaw, &result); err != nil {
		return map[string]any{"_raw": string(content.VeryRaw)}
	}
	return result
}

func marshalMap(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseDirection(raw string) (mautrix.Direction, error) {
	switch strings.TrimSpace(raw) {
	case "", "b":
		return mautrix.DirectionBackward, nil
	case "f":
		return mautrix.DirectionForward, nil
	default:
		return 0, fmt.Errorf("direction must be 'b' or 'f'")
	}
}

func randomPassword() string {
	buf := make([]byte, 12)
	if _, err := crand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

var _ API = (*Service)(nil)
