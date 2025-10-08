package sol

import (
	"context"
	"errors"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/shopspring/decimal"
	"time"
)

func lamports2Sol(value uint64) string {
	sol := float64(value) / 1e9
	lamportDecimal := decimal.NewFromFloat(sol)
	lamportDecimal.Truncate(6)
	return lamportDecimal.Truncate(6).String()
}

// 查询余额
func GetAccountBalance(rpcClient *rpc.Client, accountBase58 string) (res string, err error) {
	if rpcClient == nil {
		return res, errors.New("rpc client is nil")
	}
	accountPublicKey := solana.MustPublicKeyFromBase58(accountBase58)
	//publicKey 账户地址  commitment：状态
	GetBalanceResult, err := rpcClient.GetBalance(context.TODO(), accountPublicKey, rpc.CommitmentFinalized)
	if err != nil {
		return res, err
	}
	fmt.Println("余额：", lamports2Sol(GetBalanceResult.Value))
	return lamports2Sol(GetBalanceResult.Value), nil
}

// 查询账户信息
func GetAccountInfo(rpcClient *rpc.Client, accountBase58 string) {
	accountPublicKey := solana.MustPublicKeyFromBase58(accountBase58)
	GetAccountInfoResult, err := rpcClient.GetAccountInfo(context.TODO(), accountPublicKey)
	if err != nil {
		fmt.Println(err)
	}
	//包括账户数据，Lamports（余额），Owner（账户归属），Data（程序数据）,Executable(是否为可执行程序)，RentEpoch（下一个周期租金-已废弃）,Space（占用空间大小）
	fmt.Println("Lamports:", GetAccountInfoResult.Value.Lamports)
	fmt.Println("sol:", lamports2Sol(GetAccountInfoResult.Value.Lamports))
	fmt.Println("owner:", GetAccountInfoResult.Value.Owner.String())
	fmt.Println("space:", GetAccountInfoResult.Value.Space)
}

// 获取区块信息
func GetBlockInfo(rpcClient *rpc.Client, blockNum uint64) {
	if blockNum <= 0 {
		//拿最近一个确认的区块高度
		GetLatestBlockhashResult, err := rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
		if err != nil {
			panic(err)
		}
		blockNum = GetLatestBlockhashResult.Value.LastValidBlockHeight
	}
	version := uint64(0)
	opts := &rpc.GetBlockOpts{
		Encoding:                       solana.EncodingBase64,
		MaxSupportedTransactionVersion: &version,
	}
	blockResult, err := rpcClient.GetBlockWithOpts(context.TODO(), blockNum, opts)
	if err != nil {
		panic(err)
	}
	//主要包括：blockhash（区块hash），previousBlockhash（上一个区块hash），parentSlot（父存储曹），transactions（交易列表），signatures（交易签名），blockTime（区块确认时间，blockHeight（区块高度））
	fmt.Println(blockResult)
}

// 发起交易
// 步骤：
// Step1: 加载转出账户私钥，转入账户地址
// Step2: 定义转账金额 以lamports为单位 sol=10e9
// Step3: 获取链上最后一个区块的hash
// Step4：组织交易结构 Instruction
// Step5: 创建交易，交易签名
// Step6: 发起常规交易
func SendTransfer(rpcClient *rpc.Client, payerPrivateKey string, toAccount string, amount uint64) (*solana.Signature, error) {
	privatekey, err := solana.PrivateKeyFromBase58(payerPrivateKey)
	if err != nil {
		return nil, err
	}
	publicKey := privatekey.PublicKey()
	if amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}
	toAccountPublicKey := solana.MustPublicKeyFromBase58(toAccount)
	recentBlockHash, err := rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	transferItx := system.NewTransferInstruction(
		amount,    //金额
		publicKey, //转出地址
		toAccountPublicKey).Build()
	tx, err := solana.NewTransaction([]solana.Instruction{transferItx}, recentBlockHash.Value.Blockhash, solana.TransactionPayer(publicKey))
	if err != nil {
		fmt.Println("构建交易失败，", err)
		return nil, err
	}
	//签名
	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key == publicKey {
			return &privatekey
		}
		return nil
	})
	//发起交易
	Signature, err := rpcClient.SendTransaction(context.TODO(), tx)
	if err != nil {
		fmt.Println("交易发起失败,", err)
		return nil, err
	}
	fmt.Println("交易发送成功,签名：", Signature)
	for i := 0; i < 20; i++ {
		GetTransStatus(rpcClient, Signature)
		time.Sleep(1 * time.Second)
	}
	return &Signature, nil
}

// 查询交易结果
func GetTransStatus(rpcClient *rpc.Client, signature solana.Signature) int64 {
	statusesResult, _ := rpcClient.GetSignatureStatuses(context.TODO(), true, signature)
	if statusesResult.Value != nil && len(statusesResult.Value) > 0 {
		switch statusesResult.Value[0].ConfirmationStatus {
		case rpc.ConfirmationStatusFinalized:
			fmt.Println("交易已最终确认")
			return 1
		case rpc.ConfirmationStatusConfirmed:
			return 2
		case rpc.ConfirmationStatusProcessed:
			return 0
		default:
			return 0
		}
	}
	return -1
}

// 发起交易 通过websocket监听状态，流程与常规交易一致
func SendTransferAndSubscript(rpcClient *rpc.Client, wsClient *ws.Client, payerPrivateKey string, toAccount string, amount uint64) (*solana.Signature, error) {
	PrivateKey := solana.MustPrivateKeyFromBase58(payerPrivateKey)
	fromAccountPublicKey := PrivateKey.PublicKey()

	toPublickey := solana.MustPublicKeyFromBase58(toAccount)

	txi := system.NewTransferInstruction(amount, fromAccountPublicKey, toPublickey).Build()
	latestBlockhashResult, err := rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		panic(err)
	}
	//构建交易
	unsignedTx, err := solana.NewTransaction([]solana.Instruction{txi}, latestBlockhashResult.Value.Blockhash, solana.TransactionPayer(fromAccountPublicKey))
	if err != nil {
		return nil, err
	}
	unsignedTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(fromAccountPublicKey) {
			return &PrivateKey
		}
		return nil
	})
	Signature, err := sendandconfirmtransaction.SendAndConfirmTransaction(context.TODO(), rpcClient, wsClient, unsignedTx)
	if err != nil {
		fmt.Println("交易发起失败,", err)
		return nil, err
	}
	fmt.Println("交易签名：", Signature)
	//订阅交易状态
	Subscribe, _ := wsClient.SignatureSubscribe(Signature, rpc.CommitmentFinalized)
	defer Subscribe.Unsubscribe()
	for {
		select {
		case <-Subscribe.Response():
			fmt.Println("交易已确认")
			break
		case <-time.After(time.Second * 30):
			fmt.Println("交易确认超时")
			break
		}
	}
	return &Signature, nil
}

// 查询交易详情
func GetTransactionInfo(rpcClient *rpc.Client, signature string) {
	version := uint64(0)
	GetTransactionResult, err := rpcClient.GetTransaction(context.TODO(), solana.MustSignatureFromBase58(signature), &rpc.GetTransactionOpts{
		Encoding:                       solana.EncodingBase64,
		MaxSupportedTransactionVersion: &version,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("区块高度：", GetTransactionResult.Slot)
	fmt.Println("区块时间：", GetTransactionResult.BlockTime.String())
	TransactionMeta := GetTransactionResult.Meta

	fmt.Println("手续费：", TransactionMeta.Fee)
	fmt.Println("交易前余额：", TransactionMeta.PreBalances[0])
	fmt.Println("交易后余额：", TransactionMeta.PostBalances[0])
	fmt.Println("交易日志：", TransactionMeta.LogMessages)
	TransactionResultEnvelope := GetTransactionResult.Transaction

	Transaction, err := TransactionResultEnvelope.GetTransaction()
	fmt.Println("最近一个区块hash：", Transaction.Message.RecentBlockhash.String())
	MessageHeader := Transaction.Message.Header
	fmt.Println("MessageHeader:", MessageHeader)
	fmt.Println("Transaction->IsVersioned:", Transaction.Message.IsVersioned())
	fmt.Println("交易账户：", Transaction.Message.AccountKeys)
	tblKeys := Transaction.Message.GetAddressTableLookups().GetTableIDs()
	fmt.Println("Transaction->tblKeys:", tblKeys)
	for i, instruction := range Transaction.Message.Instructions {
		fmt.Printf("指令 %d: 程序ID索引=%d, 数据长度=%d\n",
			i, instruction.ProgramIDIndex, len(instruction.Data))
	}
}
