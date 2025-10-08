package sol

import (
	"context"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"os"
	"time"
)

type SunTokenSelector struct {
	rpcClient *rpc.Client
	wsClient  *ws.Client
	ctx       context.Context
	cancel    context.CancelFunc
}

type ChanResult struct {
	programId solana.PublicKey
	event     EventFun
	signature solana.Signature
}
type EventFun int

const (
	Deployed EventFun = 0
	CallFun  EventFun = 1
)

func NewInstance() (*SunTokenSelector, error) {
	endpoint := rpc.DevNet
	rpcClient := rpc.New(endpoint.RPC)
	//websocket
	wsClient, err := ws.Connect(context.TODO(), endpoint.WS)
	if err != nil {
		return nil, err
	}
	context, cancel := context.WithCancel(context.Background())
	return &SunTokenSelector{
		rpcClient: rpcClient,
		wsClient:  wsClient,
		ctx:       context,
		cancel:    cancel,
	}, nil
}

// 加载BPF编译好的程序
func (token *SunTokenSelector) LoadProgram(soFilePath string) ([]byte, error) {
	programData, err := os.ReadFile(soFilePath)
	if err != nil {
		return nil, err
	}
	return programData, nil
}

// 部署程序
func (token *SunTokenSelector) DeployedProgram(programData []byte, payer solana.PrivateKey) (signtrue solana.PublicKey, err error) {
	//创建keypair
	privateKey, _ := solana.NewRandomPrivateKey()
	fmt.Println("privateKey:", privateKey.String())
	programId := privateKey.PublicKey()
	fmt.Println("正在部署合约程序 id：", programId.String())
	//last block hash
	lastBlockHash, err := token.rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return programId, err
	}
	//估算费用
	guessLamports, err := token.rpcClient.GetMinimumBalanceForRentExemption(context.TODO(), uint64(len(programData)), rpc.CommitmentFinalized)
	if err != nil {
		return programId, err
	}
	//构建指令集
	instruction := system.NewCreateAccountInstruction(
		guessLamports,
		uint64(len(programData)),
		solana.BPFLoaderUpgradeableProgramID,
		payer.PublicKey(),
		programId).Build()
	//构建交易
	deployTransaction, err := solana.NewTransaction([]solana.Instruction{instruction}, lastBlockHash.Value.Blockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return programId, err
	}
	//签名
	deployTransaction.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			//付款者签名-用于支付部署费用
			return &payer
		}
		if key.Equals(privateKey.PublicKey()) {
			//程序账户签名
			return &privateKey
		}
		return nil
	})
	//发起交易
	signature, err := sendandconfirmtransaction.SendAndConfirmTransaction(context.TODO(), token.rpcClient, token.wsClient, deployTransaction)
	if err != nil {
		return programId, err
	}
	fmt.Println("部署交易签名：", signature)
	result := ChanResult{
		programId: programId,
		event:     Deployed,
		signature: signature,
	}
	//监听
	subscription, err := token.wsClient.SignatureSubscribe(result.signature, rpc.CommitmentFinalized)
	if err != nil {
		fmt.Println(result.signature.String(), " 前面监听状态失败")
		return
	}
	defer subscription.Unsubscribe()
	for {
		select {
		case <-subscription.Response():
			fmt.Println("部署已确认")
			break
		case <-time.After(30 * time.Second):
			fmt.Println("部署监听状态超时，可能失败")
			break
		}
	}
	return programId, nil
}

// 程序调用
func (token *SunTokenSelector) CallProgram(
	programId solana.PublicKey, //程序ID
	instructionData []byte, //指令
	payer solana.PrivateKey, //调用者
	recipient solana.PublicKey) (solana.Signature, error) {

	//last block hash
	recentBlockHash, err := token.rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return solana.Signature{}, err
	}
	//账户元数据
	accountMetas := []*solana.AccountMeta{
		{
			PublicKey:  payer.PublicKey(),
			IsWritable: true,
			IsSigner:   true,
		},
		{
			PublicKey:  recipient,
			IsWritable: true,
			IsSigner:   false,
		},
		{
			PublicKey:  solana.SystemProgramID,
			IsWritable: false,
			IsSigner:   false,
		},
	}

	//构建指令
	instruction := solana.NewInstruction(programId, accountMetas, instructionData)
	//创建交易
	tx, err := solana.NewTransaction([]solana.Instruction{instruction},
		recentBlockHash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		panic(err)
	}
	//签名
	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		return nil
	})
	//发起调用
	Signature, err := sendandconfirmtransaction.SendAndConfirmTransaction(context.TODO(), token.rpcClient, token.wsClient, tx)
	if err != nil {
		return solana.Signature{}, err
	}
	fmt.Println("合约调用成功:", Signature.String())
	//数据监听
	LogSubscription, err := token.wsClient.LogsSubscribe(ws.LogsSubscribeFilterAll, rpc.CommitmentFinalized)
	if err != nil {
		fmt.Println(err)
		return solana.Signature{}, err
	}
	defer LogSubscription.Unsubscribe()
	for {
		select {
		case log := <-LogSubscription.Response():
			fmt.Println("log:", log.Value.Logs)
			break
		case <-time.After(30 * time.Second):
			fmt.Println("部署监听状态超时，可能失败")
			break
		}
	}
	return Signature, nil
}

// 获取程序的账户
func (token *SunTokenSelector) GetProgramAccount(programId solana.PublicKey) (*rpc.GetAccountInfoResult, error) {
	GetAccountInfoResult, err := token.rpcClient.GetAccountInfo(context.TODO(), programId)
	return GetAccountInfoResult, err
}
