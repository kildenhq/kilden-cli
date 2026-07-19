package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/kildenhq/kilden-cli/internal/config"
)

// mustClient loads stored credentials and returns a ready API client, or a
// friendly error telling the user to sign in.
func mustClient() (*api.Client, *config.Credentials, error) {
	creds, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	return api.New(creds), creds, nil
}

// resolveHost picks the panel host: --host flag, then $KILDEN_HOST, then the
// hosted default. The trailing slash is trimmed so URL joins stay clean.
func resolveHost(flag string) string {
	host := flag
	if host == "" {
		host = os.Getenv("KILDEN_HOST")
	}
	if host == "" {
		host = config.DefaultHost
	}
	return strings.TrimRight(host, "/")
}

// resolveClientID allows self-hosters who re-seeded the CLI client to override
// the baked-in public client id.
func resolveClientID() string {
	if id := os.Getenv("KILDEN_CLIENT_ID"); id != "" {
		return id
	}
	return config.DefaultClientID
}

// currentProjectID resolves the project a data/keys command should act on:
// the --project flag if given, otherwise the account's active project.
func currentProjectID(ctx context.Context, client *api.Client, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	cur, err := client.Context(ctx)
	if err != nil {
		return "", err
	}
	if cur.Project == nil {
		return "", fmt.Errorf("no active project — run `kd project use` or pass --project")
	}
	return cur.Project.ID, nil
}

// selectFrom prints a numbered menu and returns the chosen index. It is only
// reached when stdin is interactive; scripts pass explicit ids instead.
func selectFrom(label string, options []string) (int, error) {
	fmt.Println(label)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Print("Choose a number: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(options) {
		return 0, fmt.Errorf("invalid choice")
	}
	return n - 1, nil
}

// prompt reads a single line for the given question.
func prompt(question string) (string, error) {
	fmt.Print(question)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// openBrowser best-effort opens a URL; failure is fine (the user can click the
// link the CLI printed).
func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, append(args, url)...).Start()
}

// newTab returns a tab-aligned writer over stdout.
func newTab() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

// bold wraps text in ANSI bold unless NO_COLOR is set.
func bold(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}
