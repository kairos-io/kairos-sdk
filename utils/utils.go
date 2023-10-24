package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"image"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"github.com/joho/godotenv"
	"github.com/pterm/pterm"
	"github.com/qeesung/image2ascii/convert"
)

const (
	systemd = "systemd"
	openrc  = "openrc"
	unknown = "unknown"
)

func SH(c string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", c)
	cmd.Env = os.Environ()
	o, err := cmd.CombinedOutput()
	return string(o), err
}

func SHInDir(c, dir string, envs ...string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", c)
	cmd.Env = append(os.Environ(), envs...)
	cmd.Dir = dir
	o, err := cmd.CombinedOutput()
	return string(o), err
}

func Exists(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}

// UUID TODO: move this into a machine submodule
func UUID() string {
	if os.Getenv("UUID") != "" {
		return os.Getenv("UUID")
	}
	id, _ := machineid.ID()
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%s", id, hostname)
}

func OSRelease(key string) (string, error) {
	release, err := godotenv.Read("/etc/os-release")
	if err != nil {
		return "", err
	}
	kairosKey := "KAIROS_" + key
	v, exists := release[kairosKey]
	if !exists {
		// We try with the old naming without the prefix in case the key wasn't found
		v, exists = release[key]
		if !exists {
			return "", fmt.Errorf("key not found")
		}
	}
	return v, nil
}

func FindCommand(def string, options []string) string {
	for _, p := range options {
		path, err := exec.LookPath(p)
		if err == nil {
			return path
		}
	}

	// Otherwise return default
	return def
}

func K3sBin() string {
	for _, p := range []string{"/usr/bin/k3s", "/usr/local/bin/k3s"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

func WriteEnv(envFile string, config map[string]string) error {
	content, _ := os.ReadFile(envFile)
	env, _ := godotenv.Unmarshal(string(content))

	for key, val := range config {
		env[key] = val
	}

	return godotenv.Write(env, envFile)
}

func Flavor() string {
	v, err := OSRelease("FLAVOR")
	if err != nil {
		return ""
	}

	return v
}

// GetInit Return the init system used by the OS
func GetInit() string {
	for _, file := range []string{"/run/systemd/system", "/sbin/systemctl", "/usr/bin/systemctl", "/usr/sbin/systemctl", "/usr/bin/systemctl"} {
		_, err := os.Stat(file)
		// Found systemd
		if err == nil {
			return systemd
		}
	}

	for _, file := range []string{"/sbin/openrc", "/usr/sbin/openrc", "/bin/openrc", "/usr/bin/openrc"} {
		_, err := os.Stat(file)
		// Found openrc
		if err == nil {
			return openrc
		}
	}

	return unknown
}

func Name() string {
	v, err := OSRelease("NAME")
	if err != nil {
		return ""
	}

	return strings.ReplaceAll(v, "kairos-", "")
}

func IsOpenRCBased() bool {
	return GetInit() == openrc
}

func ShellSTDIN(s, c string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", c)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewBuffer([]byte(s))
	o, err := cmd.CombinedOutput()
	return string(o), err
}

func SetEnv(env []string) {
	for _, e := range env {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) >= 2 {
			os.Setenv(pair[0], pair[1])
		}
	}
}

func OnSignal(fn func(), sig ...os.Signal) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, sig...)
	go func() {
		<-sigs
		fn()
	}()
}

func Shell() *exec.Cmd {
	cmd := exec.Command("/bin/sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd
}

func Prompt(t string) (string, error) {
	if t != "" {
		pterm.Info.Println(t)
	}
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(answer), nil
}

func PrintBanner(d []byte) {
	img, _, _ := image.Decode(bytes.NewReader(d))

	convertOptions := convert.DefaultOptions
	convertOptions.FixedWidth = 100
	convertOptions.FixedHeight = 40

	// Create the image converter
	converter := convert.NewImageConverter()
	fmt.Print(converter.Image2ASCIIString(img, &convertOptions))
}

func Reboot() {
	pterm.Info.Println("Rebooting node")
	SH("reboot") //nolint:errcheck
}

func PowerOFF() {
	pterm.Info.Println("Shutdown node")
	if IsOpenRCBased() {
		SH("poweroff") //nolint:errcheck
	} else {
		SH("shutdown") //nolint:errcheck
	}
}

func Version() string {
	v, err := OSRelease("VERSION")
	if err != nil {
		return ""
	}
	v = strings.ReplaceAll(v, "+k3s1-Kairos", "-")
	v = strings.ReplaceAll(v, "+k3s-Kairos", "-")
	return strings.ReplaceAll(v, "Kairos", "")
}

func ListToOutput(rels []string, output string) []string {
	switch strings.ToLower(output) {
	case "yaml":
		d, _ := yaml.Marshal(rels)
		return []string{string(d)}
	case "json":
		d, _ := json.Marshal(rels)
		return []string{string(d)}
	default:
		return rels
	}
}

func GetInterfaceIP(in string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("failed getting system interfaces")
		return ""
	}
	for _, i := range ifaces {
		if i.Name == in {
			addrs, _ := i.Addrs()
			// handle err
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip != nil {
					return ip.String()

				}
			}
		}
	}
	return ""
}

// GetCurrentPlatform returns the current platform in docker style `linux/amd64` for use with image utils
func GetCurrentPlatform() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
