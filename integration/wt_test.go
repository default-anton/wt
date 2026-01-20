package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

var wtBinDir string

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	repoRoot := filepath.Dir(wd)

	binDir, err := os.MkdirTemp("", "wt-integration-bin-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(binDir)

	binPath := filepath.Join(binDir, "wt")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/wt")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(err)
	}

	wtBinDir = binDir

	os.Exit(m.Run())
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "script"),
		Setup: func(env *testscript.Env) error {
			home := filepath.Join(env.WorkDir, "home")
			if err := os.MkdirAll(home, 0755); err != nil {
				return err
			}

			env.Setenv("PATH", wtBinDir+string(os.PathListSeparator)+env.Getenv("PATH"))
			env.Setenv("HOME", home)
			env.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			env.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			return nil
		},
	})
}
