package guard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// StdioPrompter reads setup prompts from stdin.
type StdioPrompter struct {
	In  io.Reader
	Out io.Writer
}

func (p StdioPrompter) out() io.Writer {
	if p.Out != nil {
		return p.Out
	}
	return os.Stderr
}

func (p StdioPrompter) Username() (string, error) {
	fmt.Fprint(p.out(), "Steam username: ")
	s, err := bufio.NewReader(p.in()).ReadString('\n')
	return strings.TrimSpace(s), err
}

func (p StdioPrompter) Password() (string, error) {
	fmt.Fprint(p.out(), "Steam password: ")
	if f, ok := p.in().(*os.File); ok {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(p.out())
		return string(b), err
	}
	s, err := bufio.NewReader(p.in()).ReadString('\n')
	return strings.TrimSpace(s), err
}

func (p StdioPrompter) GuardCode(kind string) (string, error) {
	fmt.Fprintf(p.out(), "Steam Guard %s code: ", kind)
	s, err := bufio.NewReader(p.in()).ReadString('\n')
	return strings.TrimSpace(s), err
}

func (p StdioPrompter) ConfirmRevocation(code string) error {
	fmt.Fprintln(p.out(), "Write down this Steam revocation code. Losing it and the maFile can lock the account.")
	fmt.Fprintf(p.out(), "Revocation code: %s\n", code)
	fmt.Fprint(p.out(), "Type YES to continue: ")
	s, err := bufio.NewReader(p.in()).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(s) != "YES" {
		return fmt.Errorf("setup aborted: revocation code was not confirmed")
	}
	return nil
}

func (p StdioPrompter) SMSCode() (string, error) {
	fmt.Fprint(p.out(), "SMS/email activation code: ")
	s, err := bufio.NewReader(p.in()).ReadString('\n')
	return strings.TrimSpace(s), err
}

func (p StdioPrompter) in() io.Reader {
	if p.In != nil {
		return p.In
	}
	return os.Stdin
}
