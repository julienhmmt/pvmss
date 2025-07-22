package main

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	fmt.Print("Enter password to hash: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Error reading password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Error generating hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Bcrypt hash: %s\n", string(hash))
	fmt.Printf("For .env file: ADMIN_PASSWORD_HASH=\"%s\"\n", string(hash))
	fmt.Println("Remember to escape '$' characters with '$$' in .env file!")
}
