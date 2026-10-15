package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmptyPassword = errors.New("password must be provided via argument or ADMIN_PASSWORD environment variable")
)

func getPassword(args []string, getenv func(string) string) (string, error) {
	if len(args) > 1 && args[1] != "" {
		return args[1], nil
	}
	if envPass := getenv("ADMIN_PASSWORD"); envPass != "" {
		return envPass, nil
	}
	return "", ErrEmptyPassword
}

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("generating hash: %w", err)
	}
	return string(hash), nil
}

func main() {
	pass, err := getPassword(os.Args, os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	hash, err := hashPassword(pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
