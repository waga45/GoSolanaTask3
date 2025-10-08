package sol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"testing"
)

func getMethodPrefix(methodName string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte("global:" + methodName))
	return hasher.Sum(nil)[:8]
}

// 测试部署
func TestDeployProgram(t *testing.T) {
	SunTokenSelector, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := os.Getwd()
	osFile := filepath.Join(dir, "..", "bin/SunToken.so")
	//加载直接码
	programData, err := SunTokenSelector.LoadProgram(osFile)
	if err != nil {
		t.Fatal(err)
	}
	godotenv.Load("../.env")
	payerKey := os.Getenv("payerKey")
	privateKey, err := solana.PrivateKeyFromBase58(payerKey)
	if err != nil {
		t.Fatal(err)
	}
	programId, err := SunTokenSelector.DeployedProgram(programData, privateKey)
	if err != nil {
		fmt.Println("合约部署失败", err)
		return
	}
	//programId: E6YsmrcEZWSebaW2MG25pHJXiaxvZQx5QMvwpW9WqACm
	//wc,部署一次好贵啊 1.4sol
	fmt.Println("programId:", programId)
}

// 测试合约程序调用
func TestInvokeContractSol(t *testing.T) {
	SunTokenSelector, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	godotenv.Load("../.env")
	payerKey := os.Getenv("payerKey")
	privateKey, err := solana.PrivateKeyFromBase58(payerKey)
	if err != nil {
		t.Fatal(err)
	}
	programId, _ := solana.PublicKeyFromBase58("2no9gvgnqGsxb9Vdajk93HPRxgux7uy8FxwqpXoj5kUd")
	amount := uint64(100000)
	//调用合约 transfer_sol_with_cpi 方法
	//获取方法前缀
	pref := getMethodPrefix("transfer_sol_with_cpi")
	buf := new(bytes.Buffer)
	buf.Write(pref)
	binary.Write(buf, binary.LittleEndian, amount)
	//指令数据
	instructionData := buf.Bytes()
	recipient, _ := solana.PublicKeyFromBase58("4cvMN6ar1kJGhXYNPkz8mBqo447Fsx8j4NMURTvbCqnU")
	signature, err := SunTokenSelector.CallProgram(programId, instructionData, privateKey, recipient)
	if err != nil {
		fmt.Println("合约方法调用失败，", err)
		return
	}
	fmt.Println("signature:", signature)
}
