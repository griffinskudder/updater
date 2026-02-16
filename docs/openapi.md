# OpenAPI Specification

The updater service ships with a machine-readable OpenAPI 3.0.3 specification embedded
directly in the binary. The spec is served at runtime alongside an interactive Swagger UI.

## Endpoints

| Path | Description |
|------|-------------|
| `GET /api/v1/openapi.yaml` | Raw OpenAPI 3.0.3 YAML specification |
| `GET /api/v1/docs` | Interactive Swagger UI (loads spec from CDN) |

Both endpoints are **public** and require no authentication.

## Accessing the Spec

### curl

Download the raw YAML:

```bash
curl http://localhost:8080/api/v1/openapi.yaml
```

Save it locally for offline use:

```bash
curl -o openapi.yaml http://localhost:8080/api/v1/openapi.yaml
```

### Postman

1. Open Postman and select **Import**.
2. Choose **Link** and paste `http://localhost:8080/api/v1/openapi.yaml`.
3. Postman will import all endpoints, parameters, and example request bodies.

### Code generators

The spec can be used with any OpenAPI 3.0-compatible code generator.

**Go client (oapi-codegen):**

```bash
oapi-codegen -package client -generate types,client openapi.yaml > client/client.gen.go
```

**TypeScript client (openapi-typescript):**

```bash
npx openapi-typescript http://localhost:8080/api/v1/openapi.yaml -o types.d.ts
```

**Python client (openapi-python-client):**

```bash
openapi-python-client generate --url http://localhost:8080/api/v1/openapi.yaml
```

## Interactive UI

Open `http://localhost:8080/api/v1/docs` in a browser to access Swagger UI. The UI loads
the spec from `/api/v1/openapi.yaml` and lets you browse every endpoint, inspect schemas,
and execute live requests against the running server.

> **Note:** The interactive UI loads Swagger UI from the unpkg CDN. An active internet
> connection is required. For offline use, download the raw YAML with `curl` and open it
> with a local Swagger viewer or import it into Postman.

## Validation

Validate the spec with Redocly CLI:

```bash
make openapi-validate
```

This runs `redocly/cli:latest` in Docker against `internal/api/openapi/openapi.yaml` and
reports any lint violations without requiring a local Node.js installation.

## Architecture

The spec is embedded in the binary using Go's `embed` package. No external files are
needed at runtime.

```mermaid
flowchart LR
    F[internal/api/openapi/openapi.yaml] -->|go:embed| B[handlers_openapi.go\nopenAPISpec []byte]
    B --> S[ServeOpenAPISpec\nGET /api/v1/openapi.yaml]
    B --> U[ServeSwaggerUI\nGET /api/v1/docs]
    S -->|application/yaml| C1[curl / Postman / codegen]
    U -->|text/html| C2[Browser + unpkg CDN]
```

## Spec Location

The source YAML is at `internal/api/openapi/openapi.yaml`. Edit it to add or update
endpoint documentation, then rebuild the binary so the new version is embedded:

```bash
make build
```
