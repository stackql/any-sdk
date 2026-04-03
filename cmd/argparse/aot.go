package argparse

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stackql/any-sdk/internal/anysdk"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/discovery"
	"github.com/stackql/stackql-provider-registry/signing/Ed25519/app/edcrypto"
)

var aotCmd = &cobra.Command{
	Use:   "aot <registry> <provider-doc> [service]",
	Short: "Provider AOT analysis with any-sdk",
	Long: `Provider AOT analysis with any-sdk.

Granularity is determined by positional arguments and flags:
  aot <registry> <provider-doc>                                                    provider level
  aot <registry> <provider-doc> <service>                                          service level
  aot <registry> <provider-doc> <service> --provider <name> --resource <name>      resource level
  aot <registry> <provider-doc> <service> --provider <name> --resource <name> --method <name>  method level`,
	Run: func(cmd *cobra.Command, args []string) {

		if runtimeCtx.CPUProfile != "" {
			f, err := os.Create(runtimeCtx.CPUProfile)
			if err != nil {
				printErrorAndExitOneIfError(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		if len(args) == 0 || args[0] == "" {
			cmd.Help()
			os.Exit(0)
		}

		runAotCommand(runtimeCtx, args[0], args[1], args[2:]...)
	},
}

func getNewLocalRegistry(relativePath string) (anysdk.RegistryAPI, error) {
	cleanPath := filepath.ToSlash(relativePath)
	if !strings.HasPrefix(cleanPath, "/") && !strings.HasPrefix(cleanPath, "./") {
		cleanPath = "./" + cleanPath
	}
	return anysdk.NewRegistry(
		anysdk.RegistryConfig{
			RegistryURL:      fmt.Sprintf("file://%s", cleanPath),
			LocalDocRoot:     relativePath,
			AllowSrcDownload: false,
			VerifyConfig: &edcrypto.VerifierConfig{
				NopVerify: true,
			},
		},
		nil)
}

func runAotCommand(rtCtx dto.RuntimeCtx, registryURL string, providerDoc string, extraArgs ...string) {

	analyzerFactoryFactory := discovery.NewStandardStaticAnalyzerFactoryFactory()
	registry, registryErr := getNewLocalRegistry(registryURL)
	printErrorAndExitOneIfError(registryErr)
	analyzerFactory, factoryFactoryErr := analyzerFactoryFactory.CreateNaiveSQLiteStaticAnalyzerFactory(registry, rtCtx)
	printErrorAndExitOneIfError(factoryFactoryErr)
	var analyzer discovery.StaticAnalyzer
	var factoryErr error

	// Flags override positional args for resource and method level analysis
	providerName := rtCtx.CLIProviderName
	resourceName := rtCtx.CLIResourceStr
	methodName := rtCtx.CLIMethodName

	switch {
	case methodName != "" && resourceName == "":
		fmt.Fprintln(os.Stderr, "--method requires --resource")
		os.Exit(1)
	case resourceName != "" && providerName == "":
		fmt.Fprintln(os.Stderr, "--resource requires --provider")
		os.Exit(1)
	case resourceName != "" && len(extraArgs) < 1:
		fmt.Fprintln(os.Stderr, "--resource requires a service positional arg")
		os.Exit(1)
	case resourceName != "" && methodName != "":
		analyzer, factoryErr = analyzerFactory.CreateMethodAggregateStaticAnalyzer(providerDoc, providerName, extraArgs[0], resourceName, methodName, false)
		printErrorAndExitOneIfError(factoryErr)
	case resourceName != "":
		analyzer, factoryErr = analyzerFactory.CreateResourceAggregateStaticAnalyzer(providerDoc, providerName, extraArgs[0], resourceName)
		printErrorAndExitOneIfError(factoryErr)
	case len(extraArgs) == 1:
		analyzer, factoryErr = analyzerFactory.CreateServiceLevelStaticAnalyzer(providerDoc, extraArgs[0])
		printErrorAndExitOneIfError(factoryErr)
	case len(extraArgs) == 0:
		analyzer, factoryErr = analyzerFactory.CreateStaticAnalyzer(providerDoc)
		printErrorAndExitOneIfError(factoryErr)
	default:
		fmt.Fprintf(os.Stderr, "inoperable input '%v'\n", extraArgs)
		os.Exit(1)
	}
	analyisErr := analyzer.Analyze()
	if analyisErr != nil {
		fmt.Fprintln(os.Stderr, discovery.FormatLogEntryJSON("error", analyisErr.Error()))
	}

	allErrs := analyzer.GetErrors()
	allWarnings := analyzer.GetWarnings()
	allAffirmatives := analyzer.GetAffirmatives()

	// Collect structured findings if available
	var findings []discovery.AnalysisFinding
	if fa, ok := analyzer.(discovery.FindingsAware); ok {
		findings = fa.GetFindings()
	}

	// stderr: JSONL log entries — use structured findings where available
	if len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintln(os.Stderr, discovery.FormatFindingJSON(f))
		}
	} else {
		for _, err := range allErrs {
			fmt.Fprintln(os.Stderr, discovery.FormatLogEntryJSON("error", err.Error()))
		}
		for _, warning := range allWarnings {
			fmt.Fprintln(os.Stderr, discovery.FormatLogEntryJSON("warning", warning))
		}
	}
	if rtCtx.VerboseFlag {
		for _, affirmative := range allAffirmatives {
			fmt.Fprintln(os.Stderr, discovery.FormatLogEntryJSON("info", affirmative))
		}
	}

	// stdout: JSON summary
	fmt.Fprintln(os.Stdout, discovery.FormatSummaryJSON(allErrs, allWarnings, allAffirmatives, findings))

	// Optional: write individual Python mock files
	if rtCtx.CLIMockOutputDir != "" && len(findings) > 0 {
		if mockErr := discovery.WriteMockFiles(findings, rtCtx.CLIMockOutputDir); mockErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write mock files: %v\n", mockErr)
		}
	}

	if analyisErr != nil {
		os.Exit(1)
	}
}
