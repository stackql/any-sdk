package argparse

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/public/discovery"
)

var interrogateCmd = &cobra.Command{
	Use:   "interrogate",
	Short: "Provider interrogation with any-sdk",
	Long:  `Provider interrogation with any-sdk`,
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

		runInterrogateCommand(runtimeCtx, args...)
	},
}

func runInterrogateCommand(rtCtx dto.RuntimeCtx, args ...string) {
	if len(args) < 2 {
		printErrorAndExitOneIfError(fmt.Errorf("insufficient arguments"))
	}
	interrogationType := args[0]
	switch interrogationType {
	case "services":
		if len(args) != 3 {
			fmt.Fprintf(os.Stderr, "inoperable input; expected 'interrogate %s <path to registry root> <path to provider doc>'\n", interrogationType)
			os.Exit(1)
		}
		registryURL := args[1]
		providerDoc := args[2]
		interrogator := discovery.NewInterrogator(providerDoc, registryURL)
		providerServices, interrogatorErr := interrogator.GetProviderServices()
		printErrorAndExitOneIfError(interrogatorErr)
		for _, svc := range providerServices {
			fmt.Fprintln(os.Stdout, fmt.Sprint(svc))
		}
		os.Exit(0)
	default:
		// Handle other interrogation types
		//nolint:go-staticcheck // acceptable
		fmt.Fprintf(os.Stderr, "unknown interrogation type '%s'\n", interrogationType)
		os.Exit(1)
	}
}
