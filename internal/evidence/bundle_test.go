package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jdefrancesco/macscope/internal/collect"
)

type bundleRunner struct {
	denyMaps  bool
	blockFile string
	cancel    context.CancelFunc
}

func (r bundleRunner) Run(_ context.Context, name string, args ...string) (collect.Result, error) {
	if r.blockFile != "" {
		if err := os.WriteFile(r.blockFile, []byte("existing evidence"), 0600); err != nil {
			return collect.Result{}, err
		}
	}
	result := collect.Result{Command: append([]string{name}, args...)}
	switch name {
	case "ps":
		result.Stdout = "42 20 alice staff S /Applications/Example /Applications/Example App.app/Contents/MacOS/Example App\n"
	case "/bin/ps":
		data, _ := os.ReadFile("../../testdata/process/tree.txt")
		result.Stdout = string(data)
		if args[0] != "-axww" {
			result.Stdout = "42 20 Mon Oct 26 09:00:00 2026 /Applications/Example App.app/Contents/MacOS/Example App\n"
		}
	case "codesign":
		result.Stdout = "Identifier=example\nTeamIdentifier=EXAMPLE\n"
	case "/usr/bin/vmmap":
		if r.cancel != nil {
			r.cancel()
		}
		if r.denyMaps {
			result.Stderr = "permission denied"
			return result, errors.New("exit 1")
		}
		data, _ := os.ReadFile("../../testdata/memory/vmmap.txt")
		result.Stdout = string(data)
	case "/usr/bin/lsmp":
		data, _ := os.ReadFile("../../testdata/ipc/lsmp.json")
		if err := os.WriteFile(args[3], data, 0600); err != nil {
			return result, err
		}
		result.Stdout = "raw Mach evidence"
	case "/usr/sbin/lsof":
		result.Stdout = "p42\x00\nf3\x00tPIPE\x00n->0x1\x00\n"
	default:
		return result, errors.New("unexpected command " + name)
	}
	return result, nil
}

func TestBundleWriteFailureAndCancellation(t *testing.T) {
	for _, name := range []string{"proc.json", "commands.json", "manifest.json"} {
		out := filepath.Join(t.TempDir(), "new")
		manifest, err := Create(context.Background(), Options{PID: 42, Out: out, Reports: []string{"proc"}, Runner: bundleRunner{blockFile: filepath.Join(out, name)}})
		if err == nil || manifest.Status != "partial" || manifest.ManifestPath != "" || len(manifest.Warnings) == 0 {
			t.Fatalf("manifest=%#v err=%v", manifest, err)
		}
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil || string(data) != "existing evidence" {
			t.Fatalf("existing data=%q err=%v", data, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := filepath.Join(t.TempDir(), "canceled")
	manifest, err := Create(ctx, Options{PID: 42, Out: out, Reports: []string{"maps", "tree"}, Runner: bundleRunner{cancel: cancel}})
	if !errors.Is(err, context.Canceled) || manifest.Status != "partial" || manifest.ManifestPath != "manifest.json" {
		t.Fatalf("manifest=%#v err=%v", manifest, err)
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCreateBundle(t *testing.T) {
	for _, deny := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "evidence")
		manifest, err := Create(context.Background(), Options{PID: 42, Out: out, Version: "test", Runner: bundleRunner{denyMaps: deny}})
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Reports) != 4 || len(manifest.Files) != 5 || manifest.StartedAt == "" || manifest.CreatedAt.IsZero() || manifest.CompletedAt.Before(manifest.CreatedAt) {
			t.Fatalf("manifest=%#v", manifest)
		}
		if (manifest.Status == "partial") != deny {
			t.Fatalf("status=%s deny=%v", manifest.Status, deny)
		}
		info, _ := os.Stat(out)
		if info.Mode().Perm() != 0700 {
			t.Fatalf("directory mode=%o", info.Mode().Perm())
		}
		for _, entry := range manifest.Files {
			data, err := os.ReadFile(filepath.Join(out, entry.Path))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			if entry.SHA256 != hex.EncodeToString(sum[:]) || entry.SizeBytes != int64(len(data)) {
				t.Fatalf("checksum mismatch: %#v", entry)
			}
			info, _ := os.Stat(filepath.Join(out, entry.Path))
			if info.Mode().Perm() != 0600 {
				t.Fatalf("file mode=%o", info.Mode().Perm())
			}
		}
		data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
		if err != nil || !json.Valid(data) {
			t.Fatalf("manifest err=%v data=%s", err, data)
		}
		data, _ = os.ReadFile(filepath.Join(out, "commands.json"))
		if !strings.Contains(string(data), "captured_at") || deny && !strings.Contains(string(data), "permission denied") {
			t.Fatalf("raw evidence missing: %s", data)
		}
	}
}

func TestBundleRefusesOverwriteAndInvalidSelection(t *testing.T) {
	out := t.TempDir()
	sentinel := filepath.Join(out, "keep")
	os.WriteFile(sentinel, []byte("keep"), 0600)
	if _, err := Create(context.Background(), Options{PID: 42, Out: out, Runner: bundleRunner{}}); err == nil {
		t.Fatal("overwrote existing directory")
	}
	data, _ := os.ReadFile(sentinel)
	if string(data) != "keep" {
		t.Fatal("modified existing evidence")
	}
	for _, reports := range [][]string{{"../../escape"}, {"maps", "maps"}, {""}} {
		path := filepath.Join(t.TempDir(), "new")
		if _, err := Create(context.Background(), Options{PID: 42, Out: path, Reports: reports, Runner: bundleRunner{}}); err == nil {
			t.Fatalf("accepted reports %v", reports)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("invalid selection created output")
		}
	}
	path := filepath.Join(t.TempDir(), "selected")
	m, err := Create(context.Background(), Options{PID: 42, Out: path, Reports: []string{"tree"}, Runner: bundleRunner{}})
	if err != nil || len(m.Reports) != 1 || len(m.Files) != 2 {
		t.Fatalf("selected=%#v err=%v", m, err)
	}
}
