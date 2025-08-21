package argparse

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/discovery"
)

var aotCmd = &cobra.Command{
	Use:   "aot",
	Short: "Provider AOT analysis with any-sdk",
	Long:  `Provider AOT analysis with any-sdk`,
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

func runAotCommand(rtCtx dto.RuntimeCtx, registryURL string, providerDoc string, extraArgs ...string) {

	analyzerFactory := discovery.NewSimpleSQLiteAnalyzerFactory(registryURL, rtCtx)
	var analyzer discovery.StaticAnalyzer
	var factoryErr error
	switch len(extraArgs) {
	case 1:
		analyzer, factoryErr = analyzerFactory.CreateServiceLevelStaticAnalyzer(providerDoc, extraArgs[0])
		printErrorAndExitOneIfError(factoryErr)
	case 0:
		analyzer, factoryErr = analyzerFactory.CreateStaticAnalyzer(providerDoc)
		printErrorAndExitOneIfError(factoryErr)
	default:
		fmt.Fprintf(os.Stderr, "inoperable input '%v'\n", extraArgs)
		os.Exit(1)
	}
	analyisErr := analyzer.Analyze()
	if analyisErr != nil {
		allErrs := analyzer.GetErrors()
		for _, err := range allErrs {
			fmt.Fprintln(os.Stderr, fmt.Sprint(err.Error()))
		}
	}
	if rtCtx.VerboseFlag {
		for _, affirmative := range analyzer.GetAffirmatives() {
			fmt.Fprintln(os.Stderr, fmt.Sprint(affirmative))
		}
	}
	printErrorAndExitOneIfError(analyisErr)
	fmt.Fprintf(os.Stdout, "\nsuccessfully performed AOT analysis on providerDoc = '%s'\n", providerDoc)
}
