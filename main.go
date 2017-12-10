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

func makeResponse(status, message string) map[string]interface{} {
	return map[string]interface{}{
		"status":  status,
		"message": message,
	}
}

/// Return status
func Init() map[string]interface{} {
	resp := makeResponse("Success", "No Initialization required")
	resp["capabilities"] = map[string]interface{}{
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
func Mount(target string, options map[string]string) map[string]interface{} {
	opts := strings.Split(options["mountOptions"], ",")
	sort.Strings(opts)
	sortedOpts := strings.Join(opts, ",")

	subPath := options["subPath"]
	createIfNecessary := options["createIfNecessary"] == "true"
	createModeUint64, err := strconv.ParseUint(options["createMode"], 0, 32)
	createMode := os.FileMode(createModeUint64)

	sharePath := options["share"]
	//createUid := strconv.Atoi(options["createUid"])
	//createGid := strconv.Atoi(options["createGid"])

	mountPath := fmt.Sprintf("/mnt/nfsflexvolume/%s/options/%s", sharePath, sortedOpts)

	if isStaleMount(mountPath) {
		unmountCmd := exec.Command("umount", mountPath)
		out, err := unmountCmd.CombinedOutput()
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not unmount stale mount %s %s: %s", mountPath, err.Error(), out))
		}
	}

	if !isMountPoint(mountPath) {
		err := os.MkdirAll(mountPath, 0755)

		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not make mountPath %s: %s", mountPath, err.Error()))
		}
		mountCmd := exec.Command("mount", "-t", "nfs4", sharePath, mountPath, "-o", sortedOpts)
		out, err := mountCmd.CombinedOutput()
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not mount %s: %s", err.Error(), out))
		}
	}

	srcPath := path.Join(mountPath, subPath)

	if createIfNecessary {
		err := os.MkdirAll(srcPath, createMode)
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not create subPath: %s", err.Error()))
		}
	} else {
		_, err := os.Stat(srcPath)
		if err != nil {
			return makeResponse("Failure", fmt.Sprintf("Could not find path %s to be mounted: %s", srcPath, err.Error()))
		}
	}

	// Now we rmdir the target, and then make a symlink to it!
	err = os.Remove(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return makeResponse("Failure", fmt.Sprintf("Could not remove target %s before symlink: %s", target, err.Error()))
		}
	}

	err = os.Symlink(srcPath, target)

	if err != nil {
		return makeResponse("Failure", fmt.Sprintf("Could not symlink %s to %s: %s", srcPath, target, err.Error()))
	}

	return makeResponse("Success", "Mount completed!")
}

func Unmount(mountPath string) interface{} {
	err := os.Remove(mountPath)
	if err != nil {
		return makeResponse("Failure", fmt.Sprintf("Could not unmount %s: %s", mountPath, err.Error()))
	}
	return makeResponse("Success", "Successfully unmounted")
}

func printJSON(data interface{}) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", string(jsonBytes))
}

func main() {
	switch action := os.Args[1]; action {
	case "init":
		printJSON(Init())
	case "mount":
		optsString := os.Args[3]
		opts := make(map[string]string)
		json.Unmarshal([]byte(optsString), &opts)
		printJSON(Mount(os.Args[2], opts))
	case "unmount":
		printJSON(Unmount(os.Args[2]))
	default:
		printJSON(makeResponse("Not supported", fmt.Sprintf("Operation %s is not supported", action)))
	}

}
