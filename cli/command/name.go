package command

import (
	"fmt"
	"math/rand"
)

func GenerateRandomName() string {
	letters := "abcdefghijklmnopqrstuvwxyz"
	digits := "123456789"

	name := fmt.Sprintf("%c%c%c%c",
		letters[rand.Intn(len(letters))], // Random letter
		digits[rand.Intn(len(digits))],   // Random digit
		letters[rand.Intn(len(letters))], // Random letter
		digits[rand.Intn(len(digits))])   // Random digit

	hyphenPosition := rand.Intn(len(name)-1) + 1
	nameWithHyphen := name[:hyphenPosition] + "-" + name[hyphenPosition:]

	return nameWithHyphen
}
