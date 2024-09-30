package command

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"text/tabwriter"

	"github.com/d3witt/viking/config/appconf"
	"github.com/d3witt/viking/config/userconf"
	"github.com/d3witt/viking/streams"
)

type Cli struct {
	Config   userconf.Config
	Out, Err *streams.Out
	In       *streams.In
}

func (c *Cli) AppConfig() (appconf.Config, error) {
	return appconf.ParseConfig()
}

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

func PrintTable(output io.Writer, data [][]string) error {
	w := tabwriter.NewWriter(output, 0, 0, 3, ' ', tabwriter.TabIndent)

	for _, line := range data {
		// Formatting and printing each line to fit the tabulated format
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}

	return w.Flush()
}

func Prompt(in io.Reader, out io.Writer, prompt, configDefault string) (string, error) {
	if configDefault == "" {
		fmt.Fprintf(out, "%s: ", prompt)
	} else {
		fmt.Fprintf(out, "%s (%s): ", prompt, configDefault)
	}

	line, _, err := bufio.NewReader(in).ReadLine()
	if err != nil {
		return "", fmt.Errorf("Error while reading input: %w", err)
	}

	return strings.TrimSpace(string(line)), nil
}

// PromptForConfirmation requests and checks confirmation from the user.
// This will display the provided message followed by ' [y/N] '. If the user
// input 'y' or 'Y' it returns true otherwise false. If no message is provided,
// "Are you sure you want to proceed? [y/N] " will be used instead.
func PromptForConfirmation(in io.Reader, out io.Writer, message string) (bool, error) {
	if message == "" {
		message = "Are you sure you want to proceed?"
	}
	message += " [y/N] "

	answer, err := Prompt(in, out, message, "")
	if err != nil {
		return false, err
	}

	return strings.EqualFold(answer, "y"), nil
}
