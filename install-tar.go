package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 5 {
		bail("usage install-tar: <dst> <url> <sha256> <version>")
	}

	dst := os.Args[1]
	url := os.Args[2]
	expectSha := strings.ToLower(os.Args[3])
	version := os.Args[4]

	pkgName := filepath.Base(dst)

	var dstExists bool

	d, err := os.Lstat(dst)
	if err == nil {
		if os.ModeSymlink&d.Mode() == 0 {
			bail("dst %s exists but is not a symlink", dst)
		}
		dstExists = true
	} else {
		if !os.IsNotExist(err) {
			bail("stat %s err: %s", dst, err)
		}
	}

	dstDir := filepath.Join(filepath.Dir(dst), fmt.Sprintf(".%s-%s-%s", pkgName, version, expectSha))

	if dstExists {
		_, err := os.Stat(filepath.Join(dst, fmt.Sprintf(".install-tar-%s", expectSha)))
		if err == nil {
			// the package is already there, don't do anything
			return
		}
	}

	f, err := ioutil.TempFile("", "install-tar")
	if err != nil {
		bail("tmpfile err: %s", err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	resp, err := http.Get(url)
	if err != nil {
		bail("Fetch %s err: %s", url, err)
	}

	shaer := sha256.New()

	_, err = io.Copy(f, io.TeeReader(resp.Body, shaer))
	if err != nil {
		bail("Read %s err: %s", url, err)
	}

	err = f.Close()
	if err != nil {
		bail("Close %s err: %s", f.Name(), err)
	}

	shaSum := fmt.Sprintf("%x", shaer.Sum(nil))

	if shaSum != expectSha {
		bail("SHA256 mismach got=%s expect=%s", shaSum, expectSha)
	}

	err = os.RemoveAll(dstDir)
	if err != nil {
		bail("Existing partial install found at %s, could not clean up: %s", dstDir, err)
	}

	err = os.MkdirAll(dstDir, 0777)
	if err != nil {
		bail("Mkdir err %s: %s", dstDir, err)
	}

	cmd := exec.Command("tar", "-C", dstDir, "--strip-components=1", "-xf", f.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		bail("untar err: %s, %s", err, out)
	}

	marker, err := os.Create(filepath.Join(dstDir, fmt.Sprintf(".install-tar-%s", shaSum)))
	if err != nil {
		bail("create marker err: %s", err)
	}
	marker.Close()

	os.Remove(dst)
	err = os.Symlink(dstDir, dst)
	if err != nil {
		bail("Link error: %s", err)
	}
}

func bail(tmpl string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, tmpl+"\n", args...)
	os.Exit(1)
}
