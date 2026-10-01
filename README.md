# ServFlow engine

This module is the library of actions and integrations that a ServFlow host
runs. It has no registries, planner, server, or binary of its own. [ServFlow](https://git.servflow.io/servflow/servflowai)
registers the actions and integrations it offers, compiles workflow configs,
and runs requests.

## Use an action or integration

Each action and integration package exports a `Definition` function that
describes it: its type, name, fields, and constructor. Nothing registers
itself. To offer one, a host calls `Definition` and adds the result to its own
registry:

```go
import (
	"github.com/Servflow/servflow/pkg/engine/actions/executables/fetch"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/mongo"
)

registry.RegisterAction(fetch.Definition())
registry.RegisterIntegration(mongo.Definition())
```

## Actions

The following actions live in `pkg/engine/actions/executables`:

| Type | Package |
|---|---|
| `delete` | `delete_action` |
| `fetch` | `fetch` |
| `fetchvectors` | `fetchvector` |
| `firestore` | `firestore` |
| `hash` | `hash` |
| `mongoquery` | `mongoquery` |
| `static` | `static` |
| `storevector` | `storevector` |

`stub` is a test helper; a host offers it only in tests.

## Integrations

The following integrations live in `pkg/engine/integration/integrations`:
`mongo`, `qdrant`, and `sql`.

## Services

Each of these packages holds a service's integration and, where it has them,
its actions:

| Service | Integration | Actions |
|---|---|---|
| Binance | `pkg/binance/integration` | `pkg/binance/actions`: `binance/getprice`, `binance/spotorder`, `binance/pricedifference`, `binance/tradeinfo`, `binance/accountbalance`, `binance/futuresorder` |

A package with several actions exports `Definitions`, which returns all of
them.

## The contract

Actions and integrations compile against these packages. A host implements or
supplies what they declare:

`pkg/engine/actions`
: The `ActionExecutable` and `ActionExecutableV2` interfaces, `Definition`, the
  field and output types, and `ErrFailure`, which an action wraps to mark a
  failure the run can recover from.

`pkg/engine/integration`
: The `Integration` interface, `Definition`, and the field types.

`pkg/engine/requestctx`
: The `RequestContext` interface an action reads request state through:
  variables, template resolution, secret scrubbing, files, the workspace, the
  HTTP request, and the request's integrations. The host implements it.
  `FileInput`, which names a file an action reads, lives here too.

`pkg/logging`
: Request-scoped logging.

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
