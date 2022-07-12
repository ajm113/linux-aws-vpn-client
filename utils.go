package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

func openDefaultBrowser(url string) (err error) {
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported patform %s", runtime.GOOS)
	}

	return
}

func commandExists(command string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+command)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func fileExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}

	return false
}

func getHomeDirConfigPath() (foldername string, err error) {
	home, err := os.UserHomeDir()

	if err != nil {
		return
	}

	configFolder := ".config"

	if runtime.GOOS == "windows" {
		configFolder = "AppData\\Local"
	}

	foldername = path.Join(home, configFolder, defaultConfigDirectoryName)

	return
}

func searchConfigFilename() (string, error) {

	wd, wdErr := os.Getwd()

	if wdErr != nil {
		return "", wdErr
	}

	configAtwd := path.Join(wd, defaultConfigFilename)

	home, _ := getHomeDirConfigPath()
	configAtHome := path.Join(home, defaultConfigFilename)

	if fileExists(configAtwd) {
		return configAtwd, nil
	} else if fileExists(configAtHome) {
		return configAtHome, nil
	}

	return "", os.ErrNotExist
}

func lookupIP(hostname string) (ip string, err error) {
	ips, err := net.LookupIP(hostname)

	if err != nil || len(ips) == 0 {
		return
	}

	for _, ipv4 := range ips {
		ip = ipv4.String()
		break
	}

	return
}

func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func extractSIDFromOpenVPN(output string) (SID string, err error) {
	tokens := strings.Split(output, ":")

	for _, t := range tokens {
		if strings.HasPrefix(t, "instance-") {
			SID = t
			break
		}
	}

	if SID == "" {
		err = fmt.Errorf("sid not found")
	}

	return
}

func downloadFileAsTemp(outDir, pattern, url, shaCheck string) (filepath string, err error) {
	out, err := os.CreateTemp(outDir, pattern)
	if err != nil {
		return
	}

	defer out.Close()
	filepath = out.Name()

	resp, err := http.Get(url)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	// Do some status checks to make sure we recived everything OK.
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s (%d)", resp.Status, resp.StatusCode)
	}

	// Do a sha256 check to make sure the data coming is correct.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	h := sha256.New()
	h.Write(b)
	downloadSha := fmt.Sprintf("%x", h.Sum(nil))

	if shaCheck != "" && downloadSha != shaCheck {
		return "", fmt.Errorf("unexpected SHA256 (%s) expected (%s)", downloadSha, shaCheck)
	}

	_, err = out.Write(b)
	if err != nil {
		return
	}

	return
}

func copyFile(src, dst string, bufferSize int64) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	buf := make([]byte, bufferSize)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return err
		}
	}

	return nil
}
