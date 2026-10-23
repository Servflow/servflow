# Templates

Each folder here is a template the ServFlow dashboard installs: a
`manifest.yaml` naming its inputs, required integrations, and the config files
it installs, plus those files.

The dashboard reads templates from `main`, so a change merged here reaches
every install without a release. Keep `main` installable: change a manifest
and the config files it names in the same pull request.

## Writing a template

Build the agent in the studio until it works, export it, and replace what belongs
to one installation with install-time templates. A config file is rendered with
Go's `text/template` using `<<` and `>>`, so the engine's own `{{ }}` templates
pass through untouched.

| Write | For |
|---|---|
| `<< .name >>` | an input's value |
| `<< .integrations.key >>` | the id of the integration chosen for a requirement |
| `<< .name \| json >>` | a value that may hold quotes, colons, or line breaks, such as pasted HTML |
| `<< .name \| indent 8 >>` | a value of several lines inside a YAML block scalar indented 8 spaces |
| `<< if >>`, `<< range >>` | shaping the file around the inputs |

Leave `providerID` and `workspaceID` out of every sub agent: the install sets
them from what the person chose. Use a `content` input for a document, such as an
email layout, with `formats` and `tags` for its editor. Require a Composio
toolkit with `group: composio:<toolkit>`, for example `composio:zoho_mail`.

Every template here is also a fixture in the control plane's drift guard, which
installs it against the current config model on every build.
