package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

type Response struct {
	Status       string                 `json:"status"`
	Message      string                 `json:"message"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

func (r *Response) printJson() {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", string(jsonBytes))
}

func makeResponse(status, message string) *Response {
	return &Response{
		Status:  status,
		Message: message,
	}
}

type VolumeSource struct {
	MountOptions      string `json:"mountOptions"`
	SubPath           string `json:"subPath"`
	Share             string `json:"share"`
	CreateIfNecessary string `json:"createIfNecessary"`
	CreateMode        string `json:"createMode"`
}

func (vs *VolumeSource) sortedMountOptions() string {
	opts := strings.Split(vs.MountOptions, ",")
	sort.Strings(opts)
	return strings.Join(opts, ",")
}

func (vs *VolumeSource) mountPath() string {
	return fmt.Sprintf("/mnt/nfsflexvolume/%s/options/%s", vs.Share, vs.sortedMountOptions())

}

func (vs *VolumeSource) srcPath() string {
	if vs.SubPath != "" {
		return path.Join(vs.mountPath(), vs.SubPath)
	} else {
		return vs.mountPath()
	}
}

func (vs *VolumeSource) Mount() *Response {
	if isStaleMount(vs.mountPath()) {
		unmountCmd := exec.Command("umount", vs.mountPath())
		out, err := unmountCmd.CombinedOutput()
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not unmount stale mount %s %s: %s", vs.mountPath(), err.Error(), out))
		}
	}

	if !isMountPoint(vs.mountPath()) {
		err := os.MkdirAll(vs.mountPath(), 0755)

		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not make mountPath %s: %s", vs.mountPath(), err.Error()))
		}
		mountCmd := exec.Command("mount", "-t", "nfs4", vs.Share, vs.mountPath(), "-o", vs.sortedMountOptions())
		out, err := mountCmd.CombinedOutput()
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not mount %s: %s", err.Error(), out))
		}
	}
	return nil
}

func (vs *VolumeSource) EnsureSubPath() *Response {
	if vs.CreateIfNecessary == "true" {
		createModeUint64, err := strconv.ParseUint(vs.CreateMode, 0, 32)
		createMode := os.FileMode(createModeUint64)
		err = os.MkdirAll(vs.srcPath(), createMode)
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not create subPath: %s", err.Error()))
		}
	} else {
		_, err := os.Stat(vs.srcPath())
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not find path %s to be mounted: %s", vs.srcPath(), err.Error()))
		}
	}
	return nil
}

func (vs *VolumeSource) EnsureSymlink(target string) *Response {
	// Now we rmdir the target, and then make a symlink to it!
	err := os.Remove(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return makeResponse("Failure", fmt.Sprintf("Could not remove target %s before symlink: %s", target, err.Error()))
		}
	}

	// Handle case where the target already exists, because the pod has restarted in the meantime!
	err = os.Symlink(vs.srcPath(), target)

	if err != nil {
		return makeResponse("Failure", fmt.Sprintf("Could not symlink %s to %s: %s", vs.srcPath(), target, err.Error()))
	}
	return nil
}
