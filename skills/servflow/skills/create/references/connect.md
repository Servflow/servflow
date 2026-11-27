# Connect to Servflow

Read this when the Servflow tools are missing or a call to them is refused. Both
`create` and `create-support-agent` use it.

## How to tell what is wrong

Call `list_configs` with `kind: "agent"`.

| What happens | What it means | What to do |
|---|---|---|
| It returns a list (empty is fine) | Connected | Go back to the skill and carry on. |
| There is no `list_configs` tool | The connector is not added, or the session has not picked it up | Walk the person through **Set up** below, then stop. |
| A 401, or the client asks to sign in | The connector is added but not signed in | Ask the person to sign in (see their client below), then stop. |
| A 403 that says `Sign in to the dashboard and create your team first.` | Signed in, but the account has no team yet | Relay that sentence as it is, send them to https://dash.servflow.io to create their team, then stop. |
| `invalid auth token` | The API key in their client config is wrong or was replaced | Ask them to copy the key again from the dashboard (see **API key** below), then stop. |

Stopping matters: the person has to act in another window, and the tools only appear
after they do. Say what they need to do, then wait for them to say it is done. After a
connector is added, most clients need a reconnect or a restart before the tools show up;
tell them to run the same command again afterwards.

## Set up

1. Create an account at https://dash.servflow.io and create your team. Every agent,
   integration and credit belongs to a team, so the connector cannot work without one.
2. Add the connector to your client (below).
3. Sign in in the browser when the client asks.

The connector values, which are the same everywhere:

- URL: `https://server.servflow.io/api/mcp`
- Transport: streamable HTTP
- OAuth client ID: `W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV` (a public value; there is no client secret)

### Claude Code with the Servflow plugin

The connector comes with the plugin. The person only signs in: run `/mcp`, pick
`servflow`, and finish in the browser.

### Claude Code without the plugin

```
claude mcp add --transport http --client-id W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV --callback-port 8976 servflow https://server.servflow.io/api/mcp
```

You may offer to run this command for the person: it holds no secret. It takes effect
after a reconnect or restart. Then they run `/mcp`, pick `servflow` and sign in.

### Claude (claude.ai and the desktop app)

1. Open **Customize > Connectors**, click **+ Add**, then **Add custom connector**.
2. Name: `Servflow`. URL: `https://server.servflow.io/api/mcp`.
3. Under the OAuth client choice, pick **Use your own OAuth client** and paste the client
   ID `W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV`. Leave the client secret empty.
4. Connect, and sign in in the browser.

### Codex

```
codex mcp add servflow --url https://server.servflow.io/api/mcp --oauth-client-id W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV
codex mcp login servflow
```

### API key (clients without browser sign-in)

Any client that can send a header can use the team's API key instead of signing in:

1. In the dashboard, open **Settings > MCP connection** and copy **Your API key**.
2. In the client's own MCP config, add the header `Authorization: Bearer <your API key>`.

The person puts the key in their config themselves. Never ask them to paste the key, or
any token or secret, into this chat: anything in the chat is sent to the model and kept in
the conversation, and the key gives full control of the team.
