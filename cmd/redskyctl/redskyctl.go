package main

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"os"

	"github.com/gramLabs/redsky/pkg/redskyctl/cmd"
)

func main() {
	// Seed the pseudo random number generator using the cryptographic random number generator
	// https://stackoverflow.com/a/54491783
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))

	// Run the command
	command := cmd.NewDefaultRedskyctlCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
