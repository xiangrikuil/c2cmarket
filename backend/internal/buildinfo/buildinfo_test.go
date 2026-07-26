package buildinfo

import "testing"

func TestCurrentReturnsInjectedValues(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildTime := BuildTime
	t.Cleanup(func() {
		Version = originalVersion
		GitCommit = originalCommit
		BuildTime = originalBuildTime
	})

	Version = " v1.2.3 "
	GitCommit = " 0123456789abcdef "
	BuildTime = " 2026-07-26T12:00:00Z "

	info := Current()
	if info.Version != "v1.2.3" {
		t.Fatalf("Version = %q, want v1.2.3", info.Version)
	}
	if info.GitCommit != "0123456789abcdef" {
		t.Fatalf("GitCommit = %q, want injected commit", info.GitCommit)
	}
	if info.BuildTime != "2026-07-26T12:00:00Z" {
		t.Fatalf("BuildTime = %q, want injected build time", info.BuildTime)
	}
}

func TestCurrentUsesExplicitDevelopmentFallbacks(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildTime := BuildTime
	t.Cleanup(func() {
		Version = originalVersion
		GitCommit = originalCommit
		BuildTime = originalBuildTime
	})

	Version = " "
	GitCommit = ""
	BuildTime = "\t"

	info := Current()
	if info.Version != "development" || info.GitCommit != "unknown" || info.BuildTime != "unknown" {
		t.Fatalf("unexpected fallback info: %+v", info)
	}
}
