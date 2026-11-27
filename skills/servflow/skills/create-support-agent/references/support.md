# Support agent payloads

Read this at the build step of `create-support-agent`. Shapes and limits are the server's;
a payload outside them is refused with a message that names the field, so fix that field
and send it again.

## How a reply is checked

This decides how you write the files, the instructions and the guardrails:

- Each customer message, the server asks a model which of the team's **policy labels** the
  message is about ("Does this customer message ask about "returns"?"). The files with a
  matching label load, plus every file with **no** label. Labels a conversation has matched
  stay loaded for the rest of it.
- The support agent answers with its **instructions** (the system prompt) and the loaded
  files.
- Every **guardrail** then checks the reply before the customer sees it. A guardrail's
  checker reads the customer's message, the tools the agent called and what they returned,
  the reply, and the loaded files. **It does not read the instructions.** A reply that
  states a fact found only in the instructions can be blocked by `answers_from_business`.
- A blocked reply is written once more with the reason; if it is blocked again, the
  customer gets the **fallback message**.

Files are the team's, not one support agent's: every support agent of the team answers
from every ready file.

## `create_support_agent` and `update_support_agent`

Both take `support_agent`. `update_support_agent` also takes `support_agent_id` and
replaces the whole support agent, so read it with `get_support_agent` first and send every
field back with your change.

```json
{
  "name": "Lumen Cycles support",
  "fallback_message": "Thanks for your message. Someone from Lumen Cycles will reply by email soon.",
  "channels": [ { "type": "livechat" } ],
  "agent": {
    "provider_id": "native",
    "system_prompt": "..."
  },
  "tools": [],
  "handoffs": [],
  "guardrails": [
    { "name": "answers_from_business", "instructions": "..." }
  ]
}
```

| Field | Rule |
|---|---|
| `name` | Required, up to 100 characters. |
| `fallback_message` | Up to 1000 characters. Empty sends `Thanks for your message. A member of our team will reply soon.` |
| `channels` | 1 to 5. In this skill, exactly `[{"type": "livechat"}]`: a live chat channel has no integration. |
| `agent.provider_id` | `"native"` for the built-in model, or a `providerId` from `list_providers`. |
| `agent.system_prompt` | Required. |
| `tools`, `handoffs` | Send `[]`. Handoffs and tools are set up in the dashboard. |
| `guardrails` | 1 to 5, names unique. Each `instructions` is required, up to 2000 characters. On create, an empty list gets `answers_from_business`; on update it must hold at least one. |

`id`, `created_at` and `updated_at` are ignored on a save. `create_support_agent` returns
the support agent with its `id`: that is the `support_agent_id` for the other tools and the
`agent_id` for the live chat.

### The default guardrail

```json
{
  "name": "answers_from_business",
  "instructions": "The reply answers only from what the business told the agent and from what its tools returned. It does not invent facts, prices, policies, dates or promises, and it does not guess when it does not know."
}
```

Keep it, and add others only for what the business clearly needs (no medical advice, no
price for an unreleased product, no promise of a delivery date).

## `set_support_agent_enabled`

`support_agent_id`, `enabled: true`. Enabling checks the plan (Free holds one support
agent), that no other enabled support agent answers on the same integration, and builds it;
it returns why when it cannot. An enabled support agent keeps answering with its last
enabled version until it is enabled again, so enable again after every update.

## `add_support_file`

| Input | Rule |
|---|---|
| `name` | Ends in `.txt`, `.md`, `.csv` or `.html`. Anything else is refused over MCP. |
| `text` | The file's contents, at most 25 MB. |
| `policy_labels` | Optional. Each 1 to 60 characters, at most 10 per file; repeats that differ only in case are dropped. Leave it out for a file every answer should load. |

- The file is saved `uploaded`, then `processing`, then `ready`; a file Servflow can't read
  is removed. `list_support_files` shows each file's status and labels. Only ready files
  are loaded.
- All files loaded for one message together must stay under 200,000 characters, so keep
  each file to one topic and label it rather than writing one large unlabelled file.
- Write labels as the kind of question they answer, in plain words: `returns`,
  `delivery`, `pricing`, `warranty`. Reuse the labels `list_support_files` already shows.
- Free has no files. The call returns `support_files is not on the free plan. Upgrade the
  plan on the dashboard's Billing page.` (or the same with the team's plan name).
- When the team is at its file limit, the call says so. Relay it as it is and send the
  person to the dashboard's **Files** page; never state a number of files.

Example:

```json
{
  "name": "returns.md",
  "text": "# Returns and refunds\n\n- You can return a bike or part within 30 days of delivery, unused and in its original packaging.\n- Start a return by emailing returns@lumencycles.com with your order number.\n- Refunds go back to the original payment method within 5 working days of the return arriving.\n\nSources: https://lumencycles.com/returns, https://lumencycles.com/faq",
  "policy_labels": ["returns", "refunds", "exchanges"]
}
```

## `get_livechat` and `set_livechat`

A team has **one** live chat. `set_livechat` replaces it whole, so call `get_livechat`
first and send every field back with your change.

- If `agent_id` is already set to another support agent, ask the person before switching
  it: the chat on their site would start answering with the new support agent.
- Keep the `websites` already listed and add the new ones, unless the person says
  otherwise.

```json
{
  "agent_id": "lumen-cycles-support-a1b2",
  "title": "Lumen Cycles",
  "color": "#0F6B5C",
  "greeting": "Hi, how can we help with your bike today?",
  "websites": ["lumencycles.com", "www.lumencycles.com"],
  "logo_url": "",
  "hello": { "on": true, "style": "bubble", "delay_seconds": 5 },
  "page_greeting": { "on": false, "note": "" }
}
```

| Field | Rule |
|---|---|
| `agent_id` | A support agent with a live chat channel. |
| `title` | The name on the chat, 1 to 40 characters. |
| `color` | `#RRGGBB`. |
| `greeting` | The first message, 1 to 300 characters. |
| `websites` | 1 to 10 hosts that load the chat, no `https://` and no path: `lumencycles.com`. A host with and without `www.` are two entries. No repeats. |
| `logo_url` | Empty (the chat shows the title's first letter) or an `https` link to an image. |
| `hello` | `on`, `style` (`bubble` or `open`), `delay_seconds` (0, 5, 10, 30 or 60). |
| `page_greeting` | `on`, `note` up to 500 characters. Leave it off in this skill. |

The chat answers only while its support agent is enabled. Customers see a change on their
next message or page load.

## The embed script

The code a website pastes is one line, on the team's own host:

```html
<script src="https://<your-team>.<agents-host>/_servflow/livechat/widget.js" async></script>
```

No MCP tool returns the team's host, so do not fill it in yourself: a wrong host gives a
page with no chat and no error. Tell the person to copy the exact line from the dashboard:
the **Live chat** page in the support agents side (https://dash.servflow.io/support/livechat),
section **Embed**, and paste it just before `</body>` on every page where the chat should
show. The same line keeps working when they change the chat or the support agent behind it.

## Worked example: a full build on a paid plan

1. `add_support_file` for each approved file (as above).
2. `create_support_agent`:

```json
{
  "name": "Lumen Cycles support",
  "fallback_message": "Thanks for your message. Someone from Lumen Cycles will reply by email soon.",
  "channels": [ { "type": "livechat" } ],
  "agent": {
    "provider_id": "native",
    "system_prompt": "You answer customers of Lumen Cycles, a bike shop in Leeds that sells city and e-bikes online and services bikes in its workshop. Write the way the shop's site does: friendly, plain and brief, two or three short sentences.\n\nYou help with: which bike suits someone, sizes, prices, delivery, returns and refunds, warranty, workshop bookings and opening hours. Answer only from the Lumen Cycles files you are given.\n\nWhen someone asks which bike or size to choose, ask how they will ride and their height before you suggest one.\n\nIf the files do not answer the question, say you are not sure and that the team can help at hello@lumencycles.com. Never guess a price, a date or a policy."
  },
  "tools": [],
  "handoffs": [],
  "guardrails": [
    {
      "name": "answers_from_business",
      "instructions": "The reply answers only from what the business told the agent and from what its tools returned. It does not invent facts, prices, policies, dates or promises, and it does not guess when it does not know."
    },
    {
      "name": "no_repair_diagnosis",
      "instructions": "The reply does not tell the customer a bike is safe to ride or diagnose a fault. For a fault or safety worry, it suggests booking the workshop."
    }
  ]
}
```

3. `get_livechat`, then `set_livechat` with the body above and `agent_id` set to the new
   support agent's `id`.
4. `set_support_agent_enabled` with `enabled: true`.

## On the Free plan

Free holds one support agent, live chat only, and no files. Put the researched facts in
the instructions instead of files, **and** append them to the `answers_from_business`
instructions after its default text, as "The business told the agent these facts, so a
reply may state them:" followed by the facts. The guardrail does not read the
instructions, so without that a correct reply quoting them can be blocked. Keep within
2000 characters for that guardrail; if the facts do not fit, keep the ones customers ask
about most and tell the person files need a paid plan.
