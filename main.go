package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	// Auto-load .env file if present
	loadEnv(".env")

	token := flag.String("token", os.Getenv("REDDIT_TOKEN"), "Reddit token_v2 cookie (or REDDIT_TOKEN env)")
	days := flag.Int("days", 14, "Days to keep (targets anything older)")
	dryRun := flag.Bool("dry-run", true, "Preview mode without deleting (default: true)")
	deleteFlag := flag.Bool("delete", false, "Execute deletion (disables dry-run)")
	yes := flag.Bool("yes", false, "Skip confirmation warning prompt")
	flag.Parse()

	// If -delete flag is passed, turn off dry-run
	if *deleteFlag {
		*dryRun = false
	}

	if *token == "" {
		fmt.Println("Error: Missing REDDIT_TOKEN.")
		fmt.Println("\nAdd your token_v2 cookie to a .env file:")
		fmt.Println("  REDDIT_TOKEN=your_token_v2_from_browser")
		fmt.Println("\nOr pass it via flag: -token <token_v2>")
		fmt.Println("See README.md for 3-step instructions on grabbing token_v2.")
		os.Exit(1)
	}

	client, err := NewRedditClient(*token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		os.Exit(1)
	}

	cutoff := time.Now().AddDate(0, 0, -*days)
	mode := "DRY-RUN (Preview Only)"
	if !*dryRun {
		mode = "DELETE MODE"
	}

	fmt.Printf("Logged in as /u/%s | Mode: %s\n", client.username, mode)
	fmt.Printf("Scanning for items older than %s (%d days)...\n\n", cutoff.Format("2006-01-02"), *days)

	var toDelete []RedditItem
	for _, target := range []string{"comments", "submitted"} {
		items, err := client.FetchAll(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching %s: %v\n", target, err)
			continue
		}

		for _, item := range items {
			if item.CreatedAt.Before(cutoff) {
				toDelete = append(toDelete, item)
			}
		}
	}

	if len(toDelete) == 0 {
		fmt.Println("No comments or posts found older than the cutoff. Nothing to delete.")
		return
	}

	// Display all comments and posts marked for deletion
	fmt.Printf("Found %d items older than %d days:\n", len(toDelete), *days)
	fmt.Println(strings.Repeat("-", 80))
	for i, item := range toDelete {
		fmt.Printf("%3d. [%-7s] r/%-15s (%s)\n     %s\n\n",
			i+1,
			item.Kind,
			item.Subreddit,
			item.CreatedAt.Format("2006-01-02"),
			preview(item.Content, 75),
		)
	}
	fmt.Println(strings.Repeat("-", 80))

	// Default is dry-run: do not delete
	if *dryRun {
		fmt.Printf("[Dry Run Complete] %d items would be deleted.\n", len(toDelete))
		fmt.Println("No items were modified. To actually delete these items, add the -delete flag:")
		fmt.Printf("  go run . -days %d -delete\n", *days)
		return
	}

	// In delete mode: require confirmation prompt unless -yes is passed
	if !*yes {
		fmt.Printf("WARNING: Deleting %d items cannot be undone!\n", len(toDelete))
		fmt.Print("Are you sure you want to permanently delete them? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))

		if input != "y" && input != "yes" {
			fmt.Println("Aborted. No items were deleted.")
			return
		}
	}

	// Perform deletion
	fmt.Println("\nDeleting items...")
	deleted := 0
	for i, item := range toDelete {
		if err := client.Delete(item.Name); err != nil {
			fmt.Printf("[%d/%d] Failed to delete %s: %v\n", i+1, len(toDelete), item.Name, err)
		} else {
			fmt.Printf("[%d/%d] Deleted %s in r/%s\n", i+1, len(toDelete), item.Kind, item.Subreddit)
			deleted++
		}
		time.Sleep(500 * time.Millisecond) // Respect Reddit rate limits
	}

	fmt.Printf("\nFinished. Successfully deleted %d of %d items.\n", deleted, len(toDelete))
}

// loadEnv parses a simple KEY=VALUE file and sets environment variables if not already set.
func loadEnv(filepath string) {
	f, err := os.Open(filepath)
	if err != nil {
		return // File does not exist, ignore quietly
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
	}
}

func preview(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
