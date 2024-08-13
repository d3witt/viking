package command

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

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
