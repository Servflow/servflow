# ServFlow engine

This module is the library of actions and integrations that a ServFlow host
runs. It has no planner, server, or binary of its own. [ServFlow](https://git.servflow.io/servflow/servflowai)
compiles workflow configs, runs requests, and calls the actions registered here.

## Use an action or integration

Each action and integration registers itself when its package is imported. To
make one available to a host, import it for its side effect:

```go
import (
	_ "github.com/Servflow/servflow/pkg/engine/actions/executables/http"
	_ "github.com/Servflow/servflow/pkg/engine/integration/integrations/mongo"
)
```

## Actions

The following actions live in `pkg/engine/actions/executables`:

| Type | Package |
|---|---|
| `authenticate` | `authenticate` |
| `delete` | `delete_action` |
| `download` | `download` |
| `email` | `email` |
| `fetch` | `fetch` |
| `fetchvectors` | `fetchvector` |
| `firestore` | `firestore` |
| `get_key` | `get_key` |
| `hash` | `hash` |
| `http` | `http` |
| `javascript` | `javascript` |
| `jwt` | `jwt` |
| `mongoquery` | `mongoquery` |
| `save` | `save` |
| `static` | `static` |
| `store_key` | `store_key` |
| `storevector` | `storevector` |

`stub` is a test helper and isn't offered in the catalog.

## Integrations

The following integrations live in `pkg/engine/integration/integrations`:
`mongo`, `qdrant`, and `sql`.

## The contract

Actions and integrations compile against these packages. A host implements or
supplies what they declare:

`pkg/engine/actions`
: The action registry, the `ActionExecutable` and `ActionExecutableV2`
  interfaces, and `ErrFailure`, which an action wraps to mark a failure the run
  can recover from.

`pkg/engine/integration`
: The integration registry and manager.

`pkg/engine/requestctx`
: The `RequestContext` interface an action reads request state through:
  variables, template resolution, secret scrubbing, files, the workspace, and
  the HTTP request. The host implements it.

`pkg/engine/secrets`
: Secret lookup, backed by storage the host adds.

`pkg/engine/kv`
: The key-value store behind `get_key` and `store_key`. The host sets it with
  `kv.SetStore`.

`pkg/logging`
: Request-scoped logging.

`pkg/apiconfig`
: The config types an action or integration reads: file inputs and
  integration configs.

## Develop

Run the tests and the linters:

```bash
go test ./...
make lint
```

Install the commit-message hook:

```bash
make hooks
```

## License

See [LICENSE](LICENSE).
