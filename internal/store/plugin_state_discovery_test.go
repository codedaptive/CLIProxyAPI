package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	sdkauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

func TestTokenStoreListSkipsPluginState(t *testing.T) {
	for _, backend := range []string{"file", "git", "object"} {
		t.Run(backend, func(t *testing.T) {
			root := t.TempDir()
			// A user may name the auth root itself plugin-state. Only descendant
			// plugin-state directories are reserved for plugin data.
			authDir := filepath.Join(root, "workspace", "plugin-state")
			var store coreauth.Store
			switch backend {
			case "file":
				s := sdkauth.NewFileTokenStore()
				s.SetBaseDir(authDir)
				store = s
			case "git":
				remote := setupGitRemoteRepository(t, root, "master",
					testBranchSpec{name: "master", contents: "test repository\n"})
				s := NewGitTokenStore(remote, "", "", "")
				s.SetBaseDir(authDir)
				if err := s.EnsureRepository(); err != nil {
					t.Fatal(err)
				}
				store = s
			case "object":
				store = &ObjectTokenStore{authDir: authDir}
			}

			files := map[string]string{
				"account.json":                             `{"type":"claude","access_token":"test-only"}`,
				"team/account.json":                        `{"type":"claude","access_token":"test-only"}`,
				"plugin-state.json":                        `{"type":"claude","access_token":"test-only"}`,
				"plugin-state-backup/account.json":         `{"type":"claude","access_token":"test-only"}`,
				"plugin-state/codex-auto-reset/state.json": `{"config":{},"accounts":{},"logs":[],"reset_history":[]}`,
				// Directory exclusion must apply even if state resembles a credential.
				"plugin-state/other/state.json":      `{"type":"claude","access_token":"test-only"}`,
				"team/PLUGIN-STATE/other/state.json": `{"type":"claude","access_token":"test-only"}`,
			}
			for name, contents := range files {
				path := filepath.Join(authDir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			for scan := 0; scan < 2; scan++ {
				auths, err := store.List(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, auth := range auths {
					got = append(got, filepath.ToSlash(auth.ID))
				}
				sort.Strings(got)
				want := []string{"account.json", "plugin-state-backup/account.json", "plugin-state.json", "team/account.json"}
				if len(got) != len(want) {
					t.Fatalf("scan %d: got %v, want %v", scan, got, want)
				}
				for i := range want {
					if got[i] != want[i] {
						t.Fatalf("scan %d: got %v, want %v", scan, got, want)
					}
				}
			}
			for name, want := range files {
				data, err := os.ReadFile(filepath.Join(authDir, filepath.FromSlash(name)))
				if err != nil || string(data) != want {
					t.Errorf("discovery changed %s: %v", name, err)
				}
			}
		})
	}
}
