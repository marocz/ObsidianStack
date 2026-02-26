// Package security checks TLS certificate validity and API key reachability
// for each configured source endpoint. It emits CertStatus records that are
// included in the PipelineSnapshot shipped to the server.
package security
