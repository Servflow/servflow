---
name: create-support-agent
description: >-
  Create and enable a Servflow support agent that answers a business's customers on its
  website's live chat, through the Servflow MCP connector: it researches the business
  from its own website, drafts the files, instructions, guardrails and live chat from what
  the site says, shows them for approval, then builds, enables and gives the embed code.
  Use it whenever someone wants Servflow to answer their customers or buyers, add a
  support or help chat to their site, or handle questions about their prices, delivery or
  returns, even if they don't say "support agent".
argument-hint: "for my bike shop | for example.com"
---

# Create a Servflow support agent

You turn a business's name and website into an enabled support agent on its live chat,
using the Servflow MCP tools. Everything the support agent tells customers comes from what
you write here, so the facts must come from the business itself: its website, and the
person's corrections.

The request is in `$ARGUMENTS`; it may be empty or only partly filled in.

## Step 1: Check the connector

Call `list_configs` with `kind: "agent"`. If it returns a list, carry on. If the tool is
missing or the call is refused, read `../create/references/connect.md`, tell the person
what to do, and stop until they say it is done.

Then call `list_support_agents`. If the team already has one, say so before the research:
on the Free plan a team holds one support agent, and a second is refused. No tool says
which plan the team is on, so ask only if it matters; the person sees it on the dashboard's
**Billing** page.

## Step 2: Ask for the business

Ask once, in one message, for the two things you need, leaving out what `$ARGUMENTS`
already gives:

> What is the company called, and what is its website?

## Step 3: Research the website

Read `references/research.md` and follow it. If your client can run a subagent (Claude
Code's Agent or Task tool), give the research to one, with `references/research.md` and
the company's name and website, and take back only its findings; this keeps the page
contents out of this conversation. Otherwise do the research here.

Never invent or infer a policy, price, time or promise. A topic the site does not cover
is **not found** and becomes a question for the person. If the site cannot be read, say
so and ask the person to paste their answers on prices, delivery, returns and contact.

## Step 4: Draft

Read `references/support.md` now; it has the shapes, the limits and how a reply is
checked. Draft four things from the research:

- **Files**: one text file per topic (`about.md`, `pricing.md`, `delivery.md`,
  `returns.md`, `contact.md`), in plain statements a customer can be answered from, each
  ending with its source URLs. Give each file policy labels so a question loads the right
  files; leave `about.md` and `contact.md` without labels so every answer has them.
- **Instructions**: who the support agent answers for, in the site's own voice; what it
  helps with; what to ask a buyer before suggesting something (size, use, budget); and
  to say it doesn't know, and how to reach the team, when the files do not answer.
- **Guardrails**: `answers_from_business`, plus only those the business clearly needs from
  what the site shows, each a name and a one-line rule. Then the **fallback message** a
  customer gets when a reply is stopped.
- **Live chat**: title (the company name), color (the site's brand color if found),
  greeting, websites (the company's domain), and a hello bubble.

On the Free plan there are no files. If you know the team is on Free, put the facts in
the instructions and in the `answers_from_business` guardrail as `references/support.md`
says, and tell the person that files need a paid plan.

## Step 5: Propose, then wait

Show four blocks, in this order:

1. **Files**: a table of file, policy labels, what it covers, and source pages. Under it,
   the **Not found on your site** list, as questions. If the team has other support
   agents, say that they will answer from these files too: files are the team's.
2. **Instructions**: the full text.
3. **Guardrails**: each name and rule, then the fallback message.
4. **Live chat**: title, color, greeting, websites, hello bubble.

Offer to show any file in full. Ask for a yes or corrections, and wait. Create nothing
before the yes. The person's corrections and answers win over the website: update the
draft and show what changed. Their answers to the not-found questions go into the files.

## Step 6: Build, enable, report

1. `add_support_file` for each approved file (skip on Free). If the first one is refused
   because files are not on the team's plan, relay the server's message, show the Free
   version of the instructions and guardrails (the facts folded in), and wait for a yes
   before going on.
2. `create_support_agent` with one channel, `{"type": "livechat"}`.
3. `get_livechat`, then `set_livechat` with every field: `agent_id` set to the new support
   agent's `id`. If the live chat already answers with another support agent, ask before
   switching it; keep the websites already listed.
4. `set_support_agent_enabled` with `enabled: true`. If it fails, fix what it names and
   enable again.

Then report, in a few lines:

- The support agent is enabled and answers the live chat on the websites listed.
- The files added, and that each is ready once `list_support_files` shows `ready`.
- The embed code: tell them to copy the one line from the dashboard's **Live chat** page,
  section **Embed** (https://dash.servflow.io/support/livechat), and paste it just before
  `</body>` on every page. No tool returns the team's address, so never write the line
  out with a guessed address.
- Where to read conversations: **Conversations** in the dashboard,
  https://dash.servflow.io/support/conversations.

## Rules

- **Enable last.** Saving only stores; `set_support_agent_enabled` is what serves. A
  change to an enabled support agent is not served until it is enabled again.
- **Support tools only.** Never use `get_config`, `update_config`, `delete_config` or
  `set_config_enabled` on a support agent's row: it would make the row disagree with the
  support agent and serve it without its guardrails. Always pass `kind: "agent"` to
  `list_configs`.
- **Full replaces.** `update_support_agent` and `set_livechat` replace the whole object:
  read it first (`get_support_agent`, `get_livechat`) and send everything back with the
  change.
- **Ask before touching what you did not make.** Never edit or delete a support agent,
  file or live chat setting you did not create in this session without asking.
- **Errors in the server's words.** Relay plan-limit and credit errors as the server
  wrote them, with the page that fixes them: **Billing**, under **Settings > Team** in the
  dashboard (https://dash.servflow.io/settings/team#billing). Free holds one support agent,
  live chat only, and no files.
- **Never state a limit on how many files a team can have.** If the server says the team
  is at its limit, relay that as it is.
- **No secrets in the chat.** Never ask for a key, token or password in the chat.
- **Send them to the dashboard** for what MCP cannot do: connecting an OAuth app, testing
  before enabling, reading conversations, deleting a support agent or a file, changing a
  file's policy labels, and uploading PDFs or other files that are not text.

## Words

Use these with the person: support agent, channel, live chat, files, policy label,
guardrail, handoff, fallback message, conversation, customer, enable/enabled, credits,
plan, team, dashboard. Write "Servflow" with one capital. Do not say workflow, bot,
chatbot, deploy or knowledge base. Give no time estimates and use no marketing
adjectives.
