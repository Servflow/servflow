---
name: create
description: >-
  Create and enable a Servflow agent from a plain description, through the Servflow MCP
  connector: it checks the connection, proposes the agent as a short diagram, connects
  the apps it needs, builds it, enables it and says where to see its runs. Use it
  whenever someone wants Servflow to do a job for them (a scheduled summary, a Slack or
  Telegram assistant, an agent that runs on a new email or GitHub event), even if they
  don't say "agent". For an agent that answers a business's customers, it hands off to
  the create-support-agent skill.
argument-hint: "an agent that does X | a support agent for my shop"
---

# Create a Servflow agent

You turn one sentence from the person into an enabled Servflow agent, using only the
Servflow MCP tools. The person may never have used Servflow, so speak in their words, show
before you build, and never leave them guessing what to do next.

The request is in `$ARGUMENTS`. If it is empty, ask exactly one question and wait:

> What should your agent do, or is this a support agent that answers your customers?

## Step 1: Decide what to build

- **Support agent**: it answers the business's own customers or buyers (on its website's
  live chat, by email). "A support agent for my bike shop", "answer questions from people
  on my site", "help my customers with returns".
- **Agent**: everything else. It does work for the person or their team: summarize, triage,
  post, reply in their Slack, review a pull request.

For a support agent, hand off now: invoke the `create-support-agent` skill with the
person's request (in Claude Code, `/servflow:create-support-agent <request>`). If your
client cannot invoke another skill, read `../create-support-agent/SKILL.md` and follow it.
Do not continue here.

If you cannot tell which one they mean, ask the question above.

## Step 2: Check the connector

Call `list_configs` with `kind: "agent"`. If it returns a list, carry on. If the tool is
missing or the call is refused, read `references/connect.md`, tell the person what to do,
and stop until they say it is done.

## Step 3: Propose, then wait

Look up what you need before proposing, so the proposal is something you can build:
`list_entry_handlers`, `list_app_events`, and `list_actions` (apps first, then one app). Read
`references/agent.md` now; it has the tool order and the config rules.

Show the agent as a small top-to-bottom diagram in the person's own words: the trigger,
the steps, what the agent does, and where the output goes.

```
Every weekday at 9:00
        │
        ▼
Read new rows in Google Sheets
        │
        ▼
Agent: write a summary
        │
        ▼
Post it in Slack #sales
```

Then one line each:

- **Trigger**: what starts a run (a schedule, a Slack message, a new email).
- **Apps**: each app the agent uses.
- **Model**: the built-in model, unless they asked to use their own key.
- **Credits**: each run costs credits.

Ask at most one clarifying question, and only when the answer changes what you build
(which Slack channel, which address to email). Then ask for a yes or changes, and wait.
Create nothing before the yes: a half-built agent the person did not agree to is clutter
they have to find and remove.

## Step 4: Accounts

From `list_integrations`, list each app the proposal needs, marked **connected** or **not
connected**, and ask whether they have those accounts ready.

For each app that is not connected, send them to connect it in Servflow:

<!-- Swap in the direct set-up link here when it ships; this is the only line to change. -->
**Connect it in the dashboard: https://dash.servflow.io/integrations, then New Integration.**

Connecting an app needs their browser sign-in to that app, which MCP cannot do. Wait until
they say it is done, then confirm with `list_integrations`.

For an app that takes a token instead (Discord or Telegram, which give a token for the
account the agent answers as), ask them to save
the token in the dashboard under **Secrets** and tell you the secret's name, then create
the integration over MCP as `references/agent.md` describes. The token never goes in the
chat: anything pasted here is kept in the conversation.

## Step 5: Build, enable, report

Follow the tool order in `references/agent.md`: `validate_config` until it is clean,
`create_config`, then `set_config_enabled` with `enabled: true`. If enabling fails, fix
what it names and enable again; relay what you cannot fix in the server's words.

Then report, in a few lines:

- What is enabled, by name.
- When it runs next (for a schedule), or where it listens (the app or channel it answers
  in; for an HTTP or webhook address, send them to the agent's page in the dashboard to
  copy it).
- Where to see its runs: **Runs** in the dashboard, https://dash.servflow.io/runs.

## Rules

- **Enable last.** Saving only stores; `set_config_enabled` is what serves. A change to an
  enabled agent is not served until it is enabled again, so always enable after an
  update.
- **Agents only.** Always pass `kind: "agent"` to `list_configs`, and never use
  `get_config`, `update_config`, `delete_config` or `set_config_enabled` on a support
  agent's row: it would make the row disagree with the support agent and serve it without
  its guardrails. Support agents have their own tools.
- **Ask before touching what you did not make.** Never edit or delete an agent,
  integration or secret that you did not create in this session without asking first.
- **Errors in the server's words.** Relay plan-limit and credit errors as the server
  wrote them, and point to the page that fixes them: **Billing**, under **Settings > Team**
  in the dashboard (https://dash.servflow.io/settings/team#billing).
- **No secrets in the chat.** Never ask for an API key, token or password in the chat.
  The person saves those in the dashboard; you handle only their names.
- **Send them to the dashboard** for what MCP cannot do: connecting an OAuth app, running
  a test before enabling, and reading an agent's runs.

## Words

Use these with the person: agent, trigger, run, step, sub agent, integration,
provider/model, built-in model, memory, enable/enabled, credits, plan, team, dashboard.
Write "Servflow" with one capital. Do not say workflow, bot, chatbot, deploy or knowledge
base. Give no time estimates and use no marketing adjectives.
