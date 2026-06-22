package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-ooo/config"
	go_ooo_types "go-ooo/types"

	"github.com/spf13/cobra"
)

// chainFlag is the shared --chain selector (a name or network id; "all" fans across every chain) for
// the admin + query commands. Empty resolves to the sole chain when only one is configured.
var chainFlag string

// adminCmd represents the admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Run a sub-command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run one of the sub-commands. See 'go-ooo admin --help'")
	},
}

func init() {
	adminCmd.PersistentFlags().StringVar(&chainFlag, "chain", "", "target chain (name or network id; 'all' for every chain). Optional with a single chain configured")
	rootCmd.AddCommand(adminCmd)
}

// processAdminTask resolves the --chain selector to one (or all) configured chains, stamps the task's
// target network and sends it to the running daemon for each, printing every result.
func processAdminTask(adminTask go_ooo_types.AdminTask, cfg *config.Config) {
	targets, err := resolveTargetChains(cfg, chainFlag)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for _, ch := range targets {
		task := adminTask
		task.Network = ch.NetworkId
		if len(targets) > 1 {
			fmt.Printf("\n=== %s (network %d) ===\n", chainLabel(ch), ch.NetworkId)
		}
		sendAdminTask(task, cfg)
	}
}

// resolveTargetChains maps the --chain flag to the chains a task targets: "all" → every configured
// chain; otherwise the single resolved chain (or the sole chain when the flag is empty).
func resolveTargetChains(cfg *config.Config, selector string) ([]config.ChainConfig, error) {
	if strings.EqualFold(selector, "all") {
		return cfg.ChainList(), nil
	}
	ch, err := cfg.ResolveChain(selector)
	if err != nil {
		return nil, err
	}
	return []config.ChainConfig{ch}, nil
}

func chainLabel(ch config.ChainConfig) string {
	if ch.Name != "" {
		return ch.Name
	}
	return fmt.Sprintf("network %d", ch.NetworkId)
}

func sendAdminTask(adminTask go_ooo_types.AdminTask, cfg *config.Config) {

	fmt.Println("")
	fmt.Println("attempting to send task", adminTask.Task)
	fmt.Println("")

	statusCode, body, err := postAuthedTask(cfg, "/admin", adminTask)
	if err != nil {
		fmt.Println("Something went wrong.")
		fmt.Println(err.Error())
		return
	}

	if statusCode == 200 {
		var decodedResponse go_ooo_types.AdminTaskResponse
		if err = json.Unmarshal(body, &decodedResponse); err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println("Task    :", decodedResponse.Task)
		fmt.Println("Success :", decodedResponse.Success)
		if decodedResponse.Success {
			fmt.Println("Result  :", decodedResponse.Result)
		} else {
			fmt.Println("Error   :", decodedResponse.Error)
		}
	} else {
		fmt.Println("Error   :", statusCode)
		fmt.Println("Message :", string(body))
	}
}
