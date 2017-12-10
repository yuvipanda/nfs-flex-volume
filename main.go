package main

import (
	"encoding/json"
	"fmt"
	"os"
)

/// Return status
func Init() *Response {
	resp := makeResponse("Success", "No Initialization required")
	resp.Capabilities = map[string]interface{}{
		"attach": false,
	}
	return resp
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
