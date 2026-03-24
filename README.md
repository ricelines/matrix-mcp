# matrix-mcp-go

`matrix-mcp-go` is an MCP server for Matrix. You run it against one Matrix account, and an MCP client can then inspect rooms, users, state, and timelines on that account's behalf. If you enable additional scopes, it can also resolve or manage room aliases, inspect or change room-directory visibility, create users, create or join rooms, invite or remove users from rooms, and send or modify messages.

This server is for people who want Matrix available as a tool surface inside an MCP-capable workflow. It is not a Matrix bridge and it is not a multi-user service. Everything it does is done as the configured Matrix account.

## What you get

Out of the box, the server exposes read-oriented Matrix tools for:

- active client identity and readiness
- homeserver versions and capabilities
- user search, profile lookup, and registration availability checks
- room listing, room summary lookup, and room preview
- joined-member lookup
- room state lookup
- timeline pagination, event fetch, event context, and relation inspection

If you opt into additional room metadata scopes, it can also:

- resolve room aliases
- inspect room-directory visibility

If you opt into write scopes, it can also:

- create Matrix users
- create rooms
- create or delete room aliases
- publish or unpublish rooms in the room directory
- join rooms
- invite users to rooms
- leave rooms
- send messages
- reply to messages
- edit messages
- react to events
- redact events

The server also publishes discovery resources such as `matrix://modules`, `matrix://module/<name>`, `matrix://tool/<name>`, and `matrix://scopes`, so MCP clients can inspect the available tool surface instead of treating it as opaque.

## Before you run it

You need:

- a Matrix homeserver URL
- credentials for the account this server should use
- an MCP client that can connect to a streamable HTTP MCP endpoint

Important constraints:

- The server acts as exactly one Matrix account.
- It can only read what that account can read.
- Write operations are available only if you enable the corresponding scopes.
- Homeserver permissions still apply. For example, enabling `rooms.create` or `rooms.directory.write` does not bypass a homeserver policy that disallows those actions for that account.
- If your homeserver requires a registration token for account creation, configure it when starting `matrix-mcp`; the user-creation tool does not accept it per call.

Use a dedicated bot or service account unless you have a specific reason not to.

## Quick start

Run from source:

```bash
export MATRIX_HOMESERVER_URL='https://matrix.example.com'
export MATRIX_USERNAME='matrix-bot'
export MATRIX_PASSWORD='replace-me'

go run ./cmd/matrix-mcp-go-server
```

By default the server listens on `:8080` and exposes only the default read-oriented scopes.

The MCP endpoint is the server root over HTTP, for example:

```text
http://127.0.0.1:8080/
```

## Using with Amber

This repo ships an Amber component manifest at `amber.json5`. It exports one capability named `mcp`, backed by `ghcr.io/ricelines/matrix-mcp-go:v0.1`.

The manifest is slot-routed. The Matrix homeserver is always provided through the external `matrix` HTTP slot. That keeps the homeserver on the capability path instead of smuggling it through config.

Validate the manifest:

```bash
amber check amber.json5
```

Compile it to a Docker Compose runtime directory:

```bash
amber compile amber.json5 --docker-compose /tmp/matrix-mcp-go-amber
```

That directory contains `compose.yaml`, `env.example`, and a generated README.

For a remote homeserver, set the external slot URL in `.env` and then expose the MCP export locally:

```bash
cd /tmp/matrix-mcp-go-amber
cp env.example .env
$EDITOR .env
docker compose up -d
amber proxy . --export mcp=127.0.0.1:18080
```

Set these values in `.env`:

```dotenv
AMBER_EXTERNAL_SLOT_MATRIX_URL=https://matrix.example.com
AMBER_CONFIG_USERNAME=matrix-bot
AMBER_CONFIG_PASSWORD=replace-me
```

For a local homeserver, you can also bind the slot directly through `amber proxy` instead of `.env`:

```bash
amber proxy . \
  --slot matrix=127.0.0.1:8008 \
  --export mcp=127.0.0.1:18080
```

Your local MCP endpoint is then:

```text
http://127.0.0.1:18080/
```

If you start Compose with a custom project name, pass the same name to `amber proxy`:

```bash
docker compose -p matrix-mcp-go up -d
amber proxy . --project-name matrix-mcp-go --export mcp=127.0.0.1:18080
```

The Amber config schema exposes these fields:

- `username`
- `password`
- `registration_token`
- `scopes`
- `listen_addr`

The Amber manifest also requires one external slot:

- `matrix`: HTTP capability for the homeserver base URL

`password` and `registration_token` are marked secret in the manifest schema.

## Docker

You can also run the image directly without Amber:

```bash
docker run --rm -p 8080:8080 \
  -e MATRIX_HOMESERVER_URL='https://matrix.example.com' \
  -e MATRIX_USERNAME='matrix-bot' \
  -e MATRIX_PASSWORD='replace-me' \
  ghcr.io/ricelines/matrix-mcp-go:v0.1
```

## Configuration

Required environment variables:

- `MATRIX_HOMESERVER_URL`: homeserver base URL, such as `https://matrix.example.com`
- `MATRIX_USERNAME`: username localpart used to log in
- `MATRIX_PASSWORD`: password for that account

Optional environment variables:

- `MATRIX_MCP_LISTEN_ADDR`: listen address for the HTTP server, default `:8080`
- `MATRIX_REGISTRATION_TOKEN`: registration token used by `matrix.v1.users.create` on homeservers that require `m.login.registration_token`
- `MATRIX_MCP_SCOPES`: comma-separated scope list, default `default`

Example with explicit listen address and write capabilities:

```bash
export MATRIX_HOMESERVER_URL='https://matrix.example.com'
export MATRIX_USERNAME='matrix-bot'
export MATRIX_PASSWORD='replace-me'
export MATRIX_MCP_LISTEN_ADDR='127.0.0.1:8080'
export MATRIX_MCP_SCOPES='default,rooms.alias.read,rooms.directory.read,users.create,rooms.create,rooms.alias.write,rooms.directory.write,rooms.join,rooms.invite,rooms.leave,messages.send,messages.reply,messages.edit,messages.react,messages.redact'

go run ./cmd/matrix-mcp-go-server
```

## Scopes

If `MATRIX_MCP_SCOPES` is empty or unset, the server enables this default set:

- `client.identity.read`
- `client.status.read`
- `server.read`
- `users.read`
- `rooms.read`
- `room.members.read`
- `room.state.read`
- `timeline.read`

Additional optional read scopes unlock room alias and room-directory inspection:

- `rooms.alias.read`
- `rooms.directory.read`

Additional write scopes unlock mutations:

- `users.create`
- `rooms.create`
- `rooms.alias.write`
- `rooms.directory.write`
- `rooms.join`
- `rooms.invite`
- `rooms.leave`
- `messages.send`
- `messages.reply`
- `messages.edit`
- `messages.react`
- `messages.redact`

`default` is a special token that expands to the default read set, so these are valid:

- `default`
- `default,messages.send`
- `default,rooms.alias.read,rooms.directory.read`
- `default,rooms.create,rooms.alias.write,rooms.directory.write,rooms.join,rooms.invite,rooms.leave,messages.send`

If you want a read-only deployment, leave `MATRIX_MCP_SCOPES` unset.

## Choosing scopes

Reasonable starting points:

- Read-only assistant: `default`
- Read-only assistant that can inspect aliases and room-directory visibility: `default,rooms.alias.read,rooms.directory.read`
- Messaging bot in existing rooms: `default,messages.send,messages.reply,messages.edit,messages.react`
- Room-management bot: `default,rooms.alias.read,rooms.directory.read,rooms.create,rooms.alias.write,rooms.directory.write,rooms.join,rooms.invite,rooms.leave,messages.send`
- Provisioning workflow that can create accounts: `default,users.create`

Avoid granting write scopes just because they exist. The point of the scope model is to make the MCP surface smaller and safer than "full Matrix client access."

## Tool surface

The server currently exposes tools under these module groups:

- `client`: who the server is logged in as, and whether that session is active
- `server`: homeserver versions and capability metadata
- `users`: username availability, directory search, profile lookup, and optional user creation
- `rooms`: list joined rooms, inspect or preview a room, and, when the corresponding scopes are enabled, resolve or manage room aliases, inspect or manage room-directory visibility, create or join rooms, invite users, and leave rooms
- `room.members`: list or fetch joined members in a room
- `room.state`: fetch one state event or list visible state in a room
- `timeline`: paginate messages, fetch an event, inspect context around an event, list relations such as reactions or edits
- `messages`: send, reply, edit, react, and redact when the corresponding scopes are enabled

The exact tool names are discoverable from the MCP server itself. The intended flow is:

1. read `matrix://modules`
2. inspect a module with `matrix://module/<name>`
3. call the tool you actually want

That keeps client-side prompts and tool selection grounded in the server's own advertised surface instead of stale documentation.

For an agent-facing walkthrough with actual discovery-resource output and realistic `tools/call` sequences, see `docs/agent-examples.md`.

## Operational notes

- The server uses HTTP MCP, not stdio.
- It does not expose federation admin operations or homeserver administration beyond what is implemented in the current tool set.
- User creation is done through the homeserver registration API, so homeserver-specific registration behavior still matters.
- Timeline and room reads depend on what the configured account is allowed to see and what the homeserver returns.
- If you are exposing this beyond localhost, put it behind the normal network controls you would use for any authenticated internal service.

## Building from source

```bash
go build ./cmd/matrix-mcp-go-server
```

To build the container image:

```bash
docker build -t matrix-mcp-go .
```

## Testing

Unit and integration coverage exists in the repo, including dockerized integration tests against Tuwunel. If you are changing behavior rather than just running the server, use:

```bash
env GOCACHE=/tmp/matrix-mcp-go-build-cache go test ./...
```
