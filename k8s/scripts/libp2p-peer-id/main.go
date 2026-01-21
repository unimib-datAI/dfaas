// libp2p-peer-id prints the Peer ID derived from a given ed25519 private key.
//
// The primary purpose of this tool is to obtain the Peer ID so that other
// agents can connect to the agent associated with the given private key.
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintf(os.Stderr, "Usage: %s <key.pem>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Print the Peer ID for the ed25519 private key in the given PEM file.\n")
		os.Exit(0)
	}
	pemPath := os.Args[1]

	// Read the PEM file.
	pemData, err := os.ReadFile(pemPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read PEM file: %v\n", err)
		os.Exit(1)
	}

	// Decode PEM block.
	block, _ := pem.Decode(pemData)
	if block == nil {
		fmt.Fprintln(os.Stderr, "Failed to decode PEM block")
		os.Exit(1)
	}

	// Parse PKCS#8 private key.
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse PKCS#8 private key: %v\n", err)
		os.Exit(1)
	}

	// Convert to ed25519 private key.
	ed25519PrivKey, ok := parsedKey.(ed25519.PrivateKey)
	if !ok {
		fmt.Fprintln(os.Stderr, "Given key is not an ed25519 private key.")
		os.Exit(1)
	}

	// Convert to libp2p private key.
	libp2pPrivKey, _, err := crypto.KeyPairFromStdKey(&ed25519PrivKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create libp2p private key: %v\n", err)
		os.Exit(1)
	}

	// Get and print the Peer ID.
	peerID, err := peer.IDFromPrivateKey(libp2pPrivKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get Peer ID: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Peer ID: %s\n", peerID.String())
}
