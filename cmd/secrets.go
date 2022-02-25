package cmd

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

type secrets map[string]string

func newSecrets(secretList []string) secrets {
	s := make(map[string]string)
	for _, secretPair := range secretList {
		secretPairParts := strings.SplitN(secretPair, "=", 2)
		secretPairParts[0] = strings.ToUpper(secretPairParts[0])
		if strings.ToUpper(s[secretPairParts[0]]) == secretPairParts[0] {
			log.Errorf("Secret %s is already defined (secrets are case insensitive)", secretPairParts[0])
		}
		if len(secretPairParts) == 2 {
			s[secretPairParts[0]] = secretPairParts[1]
		} else if env, ok := os.LookupEnv(secretPairParts[0]); ok && env != "" {
			s[secretPairParts[0]] = env
		} else {
			fmt.Printf("Provide value for '%s': ", secretPairParts[0])
			val, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				log.Errorf("failed to read input: %v", err)
				os.Exit(1)
			}
			s[secretPairParts[0]] = string(val)
		}
	}
	return s
}

func (s secrets) AsMap() map[string]string {
	return s
}
