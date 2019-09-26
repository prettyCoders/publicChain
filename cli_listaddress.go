package main

import (
	"fmt"
	"os"
)

func (cli *CLI) listAddresses() {
	wallets, err := NewWallets()
	if err != nil {
		fmt.Println("There are not any wallet,Please create one first.")
		os.Exit(1)
	}
	addresses := wallets.GetAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}
