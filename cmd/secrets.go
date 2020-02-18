package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/howeyc/gopass"
)

type secrets map[string]string

func newSecrets(secretList []string) secrets {
	s := make(map[string]string)
	for _, secretPair := range secretList {
		secretPairParts := strings.Split(secretPair, "=")
		if len(secretPairParts) == 2 {
			s[secretPairParts[0]] = secretPairParts[1]
		} else if env, ok := os.LookupEnv(secretPairParts[0]); ok && env != "" {
			s[secretPairParts[0]] = env
		} else {
			fmt.Printf("Provide value for '%s': ", secretPairParts[0])
			val, err := gopass.GetPasswdMasked()
			if err != nil {
				log.Fatal("abort")
			}
			s[secretPairParts[0]] = string(val)
		}
	}
	return s
}

func (s secrets) AsMap() map[string]string {
	return s
}
