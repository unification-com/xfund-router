package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/params"
	"github.com/spf13/cobra"
	"go-ooo/config"
	"go-ooo/server"
	go_ooo_types "go-ooo/types"
	"golang.org/x/term"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"syscall"
)

var (
	aNumTxs         int
	aConsumer       string
	aCurrXfundPrice float64
	aSimGasPrice    uint64
	aSimXfundFee    float64
)

// analyticsCmd represents the analytics command
var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Run analytics",
	Run: func(cmd *cobra.Command, args []string) {

		if aNumTxs <= 0 {
			aNumTxs = 100
		}

		isSim := false
		if aSimXfundFee > 0 && aSimGasPrice > 0 {
			isSim = true
		}

		currXfundPrice := float64(0)
		if aCurrXfundPrice > 0 {
			currXfundPrice = aCurrXfundPrice
		} else {
			prices := getXfundPrice()
			currXfundPrice = prices.Xfund.Eth
		}

		simXfundFee := float64(0)
		if aSimXfundFee > 0 {
			simXfundFee = aSimXfundFee
		}

		analyticsTask := go_ooo_types.AnalyticsTask{
			Consumer:       aConsumer,
			NumTxs:         aNumTxs,
			CurrXfundPrice: currXfundPrice,
			Simulate:       isSim,
			SuggestFee:     false,
			SimulationParams: go_ooo_types.AnalyticsSimulationParams{
				GasPrice: aSimGasPrice,
				XfundFee: simXfundFee,
			},
		}
		srvCtx := server.GetServerContextFromCmd(cmd)

		processAnalyticsTask(analyticsTask, srvCtx.Config)
	},
}

// suggestFeeCmd represents the analytics command
var suggestFeeCmd = &cobra.Command{
	Use:   "suggestFee",
	Short: "suggest xFUND fee",
	Run: func(cmd *cobra.Command, args []string) {

		if aNumTxs <= 0 {
			aNumTxs = 100
		}

		currXfundPrice := float64(0)
		if aCurrXfundPrice > 0 {
			currXfundPrice = aCurrXfundPrice
		} else {
			prices := getXfundPrice()
			currXfundPrice = prices.Xfund.Eth
		}

		analyticsTask := go_ooo_types.AnalyticsTask{
			NumTxs:         aNumTxs,
			CurrXfundPrice: currXfundPrice,
			Simulate:       false,
			SuggestFee:     true,
		}

		srvCtx := server.GetServerContextFromCmd(cmd)

		processAnalyticsTask(analyticsTask, srvCtx.Config)
	},
}

func init() {

	analyticsCmd.PersistentFlags().IntVar(&aNumTxs, "num-txs", 100, "number of Txs to analyse")
	analyticsCmd.PersistentFlags().Float64Var(&aCurrXfundPrice, "xfund-price", 0.0, "Current xFUND price in ETH")
	analyticsCmd.PersistentFlags().StringVar(&aConsumer, "consumer", "", "filter by consumer contract address")
	analyticsCmd.Flags().Uint64Var(&aSimGasPrice, "sim-gas-price", 0, "simulated gas prices in gwei")
	analyticsCmd.Flags().Float64Var(&aSimXfundFee, "sim-xfund-fee", 0.0, "simulated xFUND fee")

	analyticsCmd.AddCommand(suggestFeeCmd)

	rootCmd.AddCommand(analyticsCmd)
}

func processAnalyticsTask(task go_ooo_types.AnalyticsTask, cfg *config.Config) {
	fmt.Print("Enter your password:	")

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	pass := strings.TrimSpace(string(bytePassword))

	fmt.Println("")
	fmt.Println("attempting to send analytics task")
	fmt.Println("")

	requestJSON, err := json.Marshal(task)
	if err != nil {
		fmt.Println("Can't marshal request")
		return
	}
	request := bytes.NewBuffer(requestJSON)
	url := fmt.Sprintf("http://%s:%s", cfg.Serve.Host, cfg.Serve.Port)

	req, err := http.NewRequest("POST", fmt.Sprint(url, "/analytics"), request)

	bearer := "Bearer " + pass
	req.Header.Add("Authorization", bearer)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("Something went wrong.")
		fmt.Println(err.Error())
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	if resp.StatusCode == 200 {
		var decodedResponse go_ooo_types.AnalyticsTaskResponse
		err = json.Unmarshal(body, &decodedResponse)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		var prettyJSON bytes.Buffer
		err = json.Indent(&prettyJSON, body, "", "  ")
		if err != nil {
			fmt.Println("JSON parse error: ", err)
			return
		}
		if decodedResponse.Task.SuggestFee {
			usableSuggestedFee := new(big.Float).Mul(big.NewFloat(decodedResponse.Result.SuggestedFee), big.NewFloat(params.GWei))
			fee, _ := usableSuggestedFee.Uint64()
			fmt.Println("Suggested fee              :", decodedResponse.Result.SuggestedFee, "xFUND")
			fmt.Println("Fee in lowest denomination :", fee)
			fmt.Println("Profit/Loss                :", fmt.Sprintf("%.18f", decodedResponse.Result.Earnings.ProfitLossEth), "ETH")
		} else {
			fmt.Println(string(prettyJSON.Bytes()))
		}
	} else {
		fmt.Println("Error   :", resp.Status)
		fmt.Println("Body :", string(body))
	}
}

func getXfundPrice() *go_ooo_types.CoinGeckoResponse {
	client := &http.Client{}
	cgUrl := "https://api.coingecko.com/api/v3/simple/price?ids=xfund&vs_currencies=eth%2Cusd"
	req, err := http.NewRequest("GET", cgUrl, nil)
	if err != nil {
		fmt.Println(err)
		return &go_ooo_types.CoinGeckoResponse{}
	}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return &go_ooo_types.CoinGeckoResponse{}
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		fmt.Println(readErr)
		return &go_ooo_types.CoinGeckoResponse{}
	}

	cgResp := &go_ooo_types.CoinGeckoResponse{}

	jsonErr := json.Unmarshal(body, cgResp)
	if jsonErr != nil {
		fmt.Println(jsonErr)
	}
	return cgResp
}
