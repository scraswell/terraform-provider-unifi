// Copyright 2026 Sean Craswell. SPDX-License-Identifier: MPL-2.0

// Package main defines the hermetic quality gates for the UniFi provider fork.
package main

import "dagger/unifi-provider/internal/dagger"

type UnifiProvider struct{}

const (
	baseImage   = "ghcr.io/jdx/mise:2026.8.8@sha256:638270210fb690efc0a8492b2c173d51bee6cb33f26381f05790d90c8e646a80"
	workDir     = "/work"
	miseDataDir = "/mise-data"
	goModCache  = "/go/pkg/mod"
	goCache     = "/go/build-cache"
)

// Base returns the pinned tool container shared by every task.
func (m *UnifiProvider) Base(
	// +ignore=[".git", ".terraform", "dist", "coverage.*", "junit.xml"]
	src *dagger.Directory,
) *dagger.Container {
	cleanSrc := src.
		WithoutDirectory(".git").
		WithoutDirectory(".terraform").
		WithoutDirectory("dist")

	return dag.Container().
		From(baseImage).
		WithEnvVariable("MISE_CONFIG_FILE", workDir+"/.dagger/mise.toml").
		WithEnvVariable("MISE_DATA_DIR", miseDataDir).
		WithEnvVariable("MISE_TRUSTED_CONFIG_PATHS", workDir).
		WithEnvVariable("MISE_YES", "1").
		WithEnvVariable("GOMODCACHE", goModCache).
		WithEnvVariable("GOCACHE", goCache).
		WithEnvVariable("CGO_ENABLED", "1").
		WithMountedCache(miseDataDir, dag.CacheVolume("unifi-provider-mise-v1")).
		WithMountedCache(goModCache, dag.CacheVolume("unifi-provider-go-mod-v1")).
		WithMountedCache(goCache, dag.CacheVolume("unifi-provider-go-build-v1")).
		WithDirectory(workDir, cleanSrc).
		WithWorkdir(workDir).
		WithExec([]string{"mise", "install"}).
		WithExec([]string{"mise", "exec", "--", "go", "mod", "download"})
}

// Format applies the provider's pinned Go formatters.
func (m *UnifiProvider) Format(src *dagger.Directory) *dagger.Directory {
	return m.Base(src).
		WithExec([]string{"mise", "exec", "--", "golangci-lint", "fmt"}).
		Directory(workDir)
}

// Lint checks formatting, static analysis, and secrets.
func (m *UnifiProvider) Lint(src *dagger.Directory) *dagger.Container {
	return m.Base(src).
		WithExec([]string{"mise", "exec", "--", "golangci-lint", "fmt", "--diff"}).
		WithExec([]string{"mise", "exec", "--", "golangci-lint", "run", "./..."}).
		WithExec([]string{"mise", "exec", "--", "gitleaks", "detect", "--no-git", "--no-banner", "--redact", "--verbose"})
}

// Test runs all unit tests with race detection and no result caching.
func (m *UnifiProvider) Test(src *dagger.Directory) *dagger.Container {
	return m.Base(src).
		WithExec([]string{
			"mise", "exec", "--", "go", "tool", "gotestsum", "--",
			"-race", "-count=1", "-timeout=120m", "./...",
		})
}

// Build compiles the provider executable.
func (m *UnifiProvider) Build(src *dagger.Directory) *dagger.Container {
	return m.Base(src).
		WithExec([]string{"mise", "exec", "--", "go", "build", "-o", "/out/terraform-provider-unifi", "."})
}

// Ci runs the complete hermetic quality gate.
func (m *UnifiProvider) Ci(src *dagger.Directory) *dagger.Container {
	return m.Base(src).
		WithExec([]string{"mise", "exec", "--", "golangci-lint", "fmt", "--diff"}).
		WithExec([]string{"mise", "exec", "--", "golangci-lint", "run", "./..."}).
		WithExec([]string{
			"mise", "exec", "--", "go", "tool", "gotestsum", "--",
			"-race", "-count=1", "-timeout=120m", "./...",
		}).
		WithExec([]string{"mise", "exec", "--", "go", "build", "-o", "/out/terraform-provider-unifi", "."}).
		WithExec([]string{"mise", "exec", "--", "gitleaks", "detect", "--no-git", "--no-banner", "--redact", "--verbose"})
}
