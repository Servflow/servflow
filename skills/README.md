# Servflow skills

Skills that let an AI assistant build Servflow agents for you through the Servflow MCP
connector. You describe what you want in one sentence; the assistant proposes it, waits
for your yes, connects the apps it needs, builds it and enables it.

| Skill | What it does | In Claude Code |
|---|---|---|
| `create` | Creates an agent that does a job for you: a scheduled summary, an assistant in Slack or Telegram, an agent that runs on a new email. Hands support requests to `create-support-agent`. | `/servflow:create an agent that does X` |
| `create-support-agent` | Creates a support agent that answers your customers on your website's live chat, drafted from your own website. | `/servflow:create-support-agent for my bike shop` |

You need a Servflow account and team: sign up at https://dash.servflow.io.

The skills live in `servflow/skills/`. Each folder holds a `SKILL.md` and the reference
pages it reads.

## Claude Code

Install the plugin; the Servflow connector comes with it:

```
/plugin marketplace add Servflow/servflow
/plugin install servflow@servflow
```

From a shell: `claude plugin marketplace add Servflow/servflow`, then
`claude plugin install servflow@servflow`.

Then sign in: run `/mcp`, pick `servflow`, and finish in the browser. The skills are
`/servflow:create …` and `/servflow:create-support-agent …`.

To add only the connector, without the plugin:

```
claude mcp add --transport http --client-id W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV --callback-port 8976 servflow https://server.servflow.io/api/mcp
```

Restart Claude Code, then run `/mcp` to sign in.

## Claude (claude.ai and the desktop app)

1. Add the connector: open **Customize > Connectors**, click **+ Add**, then **Add custom
   connector**.
   - Name: `Servflow`
   - URL: `https://server.servflow.io/api/mcp`
   - Under the OAuth client choice, pick **Use your own OAuth client** and paste the client
     ID `W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV`. There is no client secret.
2. Connect, and sign in in the browser.
3. Add the skills: make a ZIP of each skill folder (`servflow/skills/create` and
   `servflow/skills/create-support-agent`) and upload each under **Customize > Skills**.

There are no slash commands there: ask for what you want ("create a Servflow agent that
emails me a summary of my unread mail every morning") and Claude uses the skill.

## Codex

These steps have not yet been tested end to end.

1. Copy each skill folder to `~/.agents/skills/` (or `.agents/skills/` in a repository),
   so you have `~/.agents/skills/create/SKILL.md` and
   `~/.agents/skills/create-support-agent/SKILL.md`.
2. Add the connector and sign in:

   ```
   codex mcp add servflow --url https://server.servflow.io/api/mcp --oauth-client-id W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV
   codex mcp login servflow
   ```

3. Use a skill with `$create` or from `/skills`.

## Connector values

For any other MCP client:

| | |
|---|---|
| URL | `https://server.servflow.io/api/mcp` |
| Transport | Streamable HTTP |
| OAuth client ID | `W4BjkKoNAyAsLIYrZ7NzCoPQUhjWUXKV` (public; no client secret) |

A client without browser sign-in can send the team's API key instead. Copy it from the
dashboard under **Settings > MCP connection > Your API key**, and add the header
`Authorization: Bearer <your API key>` in the client's own config. Keep the key in that
config: do not paste it into a chat.

## License

MIT. See [LICENSE](LICENSE).
