package cmd

import "fmt"

func Execute(args []string) error {
	if len(args) < 1 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "run":
		return cmdRun(args[1:])
	case "runs":
		return cmdRuns(args[1:])
	case "search":
		return cmdSearch(args[1:])
	case "ls":
		return cmdLs(args[1:])
	case "thread":
		return cmdThread(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printUsage() {
	fmt.Println(`hiveminer - Extract structured data from Reddit threads

Usage:
  hiveminer <command> [options]

Commands:
  run      Run an extraction pipeline
  runs     View extraction runs and results
  search   Search Reddit posts
  ls       List posts from a subreddit
  thread   View or export thread comments

Run 'hiveminer <command> --help' for details on a specific command.`)
}
