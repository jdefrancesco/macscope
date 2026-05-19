package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jdefrancesco/macscope/internal/output"
)

type completionFlag struct {
	Long        string
	Short       string
	Description string
}

type completionCommand struct {
	Name        string
	Summary     string
	Flags       []completionFlag
	ArgChoices  []string
	FileArgs    bool
	GenericArgs bool
}

func runCompletion(ctx context.Context, args []string, streams output.Streams) error {
	_ = ctx

	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printCompletionHelp(streams.Out)
		return nil
	}
	if len(args) != 1 {
		return errors.New("usage: macscope completion <bash|zsh|fish>")
	}

	script, err := completionScript(args[0])
	if err != nil {
		return err
	}
	_, err = io.WriteString(streams.Out, script)
	return err
}

func printCompletionHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope completion <bash|zsh|fish>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Generate shell completion scripts for local installation or direct sourcing.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  source <(macscope completion bash)")
	fmt.Fprintln(w, "  source <(macscope completion zsh)")
	fmt.Fprintln(w, "  macscope completion fish | source")
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q; choose bash, zsh, or fish", shell)
	}
}

func completionCommands() []completionCommand {
	items := []completionCommand{
		{Name: "help", Summary: "Show help."},
	}
	for _, cmd := range commands() {
		items = append(items, completionCommand{
			Name:        cmd.Name,
			Summary:     cmd.Summary,
			Flags:       completionFlags(cmd.Name),
			ArgChoices:  completionArgChoices(cmd.Name),
			FileArgs:    completionFileArgs(cmd.Name),
			GenericArgs: completionGenericArgs(cmd.Name),
		})
	}
	return items
}

func completionFlags(command string) []completionFlag {
	help := completionFlag{Long: "help", Short: "h", Description: "Show command help."}
	json := completionFlag{Long: "json", Description: "Emit stable JSON."}
	last := completionFlag{Long: "last", Description: "Set unified-log lookback window."}

	switch command {
	case "version":
		return []completionFlag{help, json}
	case "macho":
		return []completionFlag{
			help,
			json,
			{Long: "full", Description: "Include raw command output in JSON."},
			{Long: "triage", Description: "Show compact triage score and evidence."},
		}
	case "proc":
		return []completionFlag{help, json}
	case "attach":
		return []completionFlag{help, json, last}
	case "persist":
		return []completionFlag{
			help,
			json,
			{Long: "dir", Description: "Inspect a launchd directory."},
		}
	case "tcc", "es":
		return []completionFlag{
			help,
			json,
			last,
			{Long: "watch", Description: "Stream matching unified logs."},
		}
	case "vpn":
		return []completionFlag{help, json, last}
	case "panic":
		return []completionFlag{
			help,
			json,
			{Long: "last", Description: "Parse the newest panic report."},
			{Long: "file", Description: "Parse a specific panic report."},
			{Long: "since", Description: "Parse reports modified within a duration."},
		}
	case "timeline":
		return []completionFlag{
			help,
			json,
			{Long: "jsonl", Description: "Emit normalized events as JSON Lines."},
			last,
			{Long: "pid", Description: "Target process ID."},
		}
	case "completion":
		return []completionFlag{help}
	default:
		return nil
	}
}

func completionArgChoices(command string) []string {
	if command == "completion" {
		return []string{"bash", "zsh", "fish"}
	}
	return nil
}

func completionFileArgs(command string) bool {
	switch command {
	case "macho", "panic", "persist":
		return true
	default:
		return false
	}
}

func completionGenericArgs(command string) bool {
	switch command {
	case "proc", "attach", "vpn", "timeline":
		return true
	default:
		return false
	}
}

func completionCommandNames() []string {
	commands := completionCommands()
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Name)
	}
	return names
}

func completionWords(cmd completionCommand) []string {
	var words []string
	for _, flag := range cmd.Flags {
		if flag.Short != "" {
			words = append(words, "-"+flag.Short)
		}
		if flag.Long != "" {
			words = append(words, "--"+flag.Long)
		}
	}
	words = append(words, cmd.ArgChoices...)
	return words
}

func bashCompletionScript() string {
	var b strings.Builder
	b.WriteString("# bash completion for macscope\n")
	b.WriteString("_macscope()\n")
	b.WriteString("{\n")
	b.WriteString("\tlocal cur cmd\n")
	b.WriteString("\tCOMPREPLY=()\n")
	b.WriteString("\tcur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("\n")
	b.WriteString("\tif [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	fmt.Fprintf(&b, "\t\tCOMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(completionCommandNames(), " "))
	b.WriteString("\t\treturn 0\n")
	b.WriteString("\tfi\n")
	b.WriteString("\n")
	b.WriteString("\tcmd=\"${COMP_WORDS[1]}\"\n")
	b.WriteString("\tcase \"$cmd\" in\n")
	for _, cmd := range completionCommands() {
		words := completionWords(cmd)
		if len(words) == 0 && !cmd.FileArgs && !cmd.GenericArgs {
			continue
		}
		fmt.Fprintf(&b, "\t\t%s)\n", cmd.Name)
		if len(words) > 0 {
			fmt.Fprintf(&b, "\t\t\tCOMPREPLY=( $(compgen -W %q -- \"$cur\") )\n", strings.Join(words, " "))
		} else {
			b.WriteString("\t\t\tCOMPREPLY=()\n")
		}
		if cmd.FileArgs {
			b.WriteString("\t\t\tcompopt -o default 2>/dev/null || true\n")
		}
		b.WriteString("\t\t\treturn 0\n")
		b.WriteString("\t\t\t;;\n")
	}
	b.WriteString("\tesac\n")
	b.WriteString("}\n")
	b.WriteString("complete -F _macscope macscope\n")
	return b.String()
}

func zshCompletionScript() string {
	var b strings.Builder
	b.WriteString("#compdef macscope\n\n")
	b.WriteString("local -a commands\n")
	b.WriteString("commands=(\n")
	for _, cmd := range completionCommands() {
		fmt.Fprintf(&b, "\t%s\n", zshQuote(cmd.Name+":"+cmd.Summary))
	}
	b.WriteString(")\n\n")
	b.WriteString("_arguments -C \\\n")
	b.WriteString("\t'1:command:->command' \\\n")
	b.WriteString("\t'*::arg:->args'\n\n")
	b.WriteString("case $state in\n")
	b.WriteString("\tcommand)\n")
	b.WriteString("\t\t_describe -t commands 'macscope commands' commands\n")
	b.WriteString("\t\t;;\n")
	b.WriteString("\targs)\n")
	b.WriteString("\t\tcase $words[2] in\n")
	for _, cmd := range completionCommands() {
		if len(cmd.Flags) == 0 && len(cmd.ArgChoices) == 0 && !cmd.FileArgs && !cmd.GenericArgs {
			continue
		}
		fmt.Fprintf(&b, "\t\t\t%s)\n", cmd.Name)
		b.WriteString("\t\t\t\t_arguments \\\n")
		specs := zshSpecs(cmd)
		for i, spec := range specs {
			suffix := " \\\n"
			if i == len(specs)-1 {
				suffix = "\n"
			}
			fmt.Fprintf(&b, "\t\t\t\t\t%s%s", zshQuote(spec), suffix)
		}
		b.WriteString("\t\t\t\t;;\n")
	}
	b.WriteString("\t\tesac\n")
	b.WriteString("\t\t;;\n")
	b.WriteString("esac\n")
	return b.String()
}

func zshSpecs(cmd completionCommand) []string {
	specs := make([]string, 0, len(cmd.Flags)+2)
	for _, flag := range cmd.Flags {
		switch {
		case flag.Short != "" && flag.Long != "":
			specs = append(specs, fmt.Sprintf("(-%s --%s){-%s,--%s}[%s]", flag.Short, flag.Long, flag.Short, flag.Long, flag.Description))
		case flag.Long != "":
			specs = append(specs, fmt.Sprintf("--%s[%s]", flag.Long, flag.Description))
		case flag.Short != "":
			specs = append(specs, fmt.Sprintf("-%s[%s]", flag.Short, flag.Description))
		}
	}
	if len(cmd.ArgChoices) > 0 {
		specs = append(specs, "1:shell:("+strings.Join(cmd.ArgChoices, " ")+")")
	}
	if cmd.FileArgs {
		specs = append(specs, "*:path:_files")
	} else if cmd.GenericArgs {
		specs = append(specs, "*:argument:")
	}
	return specs
}

func fishCompletionScript() string {
	var b strings.Builder
	b.WriteString("# fish completion for macscope\n")
	b.WriteString("complete -c macscope -f\n")
	for _, cmd := range completionCommands() {
		fmt.Fprintf(&b, "complete -c macscope -n '__fish_use_subcommand' -a %s -d %s\n", fishQuote(cmd.Name), fishQuote(cmd.Summary))
	}
	for _, cmd := range completionCommands() {
		for _, flag := range cmd.Flags {
			condition := fishQuote("__fish_seen_subcommand_from " + cmd.Name)
			description := fishQuote(flag.Description)
			if flag.Short != "" {
				fmt.Fprintf(&b, "complete -c macscope -n %s -s %s -d %s\n", condition, flag.Short, description)
			}
			if flag.Long != "" {
				fmt.Fprintf(&b, "complete -c macscope -n %s -l %s -d %s\n", condition, flag.Long, description)
			}
		}
		for _, choice := range cmd.ArgChoices {
			fmt.Fprintf(&b, "complete -c macscope -n %s -a %s\n", fishQuote("__fish_seen_subcommand_from "+cmd.Name), fishQuote(choice))
		}
	}
	return b.String()
}

func zshQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func fishQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}
