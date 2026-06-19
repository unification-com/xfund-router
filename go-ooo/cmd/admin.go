package cmd

import (
	"encoding/json"
	"fmt"

	"go-ooo/config"
	go_ooo_types "go-ooo/types"

	"github.com/spf13/cobra"
)

// adminCmd represents the admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Run a sub-command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run one of the sub-commands. See 'go-ooo admin --help'")
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)
}

func processAdminTask(adminTask go_ooo_types.AdminTask, cfg *config.Config) {

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
