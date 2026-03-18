package command

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// VersionCommand returns the running version.
type VersionCommand struct{}

func (c *VersionCommand) Name() string {
	return "version"
}

func (c *VersionCommand) Handle(ctx *Context, _ []string) (string, error) {
	output := []string{}
	if ctx.IsDM {
		output = append(output, fmt.Sprintf("Dexter version %s", ctx.Version()))
	}
	if !ctx.IsAdmin {
		return strings.TrimSpace(strings.Join(output, "\n")), nil
	}
	status := gitLines(ctx.Root, "status")
	logs := gitLines(ctx.Root, "--no-pager", "log", "-3")
	statusLines := []string{}
	for _, line := range status {
		if strings.Contains(line, "On branch") || strings.Contains(line, "modified") || strings.Contains(line, "renamed") {
			statusLines = append(statusLines, line)
		}
	}
	updates := formatGitUpdates(logs)
	if len(statusLines) > 0 {
		output = append(output, fmt.Sprintf("**Git status** \n%s \n\n **Recent updates**", strings.Join(statusLines, "\n")))
	} else {
		output = append(output, "**Recent updates**")
	}
	if len(updates) > 0 {
		output = append(output, updates...)
	}
	return strings.TrimSpace(strings.Join(output, "\n\n")), nil
}

func gitLines(root string, args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	raw := strings.TrimSpace(out.String())
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

func formatGitUpdates(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	blocks := [][]string{}
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "commit ") {
			if start >= 0 {
				blocks = append(blocks, lines[start:i])
			}
			start = i
		}
	}
	if start >= 0 {
		blocks = append(blocks, lines[start:])
	}
	output := []string{}
	for _, block := range blocks {
		if len(block) == 0 {
			continue
		}
		hash := strings.TrimSpace(strings.TrimPrefix(block[0], "commit "))
		author := ""
		date := ""
		messageLines := []string{}
		for _, line := range block[1:] {
			if strings.HasPrefix(line, "Author:") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					author = fields[1]
				}
				continue
			}
			if strings.HasPrefix(line, "Date:") {
				date = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
				continue
			}
			if strings.HasPrefix(line, "    ") {
				value := strings.TrimSpace(line)
				if value != "" {
					messageLines = append(messageLines, value)
				}
			}
		}
		if date != "" {
			if parsed, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", date); err == nil {
				date = parsed.Format("1/2/2006")
			}
		}
		if hash != "" {
			short := hash
			if len(short) > 6 {
				short = short[:6]
			}
			message := strings.Join(messageLines, "\n")
			if len(message) > 1023 {
				message = message[:1023]
			}
			header := strings.TrimSpace(strings.Join([]string{author, date}, " - "))
			if header != "" {
				output = append(output, fmt.Sprintf("%s\n[%s] - %s", header, short, message))
			} else {
				output = append(output, fmt.Sprintf("[%s] - %s", short, message))
			}
		}
	}
	return output
}
