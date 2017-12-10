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
	"syscall"
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

/// Return status
func Init() *Response {
	resp := makeResponse("Success", "No Initialization required")
	resp.Capabilities = map[string]interface{}{
		"attach": false,
	}
	return resp
}

func isStaleMount(path string) bool {
	stat := syscall.Stat_t{}
	err := syscall.Stat(path, &stat)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok {
			if errno == syscall.ESTALE {
				return true
			}
		}
	}
	return false
}

func isMountPoint(path string) bool {
	cmd := exec.Command("mountpoint", path)
	err := cmd.Run()
	if err != nil {
		return false
	}
	return true
}

/// If NFS hasn't been mounted yet, mount!
/// If mounted, bind mount to appropriate place.
func Mount(target string, vs *VolumeSource) *Response {

	resp := vs.Mount()
	if resp != nil {
		return resp
	}

	resp = vs.EnsureSubPath()
	if resp != nil {
		return resp
	}

	resp = vs.EnsureSymlink(target)
	if resp != nil {
		return resp
	}

	return makeResponse("Success", "Mount completed!")
}

func Unmount(target string) *Response {
	err := os.Remove(target)
	if err != nil {
		return makeResponse("Failure", fmt.Sprintf("Could not unlink %s: %s", target, err.Error()))
	}
	return makeResponse("Success", "Successfully unmounted")
}

func main() {
	switch action := os.Args[1]; action {
	case "init":
		Init().printJson()
	case "mount":
		optsString := os.Args[3]
		opts := VolumeSource{}
		json.Unmarshal([]byte(optsString), &opts)
		Mount(os.Args[2], &opts).printJson()
	case "unmount":
		Unmount(os.Args[2]).printJson()
	default:
		makeResponse("Not supported", fmt.Sprintf("Operation %s is not supported", action)).printJson()
	}

}
