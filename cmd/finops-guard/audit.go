package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// printGitLedger reads the refs/notes/finops git notes and renders them as a
// formatted historical spend ledger in the terminal.
func printGitLedger() {
	out, err := exec.Command("git", "log",
		"--notes=refs/notes/finops",
		"--pretty=format:%h - %s%n%N",
	).Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		fmt.Println("❌ No FinOps Git ledger notes found on this repository yet.")
		fmt.Println("   (Notes are recorded on merges; fetch them with:")
		fmt.Println("    git fetch origin refs/notes/finops:refs/notes/finops )")
		return
	}

	bar := strings.Repeat("=", 40)
	fmt.Println(bar)
	fmt.Println("   FINOPS-GUARD ★ HISTORICAL LEDGER")
	fmt.Println(bar)
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "finops:") || strings.Contains(t, "$") {
			// A spend-delta note line.
			fmt.Printf("   💰 %s\n", t)
		} else {
			// A commit header line (hash - subject).
			fmt.Printf("\n%s\n", t)
		}
	}
	fmt.Println(bar)
}
