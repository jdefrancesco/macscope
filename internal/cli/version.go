package cli

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/jdefrancesco/macscope/internal/output"
)

type versionFlags struct {
	json bool
	help bool
}

type versionInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
}

func runVersion(ctx context.Context, args []string, streams output.Streams) error {
	_ = ctx

	flags, err := parseVersionFlags(args)
	if err != nil {
		return err
	}
	if flags.help {
		printVersionHelp(streams.Out)
		return nil
	}

	info := currentVersionInfo()
	if flags.json {
		return output.WriteJSON(streams.Out, info)
	}
	return renderVersion(streams.Out, info)
}

func parseVersionFlags(args []string) (versionFlags, error) {
	var flags versionFlags
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			flags.help = true
		case "--json":
			flags.json = true
		default:
			return versionFlags{}, fmt.Errorf("unknown version arg: %s", arg)
		}
	}
	return flags, nil
}

func currentVersionInfo() versionInfo {
	return versionInfo{
		Name:      "macscope",
		Version:   version,
		Commit:    buildCommit,
		Date:      buildDate,
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
}

func printVersionHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macscope version [--json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Show build version, commit, date, Go version, and target platform.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json   Emit stable JSON.")
}

func renderVersion(w io.Writer, info versionInfo) error {
	if _, err := fmt.Fprintf(w, "%s %s\n", info.Name, info.Version); err != nil {
		return err
	}

	tw := output.NewTextWriter(w)
	for _, kv := range [][2]string{
		{"Commit", info.Commit},
		{"Date", info.Date},
		{"Go", info.GoVersion},
		{"Platform", info.GOOS + "/" + info.GOARCH},
	} {
		if err := tw.KeyValue(kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}
