// Package docs embeds the API specification so the binary can serve it.
//
// Embedding rather than reading from disk keeps the container image
// self-contained: the running API and the spec it publishes cannot drift apart,
// because they are the same artefact.
package docs

import _ "embed"

// OpenAPI is the specification served at /api/v1/openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPI []byte

// SwaggerUI is the page served at /docs. It renders OpenAPI in a browser.
//
//go:embed swagger.html
var SwaggerUI []byte
