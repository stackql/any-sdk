package argparse

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/dto"
)

// execCmd represents the exec command
var constCmd = &cobra.Command{
	Use:   "const",
	Short: "Simple textual any-sdk const display",
	Long:  `Simple textual any-sdk const display`,
	Run: func(cmd *cobra.Command, args []string) {

		if runtimeCtx.CPUProfile != "" {
			f, err := os.Create(runtimeCtx.CPUProfile)
			if err != nil {
				printErrorAndExitOneIfError(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		if len(args) > 0 {
			cmd.Help()
			os.Exit(0)
		}

		runConstCommand(runtimeCtx)
	},
}

func runConstCommand(rtCtx dto.RuntimeCtx) {
	constMap := map[string]interface{}{
		"ExtensionKeyAlwaysRequired": anysdk.ExtensionKeyAlwaysRequired,
	}
	rv, marshalErr := json.Marshal(constMap)
	if marshalErr != nil {
		printErrorAndExitOneIfError(marshalErr)
	}
	fmt.Fprintf(os.Stdout, "%s\n", string(rv))
}
