// Command prpoller is a standalone service that monitors GitHub PR comments
// and triggers terraform apply when new comments are detected.
//
// Usage:
//
//	prpoller -pr owner/repo#123 -tf-dir /path/to/terraform
//	prpoller -pr https://github.com/owner/repo/pull/123 -tf-dir /path/to/terraform -tf-vars vars.tfvars
//
// Environment:
//
//	GITHUB_TOKEN - Required GitHub token with repo read access
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agent-dispatch/prpoller"
)

type arrayFlags []string

func (a *arrayFlags) String() string {
	return fmt.Sprintf("%v", *a)
}

func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

func main() {
	var prs arrayFlags
	var tfDir string
	var tfVars string
	var interval time.Duration
	var verbose bool

	flag.Var(&prs, "pr", "PR to monitor (can specify multiple times). Format: owner/repo#123 or URL")
	flag.StringVar(&tfDir, "tf-dir", "", "Terraform directory to apply on changes")
	flag.StringVar(&tfVars, "tf-vars", "", "Terraform vars file to use")
	flag.DurationVar(&interval, "interval", 30*time.Second, "Polling interval")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	if len(prs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: at least one -pr flag is required")
		flag.Usage()
		os.Exit(1)
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: GITHUB_TOKEN environment variable is required")
		os.Exit(1)
	}

	// Create poller
	p := prpoller.NewPoller(prpoller.Config{
		Interval: interval,
		Token:    token,
		OnChange: func(event prpoller.ChangeEvent) {
			fmt.Printf("[%s] New comments on %s:\n",
				event.Timestamp.Format(time.RFC3339),
				prpoller.FormatPRShort(event.PR.Owner, event.PR.Repo, event.PR.Number))
			for _, c := range event.NewComments {
				fmt.Printf("  - @%s: %s\n", c.Author, truncate(c.Body, 80))
			}
		},
	})

	// Register PRs
	for _, prStr := range prs {
		owner, repo, number, err := prpoller.ParsePRURL(prStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing PR %q: %v\n", prStr, err)
			os.Exit(1)
		}

		reg := prpoller.PRRegistration{
			Owner:  owner,
			Repo:   repo,
			Number: number,
		}

		// If terraform dir specified, set up auto-apply
		if tfDir != "" {
			reg.TerraformAction = &prpoller.TerraformAction{
				WorkDir:     tfDir,
				TFVarsFile:  tfVars,
				Description: fmt.Sprintf("revision triggered by comment on %s", prStr),
			}
		}

		p.Register(reg)
		fmt.Printf("Registered: %s\n", prpoller.FormatPRShort(owner, repo, number))
	}

	// Start polling
	fmt.Printf("Starting poller with %v interval...\n", interval)
	p.Start()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	p.Stop()
	fmt.Println("Done.")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
