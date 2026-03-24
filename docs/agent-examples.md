# Agent Discovery And Example Exchanges

This document shows what an agent actually sees when it connects to `matrix-mcp`, how it is expected to discover the tool surface, and a few realistic call sequences.

The resource snippets below are abbreviated from a real local run of the server. The Matrix data is illustrative, but the discovery flow, tool names, and request/response shapes are real.

## What The Agent Sees

An MCP client sees two things:

- directly callable tools such as `matrix.v1.rooms.list` or `matrix.v1.messages.react`
- discovery resources that explain what those tools are for and what arguments they take

The intended discovery resources are:

- `matrix://modules`
- `matrix://module/<name>`
- `matrix://tool/<name>`
- `matrix://scopes`

There is no deeper resource tree than that. In practice the shape is:

```text
matrix://modules
  -> matrix://module/<name>
    -> optional: matrix://tool/<name>
      -> tools/call
```

So the server is intentionally not doing a deep browse-first UX. It gives the agent a small index, then a focused per-module list, and then direct callable tool names.

### Root Discovery Resource

Reading `matrix://modules` returns a top-level index like this:

```markdown
# Matrix MCP modules

Use `matrix://module/<name>` to drill into a module, then call tools directly by name once you know what you want.

- `client`: Identity and readiness information for the active Matrix session. Resource: `matrix://module/client`
- `messages`: High-level message composition and mutation tools. Resource: `matrix://module/messages`
- `room.members`: Joined-member lookup tools for specific rooms. Resource: `matrix://module/room.members`
- `room.state`: State-event lookup tools for specific rooms. Resource: `matrix://module/room.state`
- `rooms`: Room discovery, inspection, creation, and join actions. Resource: `matrix://module/rooms`
- `server`: Homeserver-level metadata and feature discovery. Resource: `matrix://module/server`
- `timeline`: Timeline pagination, event lookup, context, and relation inspection. Resource: `matrix://module/timeline`
- `users`: User lookup, registration availability, and account creation. Resource: `matrix://module/users`
```

That is the main hint to the agent that this server expects hierarchical discovery rather than blind tool guessing.

It is also intentionally small enough that an LLM can usually read it once and pick a module without getting flooded with every tool schema up front.

### Module Detail Resource

Reading `matrix://module/rooms` returns a focused list of tools in that module:

```markdown
# Module `rooms`

Room discovery, inspection, creation, and join actions.

## Tools
- `matrix.v1.rooms.create`
  Scope: `rooms.create`
  Purpose: Create a Matrix room with optional invites and topic metadata.
  Detail: `matrix://tool/matrix.v1.rooms.create`
- `matrix.v1.rooms.get`
  Scope: `rooms.read`
  Purpose: Fetch summary metadata for a joined or otherwise accessible room by room ID.
  Detail: `matrix://tool/matrix.v1.rooms.get`
- `matrix.v1.rooms.join`
  Scope: `rooms.join`
  Purpose: Join a Matrix room by room ID or room alias.
  Detail: `matrix://tool/matrix.v1.rooms.join`
- `matrix.v1.rooms.list`
  Scope: `rooms.read`
  Purpose: List rooms joined by the active Matrix account, including summary metadata when the homeserver provides it.
  Detail: `matrix://tool/matrix.v1.rooms.list`
- `matrix.v1.rooms.preview`
  Scope: `rooms.read`
  Purpose: Preview a room by room ID or alias, optionally routing through via servers for alias resolution.
  Detail: `matrix://tool/matrix.v1.rooms.preview`
```

This is the point where the agent should stop guessing and pick a tool deliberately.

This is the main anti-overwhelm step in the design:

- the root resource is short and only names modules
- the module resource is still short and only names tools in one topic area
- full schemas are withheld until the agent asks for one specific tool

### Tool Detail Resource

Reading `matrix://tool/matrix.v1.messages.react` returns exactly how that tool should be called:

```markdown
# Tool `matrix.v1.messages.react`

- Module: `messages`
- Scope: `messages.react`
- Purpose: React to an event with an annotation such as an emoji.

## Direct call
Call this tool directly via `tools/call` with `name="matrix.v1.messages.react"`.

## Input schema
{
  "type": "object",
  "required": ["room_id", "event_id", "key"],
  "properties": {
    "event_id": {"type": "string", "description": "Matrix event ID to react to"},
    "key": {"type": "string", "description": "Reaction key, usually an emoji"},
    "room_id": {"type": "string", "description": "Matrix room ID containing the target event"}
  },
  "additionalProperties": false
}
```

Tool detail resources are the main answer to "how is the agent expected to know what this does and how to call it?"

The answer is: by reading the tool detail resource when it is not already certain.

But that read is optional. Once a tool has already been discovered and the agent knows its arguments, it can call it directly.

## Why This Does Not Overwhelm The LLM

The implementation is deliberately shallow and selective:

- `matrix://modules` gives one short index of module names and one-line descriptions.
- `matrix://module/<name>` narrows the surface to one topic such as `rooms` or `timeline`.
- `matrix://tool/<name>` is only for the one tool whose schema now matters.

That means the agent is not forced to read every tool schema to do common work.

Typical discovery cost is:

- 1 read for familiar tasks: go straight to `matrix://module/rooms`, then call `matrix.v1.rooms.list`
- 2 reads for unfamiliar tasks: `matrix://modules`, then `matrix://module/<name>`
- 3 reads only when schema detail matters: root, module, then specific tool detail

The implementation also avoids a deeper taxonomy like `module -> submodule -> category -> tool`, because that would make discovery too chatty for normal agent use.

## Direct Call After Discovery

The intended pattern is not "discover forever." It is "discover just enough, then call directly."

Minimal example:

```text
read_resource("matrix://module/rooms")
tools/call name="matrix.v1.rooms.list" arguments={}
```

Here the agent does not need to read `matrix://tool/matrix.v1.rooms.list` first, because the module listing already makes the tool's purpose obvious and the arguments are empty.

Schema-aware example:

```text
read_resource("matrix://module/messages")
read_resource("matrix://tool/matrix.v1.messages.react")
tools/call name="matrix.v1.messages.react" arguments={
  "room_id": "!ops:example.com",
  "event_id": "$latest",
  "key": "eyes"
}
```

The important point is that discovery yields a literal tool name that can be called directly. The resource tree is not a separate command system; it is only there to help the agent choose and understand the normal MCP tools.

## Expected Agent Behavior

The intended workflow is:

1. Read `matrix://modules` to see the available module groups.
2. Read `matrix://module/<name>` for the relevant area, such as `rooms`, `timeline`, or `messages`.
3. Read `matrix://tool/<name>` only when argument names or output shape matter.
4. Call the tool directly with `tools/call`.
5. Reuse returned IDs such as `room_id` and `event_id` in follow-up calls instead of trying to reconstruct them.
6. Read `matrix://scopes` before attempting writes if there is any doubt about whether mutation tools are enabled.

What the agent should not do:

- guess raw Matrix HTTP endpoints
- assume write tools exist without checking scope exposure
- invent argument names when a tool detail resource is available
- treat room aliases, room IDs, event IDs, and user IDs as interchangeable

## Example Exchanges

These are not meant to be magical one-shot examples. They show the normal rhythm of resource reads followed by direct tool calls.

### 1. "What rooms can you see?"

Reasonable sequence:

```text
read_resource("matrix://module/rooms")
tools/call name="matrix.v1.rooms.list" arguments={}
```

Possible tool result:

```json
{
  "ok": true,
  "audit": {
    "request_id": "req-1",
    "timestamp_ms": 1773590938043
  },
  "rooms": [
    {
      "room_id": "!ops:example.com",
      "name": "Ops"
    }
  ]
}
```

Reasonable reply to the human:

```text
I can currently see one joined room: Ops (`!ops:example.com`).
```

### 2. "React with eyes to the latest message in Ops."

Reasonable sequence:

```text
tools/call name="matrix.v1.rooms.list" arguments={}
tools/call name="matrix.v1.timeline.messages.list" arguments={
  "room_id": "!ops:example.com",
  "limit": 1
}
tools/call name="matrix.v1.messages.react" arguments={
  "room_id": "!ops:example.com",
  "event_id": "$latest",
  "key": "eyes"
}
```

The timeline call returns the event ID the agent needs:

```json
{
  "ok": true,
  "audit": {
    "request_id": "req-1",
    "timestamp_ms": 1773590938043
  },
  "start": "start",
  "end": "end",
  "events": [
    {
      "event_id": "$latest",
      "sender": "@alice:example.com",
      "type": "m.room.message",
      "content": {
        "body": "Ship it"
      }
    }
  ]
}
```

The important behavior here is that the agent first discovers the latest `event_id`, then uses that exact `event_id` in the reaction call.

### 3. "Create a public room called Release Ops and invite Alice."

Reasonable sequence:

```text
read_resource("matrix://scopes")
read_resource("matrix://module/rooms")
tools/call name="matrix.v1.rooms.create" arguments={
  "name": "Release Ops",
  "topic": "Release coordination",
  "is_public": true,
  "invite": ["@alice:example.com"]
}
```

Possible tool result:

```json
{
  "ok": true,
  "audit": {
    "request_id": "req-7",
    "timestamp_ms": 1773590939123
  },
  "room_id": "!new:example.com"
}
```

Reasonable reply:

```text
I created the room `!new:example.com` as a public room and requested an invite for `@alice:example.com`.
```

The agent should still understand that homeserver policy may reject the action even when the tool exists.

### 4. "Show me the context around this event before I answer."

Reasonable sequence:

```text
read_resource("matrix://module/timeline")
tools/call name="matrix.v1.timeline.event.context.get" arguments={
  "room_id": "!ops:example.com",
  "event_id": "$latest",
  "limit": 3
}
```

This is the right pattern when the agent has an `event_id` and needs nearby messages rather than only the target event.

## Practical Guidance For Agent Authors

If you are integrating this server into an agent:

- prime the agent to read `matrix://modules` before choosing tools in unfamiliar tasks
- prime it to read `matrix://tool/<name>` before calling write tools
- tell it to preserve and reuse returned IDs exactly
- tell it to check `matrix://scopes` before planning any mutation workflow

That is enough to make the server feel discoverable rather than opaque.
