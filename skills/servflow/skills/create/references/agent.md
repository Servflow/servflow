# Author an agent config with the Servflow tools

Read this at the build step of `create`. Every field name here comes from the tools
themselves; when this page and a tool disagree, the tool wins, because it is reflected
from what the server accepts.

## What an agent is

An agent is one stored config with:

- **One entry**: how a run starts. A **trigger** entry runs on its `cron` schedule (or
  when another agent hands it a task). A **handler** entry is delivered to: a message in
  Slack, Telegram or Discord, an app event, a GitHub delivery, or a plain HTTP request.
- **Steps** the entry runs before the agent, in order: actions that fetch what the agent
  needs. Each step's output is read in a prompt as `{{ .<step id> }}`.
- **One or more agent nodes** under `agents`: each calls a model with a system prompt and
  a user prompt, and may call **tools** (actions the model decides to run).

Only an agent node speaks. A run's output is the last thing the agent said; a handler
entry delivers that back to where the message came from. To do something with the result
(post it, email it), give the agent a tool that does it.

## Tool order

Call these in order, and skip what the build does not need:

1. `config_schema` and `config_example`: the shape every config must satisfy, and a valid
   one to start from.
2. `list_entry_handlers`, then `describe_entry_handler` for the one you pick: its fields,
   whether it needs a `path`, and the variables it gives a prompt (`{{ .slack.text }}`).
   For a schedule, there is no handler: use a trigger entry with a `cron`.
3. For a start on an app event (a new email, a new message): `list_app_events` without an
   app to see the apps, then with `app`, then `describe_app_event`.
4. `list_actions` without `app` to list the apps, then `list_actions` with `app` set, then
   `describe_action` with the exact type names it returned. Composio app actions are typed
   `external:<SLUG>`, for example `external:GMAIL_SEND_EMAIL`; the bare slug is refused.
5. `list_template_functions` when a prompt or field needs a function beyond `{{ .x }}`.
6. `list_providers`, only when the person asked for their own model key.
7. `list_integrations`: the integration ids the config will name.
8. `validate_config`, until it returns `"valid": true`.
9. `create_config` with the same body. It returns the stored row, disabled, with its
   `config_id`.
10. `set_config_enabled` with that `config_id` and `enabled: true`. This is what serves
    the agent. It checks what the config names and builds it, and returns why when it
    cannot; fix that and call it again.

To change an agent later: `get_config`, change it, `validate_config`, `update_config`
with the whole body (it replaces the config), then `set_config_enabled` again, because an
enabled agent keeps serving its previous version until it is enabled again.

## The body you send

`validate_config`, `create_config` and `update_config` take `config`: the stored row.

```json
{
  "name": "Morning inbox summary",
  "config_id": "morning-inbox-summary",
  "config": {
    "name": "Morning inbox summary",
    "entries": [ { "type": "trigger", "trigger": { "cron": "..." }, "steps": [], "start": "agent.<id>" } ],
    "agents": { "<id>": { "name": "...", "providerID": "native", "systemPrompt": "...", "userPrompt": "...", "next": "end", "tools": [] } }
  }
}
```

- `config_id` is the agent's identity. Set a short lowercase slug; omit it and one is made
  from `name`. It must not already exist (check `list_configs` with `kind: "agent"`).
- `enabled` is ignored on a save. Only `set_config_enabled` serves an agent.
- `start` and each node's `next` take `agent.<id>` or `end`.
- Step ids and agent ids share one namespace. A step id is lowercase letters, digits and
  underscores, not starting with a digit, because a prompt reads it as `{{ .<id> }}`.

## The model

- **Built-in model**: set `"providerID": "native"` on the agent node. Use it unless the
  person asks to use their own key. It never appears in `list_providers`, so do not look
  for it there. Runs on it cost credits.
- **Their own key**: name a `providerId` from `list_providers`. If they have none, send
  them to **Settings > Models** in the dashboard to add their key there; do not ask for the
  key in the chat.

The field is `providerID` (capital `ID`) in the config; `list_providers` returns it as
`providerId`.

## Integrations

An action field whose type is `integration` names an integration by its `integrationId`
from `list_integrations`, for example `"gmail-41ed8c01"`. `describe_action` lists the
groups that field accepts (`"integrationGroups": ["composio:gmail"]`).

A connected app such as Gmail or Slack is stored with a type like
`composio:ac_tldmHywOPPU4`, which does not name the app. Match it by the integration's
`name` and check `"status": "complete"` and `"enabled": true`. A connected app is
connected in the dashboard, never over MCP, because it needs the person's browser sign-in
to that app.

Some integrations take a token instead, such as a Discord bot (`discord`) or a Telegram
bot (`telegram_bot`). For those:

1. `describe_integration_type` with the type, to see its fields.
2. Ask the person to save the token as a secret in the dashboard, under **Secrets**, and
   to tell you the secret's name (never its value). `list_secrets` shows the names.
3. `create_integration` with `name`, `type` and a `config` mapping each field to
   `{"secret": "<secret name>"}` or `{"value": <literal>}`, for example
   `{"bot_token": {"secret": "discord_bot_token"}, "guild_id": {"value": "1234567890"}}`.
4. `set_integration_enabled` with `enabled: true`.

## Secrets

Read a stored secret inside any template as `{{ secret "NAME" }}`, for example in an
`http` step's header. The person saves the value in the dashboard under **Secrets**; you
only ever handle its name.

## Action fields: steps and tools are written differently

- In a **step**, `config` maps each field straight to its value:
  `"config": { "integration": "gmail-41ed8c01", "query": "is:unread" }`.
- In an **action tool**, each field says who fills it: `{"type": "raw", "value": ...}` for
  a value you set, or `{"type": "param", "value": "<name>"}` for one the model fills when
  it calls the tool. Set booleans, lists and maps as `raw`.

Fill every field `describe_action` marks `"required": true`; do not add fields it does not
list.

Wrap any value that can hold quotes or newlines in `escape` when you put it in a prompt:
`{{ escape (jsonout .inbox) }}`. Without it the prompt can break. The `http` action is the
exception: its body is encoded field by field, so do not escape there.

## Cron

A cron has 6 fields, seconds first: `0 0 9 * * 1-5` is 09:00 Monday to Friday. A
5-field cron is refused. The schedule runs in the server's time zone (UNVERIFIED: the
time zone is not exposed by any tool), so confirm with the person whether the time they
gave needs converting.

## Where it listens

- A trigger entry with a cron has no address: it runs on its schedule.
- A listener handler (`slack_bot`, `discord_gateway`, `app_event`; `describe_entry_handler`
  reports `"listener": true`) has no path and no address.
- Any other handler needs a `path` such as `/hooks/telegram`. Its full address is on the
  agent's own page in the dashboard; no tool returns the team's host, so send the person
  there to copy it rather than guessing it.

## Common validation failures

`validate_config` returns one problem at a time. Fix it and validate again.

| Message | Fix |
|---|---|
| `invalid trigger cron "0 9 * * 1-5": expected exactly 6 fields, found 5` | Add the seconds field first: `0 0 9 * * 1-5`. |
| `entry "trigger" start references unknown agent "b"` | `start` must be `agent.<id>` of a node under `agents`. |
| `a step id is lowercase letters, digits and underscores, not starting with a digit` | Rename the step (`in_box`, not `in-box`) and its uses in prompts. |
| `action '<step id>': missing required field "integration"` | Set the field in the step's `config`. |
| `agent "<id>": action tool "<name>": field "integration" is required` | Add `{"type": "raw", "value": "<integrationId>"}` to the tool's `actionConfig.config`. |
| `unknown action type "GMAIL_SEND_EMAIL"` | Use the exact type `list_actions` returned: `external:GMAIL_SEND_EMAIL`. |

`validate_config` checks the structure only. What the config names (the provider, the
integrations, a handler's required fields) is checked by `set_config_enabled`, so expect
some errors there too, and relay them in the server's words.

## Worked example

"Every weekday at 9:00, read my unread Gmail from the last day, summarize it, and email
me the summary." This body returned `"valid": true` from `validate_config`. Replace
`gmail-41ed8c01` with the person's Gmail integration id and `owner@example.com` with the
address they gave.

```json
{
  "name": "Morning inbox summary",
  "config_id": "morning-inbox-summary",
  "config": {
    "name": "Morning inbox summary",
    "entries": [
      {
        "type": "trigger",
        "trigger": { "cron": "0 0 9 * * 1-5" },
        "steps": [
          {
            "id": "inbox",
            "type": "external:GMAIL_FETCH_EMAILS",
            "config": {
              "integration": "gmail-41ed8c01",
              "query": "is:unread newer_than:1d",
              "max_results": "25",
              "verbose": false
            }
          }
        ],
        "start": "agent.summarize"
      }
    ],
    "agents": {
      "summarize": {
        "name": "Summarizer",
        "providerID": "native",
        "systemPrompt": "You write a short morning summary of unread email. Group the emails by what they need: reply today, read later, ignore. One line each: sender, subject, what they want. Then send the summary with the send_summary tool. If there are no emails, send one line saying the inbox is clear.",
        "userPrompt": "Unread emails from the last day:\n\n{{ escape (jsonout .inbox) }}",
        "next": "end",
        "tools": [
          {
            "type": "action",
            "name": "send_summary",
            "description": "Sends the summary by email to the inbox owner",
            "actionConfig": {
              "type": "external:GMAIL_SEND_EMAIL",
              "config": {
                "integration": { "type": "raw", "value": "gmail-41ed8c01" },
                "recipient_email": { "type": "raw", "value": "owner@example.com" },
                "subject": { "type": "raw", "value": "Your morning inbox summary" },
                "body": { "type": "param", "value": "body" }
              }
            }
          }
        ]
      }
    }
  }
}
```

Shown to the person as:

```
Every weekday at 9:00
        │
        ▼
Read unread email from the last day in Gmail
        │
        ▼
Agent: write a summary
        │
        ▼
Email the summary to owner@example.com
```
