package dstask

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"

    "github.com/elpatron68/dstask-ui/internal/config"
)

func runGit(t *testing.T, dir string, env []string, args ...string) {
    t.Helper()
    cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
    if env != nil {
        cmd.Env = append(os.Environ(), env...)
    }
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("git %v failed: %v, out=%s", args, err, string(out))
    }
}

func TestGitCloneRemote_SetRemote_And_SetUpstream(t *testing.T) {
    t.Parallel()

    tmp := t.TempDir()

    // Prepare a bare remote with a main branch
    remoteDir := filepath.Join(tmp, "remote.git")
    os.MkdirAll(remoteDir, 0755)
    runGit(t, tmp, nil, "init", "--bare", remoteDir)
    // Create a work repo, commit initial file on main and push
    work := filepath.Join(tmp, "work")
    os.MkdirAll(work, 0755)
    // init and set user
    runGit(t, work, nil, "init")
    runGit(t, work, nil, "checkout", "-b", "main")
    runGit(t, work, nil, "config", "user.email", "test@example.com")
    runGit(t, work, nil, "config", "user.name", "Test User")
    if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0644); err != nil {
        t.Fatal(err)
    }
    runGit(t, work, nil, "add", "README.md")
    runGit(t, work, nil, "commit", "-m", "init")
    runGit(t, work, nil, "remote", "add", "origin", remoteDir)
    runGit(t, work, nil, "push", "-u", "origin", "main")
    // ensure bare remote HEAD points to main
    runGit(t, remoteDir, nil, "symbolic-ref", "HEAD", "refs/heads/main")

    // Prepare app cfg and runner
    home := filepath.Join(tmp, "home")
    if err := os.MkdirAll(filepath.Join(home, ".dstask"), 0755); err != nil {
        t.Fatal(err)
    }
    cfg := &config.Config{DstaskBin: "/bin/true", Repos: map[string]string{"testuser": home}}
    r := NewRunner(cfg)

    // Clone into ~/.dstask
    if err := r.GitCloneRemote("testuser", remoteDir); err != nil {
        t.Fatalf("GitCloneRemote failed: %v", err)
    }
    // Remote URL should be set
    url, err := r.GitRemoteURL("testuser")
    if err != nil {
        t.Fatalf("GitRemoteURL error: %v", err)
    }
    if url == "" {
        t.Fatalf("GitRemoteURL empty, expected %s", remoteDir)
    }

    // Upstream should be set or settable
    if _, err := r.GitSetUpstreamIfMissing("testuser"); err != nil {
        t.Fatalf("GitSetUpstreamIfMissing failed: %v", err)
    }

    // Change remote URL via setter and verify
    newURL := filepath.Join(tmp, "another.git")
    runGit(t, tmp, nil, "init", "--bare", newURL)
    if err := r.GitSetRemoteOrigin("testuser", newURL); err != nil {
        t.Fatalf("GitSetRemoteOrigin failed: %v", err)
    }
    url2, err := r.GitRemoteURL("testuser")
    if err != nil {
        t.Fatalf("GitRemoteURL 2 error: %v", err)
    }
    if url2 != newURL {
        t.Fatalf("expected remote %s, got %s", newURL, url2)
    }
}

func TestGitCloneRemote_NonEmptyDirFails(t *testing.T) {
    t.Parallel()
    tmp := t.TempDir()
    // Set up non-empty ~/.dstask
    home := filepath.Join(tmp, "home")
    dst := filepath.Join(home, ".dstask")
    if err := os.MkdirAll(dst, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(dst, "file.txt"), []byte("x"), 0644); err != nil {
        t.Fatal(err)
    }
    // Prepare bare remote
    remote := filepath.Join(tmp, "r.git")
    runGit(t, tmp, nil, "init", "--bare", remote)

    cfg := &config.Config{DstaskBin: "/bin/true", Repos: map[string]string{"u": home}}
    r := NewRunner(cfg)
    if err := r.GitCloneRemote("u", remote); err == nil {
        t.Fatalf("expected clone to fail into non-empty dir")
    }
}

func TestGitRemoteHeadBranch_DetectsMain(t *testing.T) {
    t.Parallel()
    tmp := t.TempDir()
    // Create bare remote
    remoteDir := filepath.Join(tmp, "remote.git")
    runGit(t, tmp, nil, "init", "--bare", remoteDir)
    // Seed remote with a main branch
    seed := filepath.Join(tmp, "seed")
    os.MkdirAll(seed, 0755)
    runGit(t, seed, nil, "init")
    runGit(t, seed, nil, "checkout", "-b", "main")
    runGit(t, seed, nil, "config", "user.email", "test@example.com")
    runGit(t, seed, nil, "config", "user.name", "Test User")
    if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed"), 0644); err != nil { t.Fatal(err) }
    runGit(t, seed, nil, "add", "README.md")
    runGit(t, seed, nil, "commit", "-m", "seed")
    runGit(t, seed, nil, "remote", "add", "origin", remoteDir)
    runGit(t, seed, nil, "push", "-u", "origin", "main")

    // Ensure bare remote HEAD points to main
    runGit(t, remoteDir, nil, "symbolic-ref", "HEAD", "refs/heads/main")

    // Now local repo representing ~/.dstask
    home := filepath.Join(tmp, "home")
    repo := filepath.Join(home, ".dstask")
    os.MkdirAll(repo, 0755)
    runGit(t, repo, nil, "init")
    // create local main so branch exists
    runGit(t, repo, nil, "checkout", "-b", "main")
    runGit(t, repo, nil, "remote", "add", "origin", remoteDir)
    // fetch so refs/remotes/origin/* exist
    runGit(t, repo, nil, "fetch", "origin")

    cfg := &config.Config{DstaskBin: "/bin/true", Repos: map[string]string{"u": home}}
    r := NewRunner(cfg)
    head, err := r.GitRemoteHeadBranch("u")
    if err != nil {
        t.Fatalf("GitRemoteHeadBranch err: %v", err)
    }
    if head != "main" {
        t.Fatalf("expected main, got %s", head)
    }
}


