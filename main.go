package main

import (
	"GoSolanaTask3/sol"
	"context"
	"fmt"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
	"os"
	"time"
)

func main() {
	cluster := rpc.DevNet
	rpcClient := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(cluster.RPC, rate.Every(time.Second), 5))
	defer rpcClient.Close()

	account := "6LK6mYCQ6axrd8QDG45DwTbZEfPptUybC2RHjvCfgbVZ"
	sol.GetAccountBalance(rpcClient, account)
	sol.GetAccountInfo(rpcClient, account)
	sol.GetBlockInfo(rpcClient, 0)

	godotenv.Load(".env")
	payerKey := os.Getenv("payerKey")
	toAccount := "4cvMN6ar1kJGhXYNPkz8mBqo447Fsx8j4NMURTvbCqnU"
	sol.SendTransfer(rpcClient, payerKey, toAccount, uint64(100000))

	wsClient, err := ws.Connect(context.TODO(), cluster.WS)
	if err != nil {
		fmt.Println("Failed to connect to ws: ", err)
	}
	defer wsClient.Close()
	//交易
	sol.SendTransferAndSubscript(rpcClient, wsClient, payerKey, toAccount, uint64(100000))
}
