package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func Update(repo, directURL, exe string, restart bool) error {
	url := directURL
	if url == "" {
		if repo == "" {
			return errors.New("set --repo owner/name or --url download-url")
		}
		found, err := latestAssetURL(repo)
		if err != nil {
			return err
		}
		url = found
	}
	tmp, err := os.MkdirTemp("", "cf233-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, filepath.Base(url))
	if err := download(url, archivePath); err != nil {
		return err
	}
	newExe, err := extractBinary(archivePath, tmp)
	if err != nil {
		return err
	}
	backup := exe + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(exe, backup); err != nil {
		return err
	}
	if err := copyFile(newExe, exe, 0755); err != nil {
		_ = os.Rename(backup, exe)
		return err
	}
	_ = os.Remove(backup)
	if restart {
		return Restart(exe, nil)
	}
	return nil
}

func latestAssetURL(repo string) (string, error) {
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("github release lookup failed: %s", resp.Status)
	}
	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	osPart := runtime.GOOS
	archPart := runtime.GOARCH
	if archPart == "amd64" {
		archPart = "x86_64"
	}
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, osPart) && (strings.Contains(name, runtime.GOARCH) || strings.Contains(name, archPart)) {
			return asset.URL, nil
		}
	}
	return "", errors.New("no release asset matched this platform")
}

func download(url, target string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func extractBinary(archivePath, dir string) (string, error) {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZip(archivePath, dir)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGz(archivePath, dir)
	}
	if strings.HasSuffix(lower, ".exe") || !strings.Contains(filepath.Base(lower), ".") {
		return archivePath, nil
	}
	return "", errors.New("unsupported release asset format")
}

func extractZip(archivePath, dir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !looksLikeBinary(file.Name) {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return "", err
		}
		defer src.Close()
		target := filepath.Join(dir, filepath.Base(file.Name))
		return target, copyReader(src, target, 0755)
	}
	return "", errors.New("binary not found in zip")
}

func extractTarGz(archivePath, dir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.FileInfo().IsDir() || !looksLikeBinary(header.Name) {
			continue
		}
		target := filepath.Join(dir, filepath.Base(header.Name))
		return target, copyReader(tr, target, 0755)
	}
	return "", errors.New("binary not found in tar.gz")
}

func looksLikeBinary(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	return base == appName || base == appName+".exe" || strings.HasPrefix(base, "cloudfunction233")
}

func copyReader(src io.Reader, target string, perm os.FileMode) error {
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return copyReader(in, dst, perm)
}
