package argparse

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/closure"
)

var closureCmd = &cobra.Command{
	Use:   "closure <registry> <provider-doc> <service>",
	Short: "Generate a method closure for a StackQL resource",
	Long: `Generate the minimal service document subset needed to action a specific resource.

Usage:
  closure <registry> <provider-doc> <service> --provider <name> --resource <name> [--rewrite-url <url>]

The closure YAML is written to stdout.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 3 {
			cmd.Help()
			os.Exit(0)
		}
		runClosureCommand(runtimeCtx, args[0], args[1], args[2])
	},
}

func initClosureFlags() {
	closureCmd.Flags().StringVar(&runtimeCtx.CLIRewriteURL, "rewrite-url", "", "rewrite all server URLs to this base URL")
	closureCmd.Flags().StringVar(&runtimeCtx.CLIProviderOut, "provider-out", "", "write a minimal provider YAML to this file (for use with the closure)")
}

func runClosureCommand(rtCtx dto.RuntimeCtx, registryRoot string, providerDoc string, serviceName string) {
	providerName := rtCtx.CLIProviderName
	resourceName := rtCtx.CLIResourceStr

	if providerName == "" {
		fmt.Fprintln(os.Stderr, "closure requires --provider flag")
		os.Exit(1)
	}

	// Read the provider doc to find the service $ref
	serviceDocPath, err := resolveServiceDocPath(registryRoot, providerDoc, serviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve service doc path: %v\n", err)
		os.Exit(1)
	}

	// Read raw service doc bytes
	serviceDocBytes, err := os.ReadFile(serviceDocPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read service doc: %v\n", err)
		os.Exit(1)
	}

	cfg := closure.ClosureConfig{
		ResourceName: resourceName,
		RewriteURL:   rtCtx.CLIRewriteURL,
	}

	closureBytes, err := closure.BuildClosure(serviceDocBytes, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build closure: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.Write(closureBytes)

	// Optional: write a minimal provider doc alongside the closure
	if rtCtx.CLIProviderOut != "" {
		providerBytes, provErr := generateMinimalProviderDoc(providerDoc, providerName, serviceName, rtCtx.CLIProviderOut)
		if provErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to generate provider doc: %v\n", provErr)
		} else {
			if wErr := os.MkdirAll(filepath.Dir(rtCtx.CLIProviderOut), 0o755); wErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to create provider output dir: %v\n", wErr)
			} else if wErr := os.WriteFile(rtCtx.CLIProviderOut, providerBytes, 0o644); wErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write provider doc: %v\n", wErr)
			}
		}
	}
}

// generateMinimalProviderDoc reads the original provider doc and produces a minimal
// version with only the specified service, referencing a closure service doc at the
// conventional path: <provider>/<version>/services/<service>.yaml
func generateMinimalProviderDoc(originalProviderDoc string, providerName string, serviceName string, providerOutPath string) ([]byte, error) {
	provBytes, err := os.ReadFile(originalProviderDoc)
	if err != nil {
		return nil, err
	}
	var prov map[string]interface{}
	if err := yaml.Unmarshal(provBytes, &prov); err != nil {
		return nil, err
	}

	// Extract original service entry and auth config
	services, _ := prov["providerServices"].(map[string]interface{})
	originalSvc, _ := services[serviceName].(map[string]interface{})

	version, _ := prov["version"].(string)
	if version == "" {
		version = "v0.1.0"
	}

	// Build the service ref path relative to the registry src/ dir
	svcRefPath := fmt.Sprintf("%s/%s/services/%s.yaml", providerName, version, serviceName)

	svcEntry := map[string]interface{}{
		"id":        fmt.Sprintf("%s:%s", serviceName, version),
		"name":      serviceName,
		"preferred": true,
		"service": map[string]interface{}{
			"$ref": svcRefPath,
		},
		"title":   serviceName,
		"version": version,
	}
	// Preserve description from original if present
	if originalSvc != nil {
		if desc, ok := originalSvc["description"]; ok {
			svcEntry["description"] = desc
		}
		if title, ok := originalSvc["title"]; ok {
			svcEntry["title"] = title
		}
	}

	minimal := map[string]interface{}{
		"id":      providerName,
		"name":    providerName,
		"version": version,
		"openapi": "3.0.0",
		"providerServices": map[string]interface{}{
			serviceName: svcEntry,
		},
	}

	// Copy auth config from original
	if config, ok := prov["config"]; ok {
		minimal["config"] = config
	}

	return yaml.Marshal(minimal)
}

// resolveServiceDocPath finds the service YAML file path by reading the
// provider doc and extracting the service.$ref for the given service name.
func resolveServiceDocPath(registryRoot string, providerDoc string, serviceName string) (string, error) {
	provBytes, err := os.ReadFile(providerDoc)
	if err != nil {
		return "", fmt.Errorf("cannot read provider doc: %w", err)
	}

	var prov map[string]interface{}
	if err := yaml.Unmarshal(provBytes, &prov); err != nil {
		return "", fmt.Errorf("cannot parse provider doc: %w", err)
	}

	services, ok := prov["providerServices"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("provider doc has no providerServices")
	}

	svc, ok := services[serviceName].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("service '%s' not found in provider", serviceName)
	}

	serviceRef, ok := svc["service"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("service '%s' has no service.$ref", serviceName)
	}

	ref, ok := serviceRef["$ref"].(string)
	if !ok {
		return "", fmt.Errorf("service '%s' service.$ref is not a string", serviceName)
	}

	// The ref is relative to the registry src/ directory
	return filepath.Join(registryRoot, "src", ref), nil
}
