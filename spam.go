package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

// ...

type DEX struct {
	Name          string
	RouterABI     string
	RouterAddress string
	ChainID       *big.Int
}

var supportedDEXes = []DEX{
	{
		Name:          "UniswapV2 - ETH",
		RouterABI:     "interfaces/uniswapV2Router.json",
		RouterAddress: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
		ChainID:       big.NewInt(1),
	},
	{
		Name:          "SushiSwap - Arbitrum",
		RouterABI:     "interfaces/sushiRouter.json",
		RouterAddress: "0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506",
		ChainID:       big.NewInt(42161),
	},
	{
		Name:          "UniswapV3 - ETH",
		RouterABI:     "interfaces/uniswapV3Router.json",
		RouterAddress: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		ChainID:       big.NewInt(1),
	},
	{
		Name:          "UniswapV3 - Arbitrum",
		RouterABI:     "interfaces/uniswapV3Router.json",
		RouterAddress: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		ChainID:       big.NewInt(42161),
	},
	{
		Name:          "Camelot - Arbitrum",
		RouterABI:     "interfaces/camelotRouter.json",
		RouterAddress: "0xc873fEcbd354f5A56E00E710B90EF4201db2448d",
		ChainID:       big.NewInt(42161),
	},
	{
		Name:          "Mute - ZkSync",
		RouterABI:     "interfaces/muteRouter.json",
		RouterAddress: "0x8B791913eB07C32779a16750e3868aA8495F5964",
		ChainID:       big.NewInt(324),
	},
	{
		Name:          "Glacier - Avax",
		RouterABI:     "interfaces/glacierRouter.json",
		RouterAddress: "0xC5B8Ce3C8C171d506DEb069a6136a351Ee1629DC",
		ChainID:       big.NewInt(43114),
	},
	{
		Name:          "PepeDex - Arbitrum",
		RouterABI:     "interfaces/uniswapV2Router.json",
		RouterAddress: "0x69057AA657526acE6F54369B0E12C585aCF42AbC",
		ChainID:       big.NewInt(42161),
	},
	{
		Name:          "Velodrome - Optimism",
		RouterABI:     "interfaces/veloRouter.json",
		RouterAddress: "0x9c12939390052919aF3155f41Bf4160Fd3666A6f",
		ChainID:       big.NewInt(10),
	},
	// Add more supported DEXes here
}

var amountOutMin *big.Int
var path []common.Address

var UniswapV2FactoryABI string
var UniswapV2PairABI string

// Cache your addresses
var targetAddress = common.HexToAddress(target)

var factoryABI abi.ABI
var factoryAddress common.Address
var WETHAddress common.Address

var yourPubKey string
var yourKey string
var target string
var targetLiquidityToken string
var RPC string
var routerAddress string
var chainId *big.Int
var amountIn int64
var gasLimit int64

var spamDelay time.Duration

func init() {
	// Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Initialize your variables
	yourPubKey = os.Getenv("YOUR_PUB_KEY")
	yourKey = os.Getenv("YOUR_PRIVATE_KEY")
	target = os.Getenv("TARGET_TOKEN")
	RPC = os.Getenv("RPC")

	amountInStr := os.Getenv("AMOUNT_IN")
	amountIn, err = strconv.ParseInt(amountInStr, 10, 64)
	if err != nil {
		log.Fatalf("Error converting AMOUNT_IN to int64: %v", err)
	}

	spamDelayStr := os.Getenv("SPAM_DELAY")
	spamDelayInt, err := strconv.Atoi(spamDelayStr)
	if err != nil {
		log.Fatalf("Error converting SPAM_DELAY to int: %v", err)
	}
	spamDelay = time.Duration(spamDelayInt) * time.Millisecond

	// Initialize amountOutMin
	amountOutMin = big.NewInt(0) // Set to zero to allow any limit

	gasLimitStr := os.Getenv("GAS_LIMIT")
	gasLimit, err = strconv.ParseInt(gasLimitStr, 10, 64)
	if err != nil {
		log.Fatalf("Error converting GAS_LIMIT to int64: %v", err)
	}

	// Cache your addresses
	targetAddress = common.HexToAddress(target)
	WETHAddress = common.HexToAddress(os.Getenv("WETH_ADDRESS"))

	// Initialize the path variable
	path = []common.Address{
		WETHAddress,   // WETH
		targetAddress, // Target token
	}
}

// OK
func main() {
	color.Set(color.FgMagenta)
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get the user's selected DEX
	selectedDEX := getUserSelectedDEX()

	// Set the router and factory addresses
	routerAddress = selectedDEX.RouterAddress

	chainId = selectedDEX.ChainID

	// Connect to Ethereum client
	client, err := ethclient.Dial(RPC)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	fmt.Println("Select method:")
	fmt.Println("Press 1 to Spam transactions")

	var userInput int
	_, err = fmt.Scanf("%d", &userInput)
	if err != nil {
		log.Fatalf("Error reading user input: %v", err)
	}

	color.Unset()

	if userInput == 1 {

		// Spam transactions without waiting for the liquidity addition event
		spamTransactions(client, selectedDEX)

	}
}

func swap(client *ethclient.Client, selectedDEX DEX, nonce uint64) {

	color.Set(color.FgGreen)
	// Load the private key
	privateKey, err := crypto.HexToECDSA(yourKey)
	if err != nil {
		log.Fatal(err)
	}

	/*// Get the public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}
	/*
		// Get the Ethereum address from the public key
		fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	*/
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Create an authorized transactor
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainId)
	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.Value = big.NewInt(amountIn) // in wei
	auth.GasLimit = uint64(gasLimit)  // in units
	auth.GasPrice = gasPrice

	// Load the contract ABI
	contractABI, err := loadABI(selectedDEX.RouterABI)
	if err != nil {
		fmt.Println(err)
	}

	// Parse the ABI to create a Go binding
	contractAbi, err := abi.JSON(bytes.NewReader([]byte(contractABI)))
	if err != nil {
		fmt.Println(err)
	}

	// Create a new instance of the contract using the parsed ABI
	address := common.HexToAddress(routerAddress)
	instance := bind.NewBoundContract(address, contractAbi, client, client, client)

	// Define the recipient address
	to := common.HexToAddress(yourPubKey)

	// Define the deadline
	deadline := big.NewInt(time.Now().Add(2 * time.Minute).Unix())

	if selectedDEX.Name == "UniswapV3 - ETH" || selectedDEX.Name == "UniswapV3 - Arbitrum" {
		// Uniswap V3 swap parameters
		params := struct {
			TokenIn           common.Address
			TokenOut          common.Address
			Fee               *big.Int
			Recipient         common.Address
			Deadline          *big.Int
			AmountIn          *big.Int
			AmountOutMinimum  *big.Int
			SqrtPriceLimitX96 *big.Int
		}{
			TokenIn:           WETHAddress,      // WETH
			TokenOut:          targetAddress,    // Target token
			Fee:               big.NewInt(3000), // Set the desired fee tier, e.g., 3000 for 0.3%
			Recipient:         to,
			Deadline:          deadline,
			AmountIn:          big.NewInt(amountIn),
			AmountOutMinimum:  big.NewInt(0), // Set to zero to allow any limit
			SqrtPriceLimitX96: big.NewInt(0), // Set to zero to allow any limit
		}

		// Call the "exactInputSingle" function on the Uniswap V3 router contract
		tx, err := instance.Transact(auth, "exactInputSingle", params)
		if err != nil {
			log.Fatalf("Failed to send swap transaction: %v", err)
		}
		fmt.Printf("[ALERT] Transaction sent, tx hash: %s\n", tx.Hash().Hex())

	} else if selectedDEX.Name == "Camelot - Arbitrum" {
		//Camelot Dex swap parameters

		referrer := common.HexToAddress("0x0000000000000000000000000000000000000000")

		tx, err := instance.Transact(auth, "swapExactETHForTokensSupportingFeeOnTransferTokens", amountOutMin, path, to, referrer, deadline)
		if err != nil {
			log.Fatalf("Failed to send swap transaction : %v", err)
		}
		fmt.Printf("[ALERT] Transaction sent, tx hash: %s\n", tx.Hash().Hex())

	} else if selectedDEX.Name == "Mute - ZkSync" {
		// Execute the swap transaction
		stablePool := false
		stablePoolSlice := []bool{stablePool, stablePool}
		tx, err := instance.Transact(auth, "swapExactETHForTokensSupportingFeeOnTransferTokens", amountOutMin, path, to, deadline, stablePoolSlice)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("[ALERT] Transaction sent, tx hash: %s\n", tx.Hash().Hex())

		// Execute the swap transaction
	} else if selectedDEX.Name == "Glacier - Avax" {
		type Route struct {
			From   common.Address
			To     common.Address
			Stable bool
		}

		type GlcrPathTuple struct {
			AmountOutMin *big.Int
			Routes       []Route
			To           common.Address
			Deadline     *big.Int
		}
		// Create the Route
		route := Route{
			From:   WETHAddress,
			To:     targetAddress,
			Stable: false,
		}

		routes := []Route{route}

		// Execute the swap transaction
		tx, err := instance.Transact(auth, "swapExactAVAXForTokens", amountOutMin, routes, to, deadline)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("[ALERT] Transaction sent, tx hash: %s\n", tx.Hash().Hex())

	} else if selectedDEX.Name == "Velodrome - Optimism" {
		// Define the Route for Velodrome
		type Route struct {
			From   common.Address
			To     common.Address
			Stable bool
		}

		// Define the route array. Modify this array according to your requirements.
		routes := []Route{
			{
				From:   WETHAddress,
				To:     targetAddress,
				Stable: false,
			},
			// additional routes could go here
		}

		// Define the params for the Velodrome swapExactETHForTokens function
		params := struct {
			AmountOutMin *big.Int
			Routes       []Route
			To           common.Address
			Deadline     *big.Int
		}{
			AmountOutMin: big.NewInt(0), // Set to zero to allow any limit
			Routes:       routes,
			To:           to,
			Deadline:     deadline,
		}

		// Call the "swapExactETHForTokens" function on the Velodrome router contract
		tx, err := instance.Transact(auth, "swapExactETHForTokens", params.AmountOutMin, params.Routes, params.To, params.Deadline)
		if err != nil {
			log.Fatalf("Failed to send swap transaction: %v", err)
		}
		fmt.Printf("[ALERT] Transaction sent, tx hash: %s\n", tx.Hash().Hex())

	} else {
		// Execute the swap transaction
		tx, err := instance.Transact(auth, "swapExactETHForTokens", amountOutMin, path, to, deadline)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("[ALERT] Transaction sent, tx hash: %s\n", tx.Hash().Hex())
	}
	color.Unset()
}

func loadABI(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func getUserSelectedDEX() DEX {
	fmt.Println("Select the DEX to snipe:")
	for i, dex := range supportedDEXes {
		fmt.Printf("%d. %s\n", i+1, dex.Name)
	}

	var selection int
	fmt.Scan(&selection)

	if selection < 1 || selection > len(supportedDEXes) {
		fmt.Println("Invalid selection, exiting.")
		os.Exit(1)
	}

	selectedDEX := supportedDEXes[selection-1]
	return selectedDEX
}

func spamTransactions(client *ethclient.Client, selectedDEX DEX) {
	// Load the private key
	privateKey, err := crypto.HexToECDSA(yourKey)
	if err != nil {
		log.Fatal(err)
	}

	// Get the public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	// Get the Ethereum address from the public key
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Get initial nonce
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	// Now start the loop
	for {
		swap(client, selectedDEX, nonce)

		time.Sleep(spamDelay)

		// increment the nonce after each transaction
		nonce++
	}
}
