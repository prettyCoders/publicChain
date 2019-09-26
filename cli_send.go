package main

import (
	"fmt"
	"log"
)

func (cli *CLI) send(from, to string, amount int, mineNow bool) {
	if !ValidateAddress(from) {
		log.Panic("ERROR: Sender Address is not valid")
	}
	if !ValidateAddress(to) {
		log.Panic("ERROR: Recipient Address is not valid")
	}

	bc := NewBlockchain()
	UTXOSet := UTXOSet{bc}
	defer bc.db.Close()

	wallets, err := NewWallets()
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)

	tx := NewUTXOTransaction(&wallet, to, amount, &UTXOSet)

	if mineNow {
		cbTx := NewCoinbaseTX(from, "")
		txs := []*Transaction{cbTx, tx}

		newBlock := bc.MineBlock(txs)
		UTXOSet.Update(newBlock)
	} else {
		//sendTx(knownNodes[0], tx)
	}

	fmt.Println("Success!")
}
