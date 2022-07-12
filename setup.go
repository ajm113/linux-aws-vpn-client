package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

const (
	openVPNURLSupportURL string = "https://openvpn.net/community/"
	openVPNRelease       string = "https://swupdate.openvpn.org/community/releases/openvpn-2.5.1.tar.xz"
	openVPNReleaseMirror string = "https://build.openvpn.net/downloads/releases/openvpn-2.5.1.tar.xz"
	openVPNSha           string = ""

	openVPNBuildTopLevelDir string = "aws-openvpn"
	openVPNBuildDest        string = "/tmp"
)

var (
	openVPNBuildDirPath string = path.Join(openVPNBuildDest, openVPNBuildTopLevelDir)
)

// setupAction Compiles and builds patched version of openvpn.
func setupAction(c *cli.Context) error {
	log.Info().Msg("creating ready openvpn binary")

	// Safe place to keep temp build files.
	tempDir := c.String("tmpDir")

	var quit bool
	if !commandExists("tar") {
		log.Error().Msg("git not found! Please install git to continue." + errorSuffix)
		quit = true
	}

	if !commandExists("patch") {
		log.Error().Msg("Ruby not found! Please install patch to continue." + errorSuffix)
		quit = true
	}

	if !commandExists("make") {
		log.Error().Msg("Make not found! Please install build-essentials or development tools to continue." + errorSuffix)
		quit = true
	}

	if quit {
		return fmt.Errorf("one or more commands not found")
	}

	log.Info().Msg("fetching OpenVPN code and patch data online (this may take a moment)...")
	log.Debug().Str("openVPNURL", openVPNRelease).Str("openVPNSha", openVPNSha).Msg("downloading OpenVPN...")
	openVPNTarName, err := downloadFileAsTemp(tempDir, "*openvpn-2.5.1.tar.xz", openVPNRelease, openVPNSha)
	if err != nil {
		return fmt.Errorf("failed downloading OpenVPN code: %s %s", err, errorSuffix)
	}

	log.Debug().Str("openVPNTarName", openVPNTarName).Msg("downloaded openvpn tar file")
	log.Info().Msg("unpacking openvpn tar file")

	tarCommand := exec.Command(
		"tar",
		"-xf", openVPNTarName,
		"-C", openVPNBuildDest,
		"--strip-components", "1",
		"--one-top-level="+openVPNBuildTopLevelDir,
	)

	out, err := tarCommand.CombinedOutput()
	log.Debug().Str("command", tarCommand.String()).Str("payload", string(out)).Msg("unpacked tar file")

	if err != nil {
		return err
	}

	log.Info().Msg("compiling openvpn")

	configureCommand := exec.Command(
		"./configure",
		"--disable-debug",
		"--disable-dependency-tracking",
		"--disable-silent-rules",
		"--with-crypto-library=openssl",
		// "--enable-pkcs11",
	)
	configureCommand.Dir = openVPNBuildDirPath
	configureCommand.Env = os.Environ()
	configureCommand.Stdout = os.Stdout
	configureCommand.Stderr = os.Stderr
	configureCommand.Stdin = os.Stdin

	log.Debug().Str("command", configureCommand.String()).Msg("executing ./configure")

	err = configureCommand.Start()
	if err != nil {
		return err
	}

	err = configureCommand.Wait()
	if err != nil {
		return err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	patchCommand := exec.Command(
		"patch",
		"-ruN",
		"-p0",
		"-f",
		"-s",
		"-d",
		openVPNBuildDirPath,
		"--input="+path.Join(pwd, "scripts/openvpn-v2.5.1-aws.patch"),
	)

	patchCommand.Env = os.Environ()
	patchCommand.Stdout = os.Stdout
	patchCommand.Stderr = os.Stderr

	log.Debug().
		Str("command", patchCommand.String()).
		Msg("executing patch command")

	err = patchCommand.Start()
	if err != nil {
		return err
	}

	err = patchCommand.Wait()
	if err != nil {
		return err
	}

	makeCommand := exec.Command("make")
	makeCommand.Dir = openVPNBuildDirPath
	makeCommand.Env = os.Environ()
	makeCommand.Stdout = os.Stdout
	makeCommand.Stderr = os.Stderr
	makeCommand.Stdin = os.Stdin

	log.Debug().Str("command", makeCommand.String()).Msg("executing make")

	err = makeCommand.Start()
	if err != nil {
		return err
	}

	err = makeCommand.Wait()
	if err != nil {
		return err
	}

	log.Info().Msg("copying openvpn executable")
	err = copyFile(openVPNBuildDirPath+"/src/openvpn/openvpn", "./openvpn", 5120)

	if err != nil {
		return err
	}

	log.Info().Msg("successfully compiled and copied openvpn executable")
	log.Info().Msg("cleaning up...")

	err = os.RemoveAll(openVPNBuildDirPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed removing openvpn code")
	}

	err = os.Remove(openVPNTarName)
	if err != nil {
		log.Warn().Err(err).Msg("failed removing openvpn tar")
	}

	log.Info().Msg("done")

	return nil
}
