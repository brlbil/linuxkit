package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"

	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"
	log "github.com/sirupsen/logrus"
)

const (
	guestMetaData = "guestinfo.metadata"
	guestUserData = "guestinfo.userdata"
)

// ProviderVMware is the type implementing the Provider interface for VMware
type ProviderVMware struct {}

// NewVMware returns a new ProviderVMware
func NewVMware() *ProviderVMware {
	return &ProviderVMware{}
}

func (p *ProviderVMware) String() string {
	return "VMWARE"
}

// Probe checks if we are running on VMware
func (p *ProviderVMware) Probe() bool {
	isVM, err := vmcheck.IsVirtualWorld()
	if err != nil {
		log.Fatalf("Error: %s", err)
		return false
	}

	if !isVM {
		log.Fatalf("ERROR: not in a virtual world.")
		return false
	}

	b, err := p.vmwareGet(guestUserData)
	return (err == nil) && len(b) > 0 && string(b) != " " && string(b) != "---"
}

// Extract gets both the hostname and generic userdata
func (p *ProviderVMware) Extract() ([]byte, error) {
	// Get host name. This must not fail
	metaData, err := p.vmwareGet(guestMetaData)
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(path.Join(ConfigPath, "metadata"), metaData, 0644)
	if err != nil {
		return nil, fmt.Errorf("VMWare: Failed to write metadata: %s", err)
	}

	// Generic userdata
	userData, err := p.vmwareGet(guestUserData)
	if err != nil {
		log.Printf("VMware: Failed to get user-data: %s", err)
		// This is not an error
		return nil, nil
	}

	return userData, nil
}

// vmwareGet gets and extracts the guest data
func (p *ProviderVMware) vmwareGet(name string) ([]byte, error) {
	config := rpcvmx.NewConfig()

	// get the guest info value
	sout, err := config.String(name, "")
	if err != nil {
		eErr := err.(*exec.ExitError)
		log.Debugf("Getting guest info %s failed: error %s", name, string(eErr.Stderr))
		return nil, err
	}

	// get the guest info encryption
	senc, err := config.String(name+".encoding", "")
	if err != nil {
		eErr := err.(*exec.ExitError)
		log.Debugf("Getting guest info %s.encoding failed: error %s", name, string(eErr.Stderr))
		return nil, err
	}

	out := []byte(sout)
	enc := []byte(senc)

	switch strings.TrimSuffix(string(enc), "\n") {
	case " ":
		return bytes.TrimSuffix(out, []byte("\n")), nil
	case "base64":
		r := base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(out))

		dst, err := ioutil.ReadAll(r)
		if err != nil {
			log.Debugf("Decoding base64 of '%s' failed %v", name, err)
			return nil, err
		}

		return dst, nil
	case "gzip+base64":
		r := base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(out))

		zr, err := gzip.NewReader(r)
		if err != nil {
			log.Debugf("New gzip reader from '%s' failed %v", name, err)
			return nil, err
		}

		dst, err := ioutil.ReadAll(zr)
		if err != nil {
			log.Debugf("Read '%s' failed %v", name, err)
			return nil, err
		}

		return dst, nil
	default:
		return nil, fmt.Errorf("Unknown encoding %s", string(enc))
	}
}
