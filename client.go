package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"syscall"

	"gopkg.in/ini.v1"
)

type InterfaceConf struct {
	PrivateKey string
	Address    string
	ListenPort int
	PostUp     string
	PreDown    string
}

type PeerConf struct {
	PublicKey           string
	AllowedIPs          string
	Endpoint            string
	PersistentKeepalive int
}

type WgConf struct {
	interfaceConf *InterfaceConf
	peerConfList  []*PeerConf
}

func (interfaceConf InterfaceConf) generateString() string {
	return fmt.Sprintf(`
[Interface]
PrivateKey = %s
Address = %s
ListenPort = %d
PostUp = %s
PreDown = %s

`, interfaceConf.PrivateKey, interfaceConf.Address, interfaceConf.ListenPort, interfaceConf.PostUp, interfaceConf.PreDown)
}

func (peerConf PeerConf) generateString() string {
	return fmt.Sprintf(`
[Peer]
Publickey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = %d

`, peerConf.PublicKey, peerConf.AllowedIPs, peerConf.Endpoint, peerConf.PersistentKeepalive)
}

func (wgConf WgConf) generateString() string {
	buf := bytes.Buffer{}
	buf.WriteString(wgConf.interfaceConf.generateString())
	for _, v := range wgConf.peerConfList {
		buf.WriteString(v.generateString())
	}
	return buf.String()
}

const (
	basePath     = "/etc/wireguard/"
	confFilename = "/etc/wireguard/wukuard.conf"
	serviceName  = "wg-quick@wukuard.service"
)

func compareInterfaceConf(src, input *InterfaceConf) bool {
	if src == nil || input == nil {
		return src == input
	}
	if src.PrivateKey != input.PrivateKey ||
		src.Address != input.Address ||
		src.ListenPort != input.ListenPort ||
		src.PostUp != input.PostUp ||
		src.PreDown != input.PreDown {
		return false
	}
	return true
}

func getCurrentConf() (*InterfaceConf, string, error) {
	wholeConfStr, err := readFile(confFilename)
	if err != nil {
		return nil, "", err
	}
	cfg, err := ini.Load(confFilename)
	if err != nil {
		return nil, "", err
	}
	interfaceConf := &InterfaceConf{}
	err = cfg.Section("Interface").MapTo(interfaceConf)
	if err != nil {
		return nil, "", err
	}
	return interfaceConf, wholeConfStr, nil
}

func checkServiceIsRunning() bool {
	output, err := exec.Command("systemctl", "status", serviceName).Output()
	if err != nil {
		return false
	}
	matched, err := regexp.MatchString("Active: active", string(output))
	return err == nil && matched
}

func prepareConfFile() error {
	syscall.Umask(0022)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		err = os.MkdirAll(basePath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// generate the wg conf
	if _, err := os.Stat(confFilename); os.IsNotExist(err) {
		confFile, err := os.Create(confFilename)
		if err != nil {
			return err
		}
		_ = confFile.Close()
	} else {
		return err
	}
	return nil
}

func sync(inputConf *WgConf) {
	var err error
	err = prepareConfFile()
	checkErr(err)

	restartFlag := false
	if !checkServiceIsRunning() {
		restartFlag = true
	}

	interfaceConf, wholeConfStr, err := getCurrentConf()
	checkErr(err)
	if !compareInterfaceConf(interfaceConf, inputConf.interfaceConf) {
		restartFlag = true
	}

	if restartFlag {
		err = writeFile(confFilename, inputConf.generateString())
		checkErr(err)
		err = exec.Command("systemctl", "restart", serviceName).Run()
		checkErr(err)
	}

	inputConfStr := inputConf.generateString()
	if wholeConfStr != inputConfStr {
		// need sync
		err = writeFile(confFilename, inputConfStr)
		checkErr(err)
		err = exec.Command("wg syncconf wukuard <(wg-quick strip wukuard)").Run()
		checkErr(err)
	}
}

func clientMain() {

}
