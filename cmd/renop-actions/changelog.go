/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func runChangelog(args []string) error {
	fs := flag.NewFlagSet("changelog", flag.ContinueOnError)
	commit := fs.String("commit", "HEAD", "End of the range")
	Commit := fs.String("Commit", "", "Alias for commit")
	outFile := fs.String("out", "", "Optional path to write the notes")
	OutFile := fs.String("OutFile", "", "Alias for out")
	outFileAlias := fs.String("outfile", "", "Alias for out")
	OutFileAlias := fs.String("Outfile", "", "Alias for out")
	footer := fs.String("footer", "", "Optional footer text")
	Footer := fs.String("Footer", "", "Alias for footer")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cmt := *commit
	if *Commit != "" {
		cmt = *Commit
	}
	out := *outFile
	if *OutFile != "" {
		out = *OutFile
	} else if *outFileAlias != "" {
		out = *outFileAlias
	} else if *OutFileAlias != "" {
		out = *OutFileAlias
	}
	ftr := *footer
	if *Footer != "" {
		ftr = *Footer
	}

	resolvedCommitBytes, err := exec.Command("git", "rev-parse", cmt).Output()
	if err != nil {
		return errors.New("Could not resolve Commit")
	}
	resolvedCommit := strings.TrimSpace(string(resolvedCommitBytes))
	if resolvedCommit == "" {
		return errors.New("Could not resolve Commit")
	}

	subjectBytes, _ := exec.Command("git", "log", "-1", "--format=%s", resolvedCommit).Output()
	headSubject := strings.TrimSpace(string(subjectBytes))
	isReleaseCommit := strings.HasPrefix(strings.ToLower(headSubject), "[release]")

	var logArgs []string
	if isReleaseCommit {
		prev := findPreviousReleaseRef(resolvedCommit)
		if prev != "" {
			fmt.Fprintf(os.Stderr, "Changelog range: %s..%s\n", prev, resolvedCommit)
			logArgs = []string{"log", "--format=%B\x1e", prev + ".." + resolvedCommit}
		} else {
			fmt.Fprintf(os.Stderr, "Changelog range: full history through %s (no previous release found)\n", resolvedCommit)
			logArgs = []string{"log", "--format=%B\x1e", resolvedCommit}
		}
	} else {
		fmt.Fprintf(os.Stderr, "Changelog range: single commit %s (nightly build)\n", resolvedCommit)
		logArgs = []string{"log", "-1", "--format=%B\x1e", resolvedCommit}
	}

	rawBytes, err := exec.Command("git", logArgs...).Output()
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	raw := string(rawBytes)
	var entries []string
	seen := make(map[string]bool)

	if strings.TrimSpace(raw) != "" {
		chunks := strings.SplitSeq(raw, "\x1e")
		for chunk := range chunks {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			line := getFirstParagraphLine(chunk)
			if testOmitSubject(line) {
				continue
			}
			entry := line
			if isReleaseCommit {
				if !strings.HasPrefix(entry, "- ") {
					entry = "- " + entry
				}
			}
			if !seen[entry] {
				seen[entry] = true
				entries = append(entries, entry)
			}
		}
	}

	body := strings.Join(entries, "\n")
	if strings.TrimSpace(ftr) != "" {
		footerText := strings.TrimSpace(ftr)
		if strings.TrimSpace(body) == "" {
			body = footerText
		} else {
			body = body + "\n\n" + footerText
		}
	}

	if out != "" {
		absOut, err := filepath.Abs(out)
		if err == nil {
			out = absOut
		}
		parent := filepath.Dir(out)
		if parent != "" {
			os.MkdirAll(parent, 0755)
		}
		if err := os.WriteFile(out, []byte(body), 0644); err != nil {
			return fmt.Errorf("failed to write changelog: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote changelog (%d entries) to %s\n", len(entries), out)
	} else if body != "" {
		fmt.Println(body)
	}

	return nil
}

func testOmitSubject(subject string) bool {
	s := strings.TrimSpace(strings.ToLower(subject))
	if s == "" {
		return true
	}
	prefixes := []string{"[release]", "[ci skip]", "[skip ci]", "[web]"}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func getFirstParagraphLine(message string) string {
	msg := strings.ReplaceAll(message, "\r\n", "\n")
	msg = strings.ReplaceAll(msg, "\r", "\n")
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	paras := regexp.MustCompile(`\n\s*\n`).Split(msg, 2)
	firstPara := strings.TrimSpace(paras[0])
	lines := strings.Split(firstPara, "\n")
	return strings.TrimSpace(lines[0])
}

func findPreviousReleaseRef(head string) string {
	out, err := exec.Command("git", "log", "-1", "--format=%H", "-i", "--grep=^\\[release\\]", head+"^").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return strings.TrimSpace(string(out))
	}
	out, err = exec.Command("git", "describe", "--tags", "--abbrev=0", head+"^").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return strings.TrimSpace(string(out))
	}
	return ""
}
