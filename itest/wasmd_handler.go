package e2etest

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

// runScript executes a shell script and returns the output and error (if any).
func runScript(script string) (string, error) {
	cmd := exec.Command("/bin/sh", script)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// runScriptInBackground runs a shell script in the background and returns the process.
func runScriptInBackground(script string) (*exec.Cmd, error) {
	cmd := exec.Command("/bin/sh", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, err
}

// StartWasmdNode triggers the setup and start node scripts, then creates the accounts.
func StartWasmdNode() {
	// Run setup script
	setupScript := "wasmd_scripts/setup_wasmd.sh"
	output, err := runScript(setupScript)
	if err != nil {
		log.Fatalf("Error running script %s: %s\nOutput: %s", setupScript, err, output)
	}
	fmt.Printf("Script %s output:\n%s\n", setupScript, output)

	// Introduce a delay before running the next script
	time.Sleep(5 * time.Second) // Adjust the delay as needed

	// Run start node script in the background
	startNodeScript := "wasmd_scripts/start_node.sh"
	_, err = runScriptInBackground(startNodeScript)
	if err != nil {
		log.Fatalf("Error running script %s: %s", startNodeScript, err)
	}
	fmt.Printf("Script %s started in background.\n", startNodeScript)

	// Introduce a delay to ensure the node starts properly before running the next script
	time.Sleep(10 * time.Second) // Adjust the delay as needed

	// Run accounts script
	accountsScript := "wasmd_scripts/01-accounts.sh"
	output, err = runScript(accountsScript)
	if err != nil {
		log.Fatalf("Error running script %s: %s\nOutput: %s", accountsScript, err, output)
	}
	fmt.Printf("Script %s output:\n%s\n", accountsScript, output)
}

// StopWasmdNode stops the wasmd node process.
func StopWasmdNode() {
	// This is an example of stopping the process. Adjust the command based on your specific needs.
	cmd := exec.Command("pkill", "wasmd")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Error stopping wasmd node: %s", err)
	}
	fmt.Println("wasmd node stopped successfully.")
}
