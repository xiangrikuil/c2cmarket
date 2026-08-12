package auth

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var capabilityLiteralPattern = regexp.MustCompile(`'([a-z]+(?:[._][a-z]+)+)'`)

func TestCapabilityContractMatchesOpenAPIAndGeneratedFrontend(t *testing.T) {
	repositoryRoot := capabilityContractRepositoryRoot(t)
	want := append([]string(nil), AllCapabilities...)
	sort.Strings(want)

	openAPI := readCapabilityContractFile(t, filepath.Join(repositoryRoot, "docs", "openapi", "c2c-market-api-v1.yaml"))
	generated := readCapabilityContractFile(t, filepath.Join(repositoryRoot, "frontend", "src", "api", "generated", "openapi", "types.gen.ts"))

	if got := openAPICapabilities(t, openAPI); !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenAPI Capability enum = %v, want %v", got, want)
	}
	if got := generatedCapabilities(t, generated); !reflect.DeepEqual(got, want) {
		t.Fatalf("generated frontend Capability union = %v, want %v", got, want)
	}

	for _, forbidden := range []string{"api_order.after_sales", "api_model_tester.use_owned_order"} {
		if strings.Contains(openAPI, forbidden) || strings.Contains(generated, forbidden) {
			t.Fatalf("forbidden global capability %q leaked into a cross-layer contract", forbidden)
		}
	}
}

func capabilityContractRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve capability contract test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
}

func readCapabilityContractFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func openAPICapabilities(t *testing.T, document string) []string {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(document))
	inCapabilitySchema := false
	values := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "    Capability:" {
			inCapabilitySchema = true
			continue
		}
		if !inCapabilitySchema {
			continue
		}
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") && strings.TrimSpace(line) != "" {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			values = append(values, strings.TrimPrefix(trimmed, "- "))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan OpenAPI capability schema: %v", err)
	}
	sort.Strings(values)
	return values
}

func generatedCapabilities(t *testing.T, document string) []string {
	t.Helper()
	const prefix = "export type Capability = "
	for _, line := range strings.Split(document, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		matches := capabilityLiteralPattern.FindAllStringSubmatch(line, -1)
		values := make([]string, 0, len(matches))
		for _, match := range matches {
			values = append(values, match[1])
		}
		sort.Strings(values)
		return values
	}
	t.Fatal("generated frontend Capability union not found")
	return nil
}
