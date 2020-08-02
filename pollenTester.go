package main

import (
	"fmt"
	"net/http"
	"time"
	"log"
	"sync"
	"strings"
	"flag"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet"
	walletaddr "github.com/iotaledger/goshimmer/client/wallet/packages/address"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	//"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	valueAddr "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	//valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	//"github.com/mr-tron/base58"
)

func main() {

	nbrNodes := flag.Int("nbrNodes",50,"Number of nodes you want to test against")
	instances := flag.Int("inst",5,"Number of tests against each node")
	myNode := flag.String("node","http://161.35.154.87:8080","Valid node for initial transactions")
	collision := flag.Bool("collision",false,"Test collisions")
	txns := flag.Int("txns",50,"Number of transactions to run")

	flag.Parse()

	if *collision == true {
		*instances = 1
	}
	var wg sync.WaitGroup

	//nodeList := []string{"http://161.35.154.87:8080","https://node.naerd.tech:443","http://94.130.96.130:8080","http://37.97.145.239:8080","http://159.69.144.242:8080","http://88.99.190.9:8080"}

	wc := wallet.NewWebConnector(*myNode, http.Client{Timeout: 120 * time.Second})
	status, err:=wc.ServerStatus()

	if err != nil || status.Synced == false {
		log.Fatalf("Error getting status")
	} else {
		fmt.Println(status)
	}

	nodeList := getSomeNodes(*myNode,*nbrNodes)
	fmt.Println(nodeList)
	
	faucetWallet := walletseed.NewSeed()
	err = wc.RequestFaucetFunds(faucetWallet.Address(0))
	if err != nil {
		log.Fatal("Unable to get funds from faucet")
	}	

	fromWallet := walletseed.NewSeed()
        toWallet := walletseed.NewSeed()

	var faucetIndex uint64

	for i := 0; i< (len(nodeList) * *instances);i=i+len(nodeList) {

		var addressList []walletaddr.Address
		if *collision == true {
			addressList = append(addressList,fromWallet.Address(uint64(i)))
		} else {
			for x := 0; x<len(nodeList); x++ {
				addressList = append(addressList,fromWallet.Address(uint64(i+x)))
			}
		}
		fmt.Println(addressList)
		found := getFundsFromFaucetWallet(wc,faucetWallet,faucetIndex,addressList)
		if found == true {
			faucetIndex = faucetIndex + 1
			for node := 0; node < len(nodeList); node++  {
				wg.Add(1)
				if *collision == true {
					go testPollen(nodeList[node],fromWallet,toWallet,uint64(i),*txns,&wg)
				} else {
					go testPollen(nodeList[node],fromWallet,toWallet,uint64(i+node),*txns,&wg)
				}
			}
		} else {
			time.Sleep(5 * time.Second)
		}
	}

	wg.Wait()

}

func getSomeNodes(url string,numberOfNodes int) []string {


	var nodes []string
	count := 0
	api := client.NewGoShimmerAPI(url,http.Client{Timeout:10 * time.Second})

	data,_ := api.GetNeighbors(true) 

	for _,node := range data.KnownPeers {
		for _,service := range node.Services {
			if service.ID == "peering" {
				api := client.NewGoShimmerAPI("http://" + strings.Split(service.Address,":")[0] + ":8080",http.Client{Timeout:60 * time.Second})
				info,_ := api.Info()
				if info != nil {
                                 	if info.Synced == true {
				  		nodes = append(nodes,"http://" + strings.Split(service.Address,":")[0] + ":8080")
						count++
						if count == numberOfNodes {
							return nodes
						}
			         	}
		        	}
	        	}
		}

        }
	return nodes
}


func createTransaction(txn transaction.OutputID, amt int64, to walletaddr.Address) *transaction.Transaction {

	//txOutputID := transaction.NewOutputID(from.Address, from.TransactionID)
        tx := transaction.New(
        	transaction.NewInputs(txn),
		transaction.NewOutputs(map[valueAddr.Address][]*balance.Balance{
		to.Address : {
			{Value: amt, Color: balance.ColorIOTA},
		},
	}))
	return tx
}

func getFundsFromFaucetWallet(wc *wallet.WebConnector, faucetWallet *walletseed.Seed, faucetIndex uint64, toAddressList []walletaddr.Address) bool {

	faucetAddr := faucetWallet.Address(faucetIndex)
	faucetRemainder := faucetWallet.Address(faucetIndex+ 1)

	for i :=0 ; i < 10; i++ {
		unspent,_ := wc.UnspentOutputs(faucetAddr)
		for _,address := range unspent {
			for _,output := range address {
				if output.InclusionState.Confirmed == true {
					txOutputID := transaction.NewOutputID(output.Address, output.TransactionID)
					txn := transaction.New(
						transaction.NewInputs(txOutputID),
						transaction.NewOutputs(map[valueAddr.Address][]*balance.Balance{}),)
					for _,address := range toAddressList {
						txn.Outputs().Add(address.Address, []*balance.Balance{balance.New(balance.ColorIOTA, 2)})
					}
					txn.Outputs().Add(faucetRemainder.Address, []*balance.Balance{balance.New(balance.ColorIOTA, int64(output.Balances[balance.ColorIOTA]) - int64(len(toAddressList) * 2))})
        				txn = txn.Sign(signaturescheme.ED25519(*faucetWallet.KeyPair(faucetIndex)))
					err := wc.SendTransaction(txn)
					if err != nil {
						log.Fatalf("Invalid txn from faucet Wallet, error - %v",err)
					}
					return true
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
	return false
}

func testPollen(url string, fromWallet *walletseed.Seed, toWallet *walletseed.Seed, index uint64, txns int,wg *sync.WaitGroup) {


	defer wg.Done()
	
	var txnCount, failedTxnCount, unconfirmedCount int
	//wc := wallet.NewWebConnector(url, http.Client{Timeout: 90 * time.Second})
	api := client.NewGoShimmerAPI(url, http.Client{Timeout: 120 * time.Second})

	fromAddr := fromWallet.Address(index)
	toAddr := toWallet.Address(index)

	dataOutput := url + "," + fromAddr.String() + ",," 

	start := time.Now()
	for txnCount < txns {
		utxos,_ := api.GetUnspentOutputs([]string{fromAddr.String(),toAddr.String()})
		for _, unspent := range utxos.UnspentOutputs {
		   	for _, output := range unspent.OutputIDs {
				if output.InclusionState.Confirmed == true {
					fmt.Printf("%v,%s\n",dataOutput,time.Since(start).String())
					start = time.Now()
					dataOutput = ""

					spentTxnID,_ :=  transaction.OutputIDFromBase58(output.ID)
					//inTxnID,err := transaction.IDFromBase58(output.ID)
					var txn *transaction.Transaction
					if unspent.Address == fromAddr.Address.String() {
						txnOutputID := transaction.NewOutputID(fromAddr.Address, spentTxnID.TransactionID())
						txn = createTransaction(txnOutputID,output.Balances[0].Value,toAddr)
        					txn = txn.Sign(signaturescheme.ED25519(*fromWallet.KeyPair(index)))
					} else {
						txnOutputID := transaction.NewOutputID(toAddr.Address, spentTxnID.TransactionID())
						txn = createTransaction(txnOutputID,output.Balances[0].Value,fromAddr)
        					txn = txn.Sign(signaturescheme.ED25519(*toWallet.KeyPair(index)))
					}
					txnID,err := api.SendTransaction(txn.Bytes())
					if err != nil { 
        					fmt.Printf("Error from send transaction = %v\n",err)
						_,err1 := api.SendTransaction(txn.Bytes())
						if err1 != nil {
							failedTxnCount++
        						fmt.Printf("Error from send transaction = %v\n",err1)
						}
					}
					txnCount++
					dataOutput = url + "," + txnID + "," + time.Since(start).String()
					start = time.Now()
				} else {
					// Unconfirmed so send a data txn.
					unconfirmedCount++
					if unconfirmedCount > 100  {
						fmt.Printf("Unconfirmed txn for more that 100 seconds, please check address %v",unspent.Address)
						fmt.Printf("%v : Transactions sent = %v, of whcih %v failed\n",url,txnCount,failedTxnCount)
						return
					}

					//dataStartTime := time.Now()
					//msgOut := "Txn " + output.ID +  "is unconfirmed"
					//msgID, err := api.Data([]byte(msgOut))
					//if err != nil {
					//	fmt.Println("Unable to send data message")
					//}
					//fmt.Printf("Data Message sent with id = %v, it took %s\n",msgID,time.Since(dataStartTime))
				}
			}
		   
		}
		time.Sleep(1 * time.Second)	
	}
	fmt.Printf("%v : Transactions sent = %v, of whcih %v failed\n",url,txnCount,failedTxnCount)
}
